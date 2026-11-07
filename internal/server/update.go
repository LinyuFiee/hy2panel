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
	"strconv"
	"strings"
	"sync"
	"time"

	"hy2panel/internal/version"
)

type UpdateView struct {
	CurrentVersion string    `json:"current_version"`
	LatestVersion  string    `json:"latest_version"`
	ReleaseURL     string    `json:"release_url"`
	Available      bool      `json:"available"`
	Checking       bool      `json:"checking"`
	Updating       bool      `json:"updating"`
	Message        string    `json:"message,omitempty"`
	Error          string    `json:"error,omitempty"`
	CheckedAt      time.Time `json:"checked_at,omitempty"`
}

type UpdateManager struct {
	mu   sync.Mutex
	view UpdateView
	dir  string
}

func NewUpdateManager(dir string) *UpdateManager {
	return &UpdateManager{dir: dir, view: UpdateView{CurrentVersion: version.Version}}
}

func (m *UpdateManager) State() UpdateView {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.view
}

func (m *UpdateManager) Check(ctx context.Context) (UpdateView, error) {
	m.mu.Lock()
	if m.view.Checking {
		m.mu.Unlock()
		return m.view, errors.New("正在检查更新")
	}
	m.view.Checking = true
	m.view.Error = ""
	m.mu.Unlock()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/LinyuFiee/hy2panel/releases/latest", nil)
	if err != nil {
		return m.finishCheck(err), err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "hy2panel")
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return m.finishCheck(err), err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		err := fmt.Errorf("GitHub 返回 HTTP %d", response.StatusCode)
		return m.finishCheck(err), err
	}
	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&release); err != nil {
		return m.finishCheck(err), err
	}
	if release.TagName == "" || release.HTMLURL == "" {
		err := errors.New("GitHub 未返回有效版本信息")
		return m.finishCheck(err), err
	}
	m.mu.Lock()
	m.view.LatestVersion = strings.TrimPrefix(release.TagName, "v")
	m.view.ReleaseURL = release.HTMLURL
	m.view.Available = compareVersions(m.view.LatestVersion, m.view.CurrentVersion) > 0
	m.view.CheckedAt = time.Now().UTC()
	m.view.Checking = false
	m.mu.Unlock()
	return m.State(), nil
}

func (m *UpdateManager) finishCheck(err error) UpdateView {
	m.mu.Lock()
	m.view.Checking = false
	m.view.Error = err.Error()
	m.mu.Unlock()
	return m.State()
}

func (m *UpdateManager) StartUpgrade() (UpdateView, error) {
	m.mu.Lock()
	if m.view.Updating {
		m.mu.Unlock()
		return m.view, errors.New("已有升级任务正在运行")
	}
	m.view.Updating = true
	m.view.Error = ""
	m.view.Message = "正在下载并校验更新，面板将短暂重启"
	m.mu.Unlock()
	request := filepath.Join(m.dir, "upgrade.request")
	result := filepath.Join(m.dir, "upgrade.result")
	if err := os.Remove(result); err != nil && !errors.Is(err, os.ErrNotExist) {
		return m.finishUpgrade(err), err
	}
	if err := os.Remove(request); err != nil && !errors.Is(err, os.ErrNotExist) {
		return m.finishUpgrade(err), err
	}
	if err := atomicWrite(request, []byte("latest\n"), 0o600); err != nil {
		return m.finishUpgrade(err), err
	}
	go m.waitUpgrade(result)
	return m.State(), nil
}

func (m *UpdateManager) waitUpgrade(result string) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(3 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			m.finishUpgrade(errors.New("等待升级结果超时"))
			return
		case <-ticker.C:
			data, err := os.ReadFile(result)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				m.finishUpgrade(err)
				return
			}
			message := strings.TrimSpace(string(data))
			if message == "ok" {
				m.mu.Lock()
				m.view.Updating = false
				m.view.CurrentVersion = m.view.LatestVersion
				m.view.Available = false
				m.view.Message = "更新完成，请刷新页面"
				m.mu.Unlock()
				return
			}
			m.finishUpgrade(fmt.Errorf("升级失败: %s", message))
			return
		}
	}
}

func (m *UpdateManager) finishUpgrade(err error) UpdateView {
	m.mu.Lock()
	m.view.Updating = false
	m.view.Error = err.Error()
	m.mu.Unlock()
	return m.State()
}

func compareVersions(left, right string) int {
	left = strings.TrimPrefix(left, "v")
	right = strings.TrimPrefix(right, "v")
	for index := 0; index < 3; index++ {
		leftPart := versionPart(left, index)
		rightPart := versionPart(right, index)
		if leftPart > rightPart {
			return 1
		}
		if leftPart < rightPart {
			return -1
		}
	}
	return 0
}

func versionPart(value string, index int) int {
	parts := strings.Split(value, ".")
	if index >= len(parts) {
		return 0
	}
	number, _ := strconv.Atoi(parts[index])
	return number
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
