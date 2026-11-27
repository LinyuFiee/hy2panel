package server

import (
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const subscriptionPrefix = "/api/sub/"

type subscriptionResponse struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url,omitempty"`
}

func (s *Server) subscriptionStatus(w http.ResponseWriter, _ *http.Request) {
	token, err := s.subscriptions.current()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取订阅状态失败")
		return
	}
	writeJSON(w, http.StatusOK, subscriptionResponse{Enabled: token != ""})
}

func (s *Server) manageSubscription(w http.ResponseWriter, r *http.Request) {
	if !validMutation(r) {
		writeError(w, http.StatusForbidden, "请求来源无效")
		return
	}
	var input struct {
		Action string `json:"action"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "订阅请求无效")
		return
	}

	var link url.URL
	switch input.Action {
	case "create", "copy", "rotate":
		// Resolve the public authority before committing any change. Never trust
		// the request Host or forwarding headers for a URL carrying credentials.
		cfg, err := s.store.Load()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "读取面板配置失败")
			return
		}
		_, port, err := net.SplitHostPort(cfg.Panel.Listen)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "面板监听地址无效")
			return
		}
		link = url.URL{Scheme: "https", Host: net.JoinHostPort(cfg.Panel.Domain, port)}
	case "revoke": // Revocation doesn't depend on the profile or panel config.
	default:
		writeError(w, http.StatusBadRequest, "不支持的订阅操作")
		return
	}

	token, err := s.subscriptions.change(input.Action)
	if errors.Is(err, errSubscriptionDisabled) {
		writeError(w, http.StatusConflict, "订阅尚未启用")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "订阅操作失败")
		return
	}
	response := subscriptionResponse{Enabled: token != ""}
	if response.Enabled {
		link.Path = subscriptionPrefix + token
		response.URL = link.String()
	}
	writeJSON(w, http.StatusOK, response)
}

func malformedSubscriptionPath(u *url.URL) bool {
	token, ok := strings.CutPrefix(u.Path, subscriptionPrefix)
	return u.Path == "/api/sub" || (ok && (!validSubscriptionToken(token) || u.EscapedPath() != u.Path))
}

func (s *Server) subscriptionProfile(w http.ResponseWriter, r *http.Request) {
	candidate := r.PathValue("token")
	token, err := s.subscriptions.current()
	if err != nil || token == "" || subtle.ConstantTimeCompare([]byte(candidate), []byte(token)) != 1 {
		http.NotFound(w, r)
		return
	}
	cfg, err := s.store.Load()
	if err != nil || (!cfg.Reality.Enabled && !cfg.Hysteria2.Enabled) {
		writeError(w, http.StatusServiceUnavailable, "订阅配置暂不可用")
		return
	}
	profile, err := cfg.SingBoxClientProfile()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "订阅配置暂不可用")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(profile)
}
