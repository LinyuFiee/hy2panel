package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"hy2panel/internal/singbox"
)

const singBoxManifestPath = "/usr/local/share/hy2panel/sing-box-versions.json"

type singBoxOperation struct {
	ID            string `json:"id,omitempty"`
	Action        string `json:"action,omitempty"`
	Status        string `json:"status,omitempty"`
	Phase         string `json:"phase,omitempty"`
	TargetVersion string `json:"target_version,omitempty"`
	Message       string `json:"message,omitempty"`
	Rollback      any    `json:"rollback,omitempty"`
}

type singBoxState struct {
	Schema             int              `json:"schema"`
	CurrentVersion     string           `json:"current_version,omitempty"`
	PreviousVersion    string           `json:"previous_version,omitempty"`
	RecommendedVersion string           `json:"recommended_version,omitempty"`
	Operation          singBoxOperation `json:"operation,omitempty"`
}

type singBoxVersionsResponse struct {
	CurrentVersion     string                  `json:"current_version,omitempty"`
	PreviousVersion    string                  `json:"previous_version,omitempty"`
	RecommendedVersion string                  `json:"recommended_version"`
	CanRollback        bool                    `json:"can_rollback"`
	Versions           []singbox.PublicVersion `json:"versions"`
	Operation          singBoxOperation        `json:"operation,omitempty"`
}

func (s *Server) loadSingBoxVersions() (singBoxVersionsResponse, error) {
	manifestPath := singBoxManifestPath
	if value := os.Getenv("HY2PANEL_SING_BOX_MANIFEST"); value != "" {
		manifestPath = value
	}
	manifest, err := singbox.LoadFile(manifestPath)
	if err != nil {
		return singBoxVersionsResponse{}, err
	}
	public, err := manifest.Public()
	if err != nil {
		return singBoxVersionsResponse{}, err
	}
	response := singBoxVersionsResponse{
		RecommendedVersion: public.RecommendedVersion,
		Versions:           public.Versions,
	}
	statePath := filepath.Join("/var/lib/hy2panel-system/sing-box", "state.json")
	if data, readErr := os.ReadFile(statePath); readErr == nil {
		var state singBoxState
		if json.Unmarshal(data, &state) == nil && state.Schema == 1 {
			response.CurrentVersion = state.CurrentVersion
			response.PreviousVersion = state.PreviousVersion
			response.Operation = state.Operation
		}
	}
	response.CanRollback = response.PreviousVersion != ""
	return response, nil
}

func (s *Server) singBoxVersions(w http.ResponseWriter, _ *http.Request) {
	view, err := s.loadSingBoxVersions()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "读取 sing-box 兼容版本失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) switchSingBox(w http.ResponseWriter, r *http.Request) {
	if !validMutation(r) {
		writeError(w, http.StatusForbidden, "请求来源无效")
		return
	}
	var input struct {
		Version string `json:"version"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "sing-box 版本请求无效")
		return
	}
	view, err := s.loadSingBoxVersions()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	allowed := false
	for _, entry := range view.Versions {
		if entry.Version == input.Version {
			allowed = true
			break
		}
	}
	if !allowed {
		writeError(w, http.StatusBadRequest, "该 sing-box 版本不在兼容清单中")
		return
	}
	if view.Operation.Status == "running" {
		writeError(w, http.StatusConflict, "已有核心维护任务正在运行")
		return
	}
	operation, err := s.submitSingBoxOperation("switch", input.Version)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, operation)
}

func (s *Server) rollbackSingBox(w http.ResponseWriter, r *http.Request) {
	if !validMutation(r) {
		writeError(w, http.StatusForbidden, "请求来源无效")
		return
	}
	view, err := s.loadSingBoxVersions()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if !view.CanRollback {
		writeError(w, http.StatusConflict, "没有可回滚的上一版本")
		return
	}
	if view.Operation.Status == "running" {
		writeError(w, http.StatusConflict, "已有核心维护任务正在运行")
		return
	}
	operation, err := s.submitSingBoxOperation("rollback", "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, operation)
}

func (s *Server) submitSingBoxOperation(action, target string) (singBoxOperation, error) {
	id, err := newOperationID()
	if err != nil {
		return singBoxOperation{}, err
	}
	operation := singBoxOperation{ID: id, Action: action, Status: "running", Phase: "requested", TargetVersion: target, Message: "请求已提交"}
	request := map[string]any{"schema": 1, "id": id, "action": action}
	if target != "" {
		request["target_version"] = target
	}
	data, err := json.Marshal(request)
	if err != nil {
		return singBoxOperation{}, err
	}
	data = append(data, '\n')
	dir := filepath.Dir(s.store.Path())
	if err := os.Remove(filepath.Join(dir, "sing-box-update.result")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return singBoxOperation{}, err
	}
	if err := atomicWrite(filepath.Join(dir, "sing-box-update.request"), data, 0o600); err != nil {
		return singBoxOperation{}, err
	}
	return operation, nil
}
