package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"hy2panel/internal/singbox"
	"hy2panel/internal/version"
)

const panelBundleName = "hy2panel-bundle-linux-amd64.tar.gz"

type UpdateView struct {
	CurrentVersion string    `json:"current_version"`
	LatestVersion  string    `json:"latest_version"`
	ReleaseURL     string    `json:"release_url"`
	Available      bool      `json:"available"`
	Checking       bool      `json:"checking"`
	Updating       bool      `json:"updating"`
	OperationID    string    `json:"operation_id,omitempty"`
	Phase          string    `json:"phase,omitempty"`
	Status         string    `json:"status,omitempty"`
	Message        string    `json:"message,omitempty"`
	Error          string    `json:"error,omitempty"`
	CheckedAt      time.Time `json:"checked_at,omitempty"`
}

type panelRelease struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		Digest string `json:"digest"`
		URL    string `json:"browser_download_url"`
	} `json:"assets"`
}

type upgradeResult struct {
	Schema        int    `json:"schema"`
	ID            string `json:"id"`
	Status        string `json:"status"`
	Phase         string `json:"phase"`
	TargetVersion string `json:"target_version"`
	Message       string `json:"message"`
}

type UpdateManager struct {
	mu         sync.Mutex
	view       UpdateView
	dir        string
	httpClient *http.Client
	apiURL     string
}

func NewUpdateManager(dir string) *UpdateManager {
	manager := &UpdateManager{
		dir:        dir,
		view:       UpdateView{CurrentVersion: version.Version},
		httpClient: &http.Client{Timeout: 10 * time.Second},
		apiURL:     "https://api.github.com/repos/LinyuFiee/hy2panel/releases/latest",
	}
	manager.restoreResult()
	return manager
}

func (m *UpdateManager) State() UpdateView {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.restoreResultLocked()
	return m.view
}

func (m *UpdateManager) Check(ctx context.Context) (UpdateView, error) {
	m.mu.Lock()
	if m.view.Checking {
		view := m.view
		m.mu.Unlock()
		return view, errors.New("正在检查更新")
	}
	m.view.Checking = true
	m.view.Error = ""
	m.mu.Unlock()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, m.apiURL, nil)
	if err != nil {
		return m.finishCheck(err), err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "hy2panel")
	response, err := m.httpClient.Do(request)
	if err != nil {
		return m.finishCheck(err), err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		err = fmt.Errorf("GitHub 返回 HTTP %d", response.StatusCode)
		return m.finishCheck(err), err
	}
	var release panelRelease
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2<<20))
	if err := decoder.Decode(&release); err != nil {
		return m.finishCheck(err), err
	}
	if release.Draft || release.Prerelease || release.TagName == "" || release.HTMLURL == "" {
		err = errors.New("GitHub 未返回有效稳定版本")
		return m.finishCheck(err), err
	}
	latest, err := singbox.NormalizeVersion(release.TagName)
	if err != nil {
		return m.finishCheck(err), err
	}
	matched := 0
	for _, asset := range release.Assets {
		if asset.Name == panelBundleName {
			matched++
			if asset.Size <= 0 || !strings.HasPrefix(asset.URL, "https://github.com/LinyuFiee/hy2panel/releases/download/") {
				err = errors.New("GitHub 面板安装包元数据无效")
				return m.finishCheck(err), err
			}
			if asset.Digest != "" && !strings.HasPrefix(asset.Digest, "sha256:") {
				err = errors.New("GitHub 面板安装包摘要格式无效")
				return m.finishCheck(err), err
			}
		}
	}
	if matched != 1 {
		err = errors.New("GitHub Release 未包含唯一的面板安装包")
		return m.finishCheck(err), err
	}
	comparison, err := singbox.CompareVersions(latest, version.Version)
	if err != nil {
		return m.finishCheck(err), err
	}
	m.mu.Lock()
	m.view.LatestVersion = latest
	m.view.ReleaseURL = release.HTMLURL
	m.view.Available = comparison > 0
	m.view.CheckedAt = time.Now().UTC()
	m.view.Checking = false
	m.view.Status = "checked"
	m.mu.Unlock()
	return m.State(), nil
}

func (m *UpdateManager) finishCheck(err error) UpdateView {
	m.mu.Lock()
	m.view.Checking = false
	m.view.Error = err.Error()
	view := m.view
	m.mu.Unlock()
	return view
}

func (m *UpdateManager) StartUpgrade() (UpdateView, error) {
	m.mu.Lock()
	m.restoreResultLocked()
	if m.view.Updating {
		view := m.view
		m.mu.Unlock()
		return view, errors.New("已有升级任务正在运行")
	}
	if !m.view.Available || m.view.LatestVersion == "" {
		view := m.view
		m.mu.Unlock()
		return view, errors.New("请先检测并确认可用更新")
	}
	target := m.view.LatestVersion
	m.mu.Unlock()

	operationID, err := newOperationID()
	if err != nil {
		return m.finishUpgrade(err), err
	}
	requestData, err := json.Marshal(map[string]any{"schema": 1, "id": operationID, "action": "upgrade", "target_version": target})
	if err != nil {
		return m.finishUpgrade(err), err
	}
	requestData = append(requestData, '\n')
	requestPath := filepath.Join(m.dir, "upgrade.request")
	resultPath := filepath.Join(m.dir, "upgrade.result")
	if err := os.Remove(resultPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return m.finishUpgrade(err), err
	}
	if err := os.Remove(requestPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return m.finishUpgrade(err), err
	}
	if err := atomicWrite(requestPath, requestData, 0o600); err != nil {
		return m.finishUpgrade(err), err
	}
	m.mu.Lock()
	m.view.Updating = true
	m.view.OperationID = operationID
	m.view.Status = "running"
	m.view.Phase = "requested"
	m.view.Error = ""
	m.view.Message = "升级请求已提交，面板可能短暂重启"
	view := m.view
	m.mu.Unlock()
	return view, nil
}

func (m *UpdateManager) restoreResult() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.restoreResultLocked()
}

func (m *UpdateManager) restoreResultLocked() {
	data, err := os.ReadFile(filepath.Join(m.dir, "upgrade.result"))
	if err != nil {
		return
	}
	var result upgradeResult
	if json.Unmarshal(data, &result) != nil || result.Schema != 1 || result.ID == "" {
		return
	}
	if m.view.OperationID != "" && result.ID != m.view.OperationID {
		return
	}
	m.view.OperationID = result.ID
	m.view.Status = result.Status
	m.view.Phase = result.Phase
	m.view.Message = result.Message
	m.view.Updating = result.Status == "running"
	if result.Status == "succeeded" {
		m.view.CurrentVersion = result.TargetVersion
		m.view.Available = false
		m.view.Error = ""
	} else if result.Status != "running" {
		m.view.Error = result.Message
	}
}

func (m *UpdateManager) finishUpgrade(err error) UpdateView {
	m.mu.Lock()
	m.view.Updating = false
	m.view.Error = err.Error()
	view := m.view
	m.mu.Unlock()
	return view
}

func compareVersions(left, right string) int {
	comparison, err := singbox.CompareVersions(left, right)
	if err != nil {
		return 0
	}
	return comparison
}

func (s *Server) checkUpdate(w http.ResponseWriter, r *http.Request) {
	if !validMutation(r) {
		writeError(w, http.StatusForbidden, "请求来源无效")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	view, err := s.update.Check(ctx)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, view)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) applyUpdate(w http.ResponseWriter, r *http.Request) {
	if !validMutation(r) {
		writeError(w, http.StatusForbidden, "请求来源无效")
		return
	}
	view, err := s.update.StartUpgrade()
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, view)
}
