package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"hy2panel/internal/config"
	"hy2panel/internal/racknerd"
)

type rackNerdInfoClient interface {
	Info(context.Context, string, string) (racknerd.Info, error)
}

type RackNerdView struct {
	Configured bool           `json:"configured"`
	Available  bool           `json:"available"`
	Error      string         `json:"error,omitempty"`
	Info       *racknerd.Info `json:"info,omitempty"`
}

type rackNerdSettingsRequest struct {
	APIKey  string `json:"api_key"`
	APIHash string `json:"api_hash"`
	Clear   bool   `json:"clear"`
}

func (s *Server) rackNerdState(ctx context.Context, cfg *config.Config) RackNerdView {
	if !cfg.RackNerd.Configured() {
		return RackNerdView{}
	}
	s.rackNerdMu.Lock()
	defer s.rackNerdMu.Unlock()
	if !s.rackNerdCachedAt.IsZero() && time.Since(s.rackNerdCachedAt) < s.rackNerdTTL {
		return s.rackNerdCache
	}
	view := RackNerdView{Configured: true}
	requestContext, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	info, err := s.rackNerdClient.Info(requestContext, cfg.RackNerd.APIKey, cfg.RackNerd.APIHash)
	if err != nil {
		view.Error = err.Error()
		if s.rackNerdCache.Info != nil {
			view.Info = s.rackNerdCache.Info
		}
		s.rackNerdTTL = time.Minute
	} else {
		view.Available = true
		view.Info = &info
		s.rackNerdTTL = 5 * time.Minute
	}
	s.rackNerdCache = view
	s.rackNerdCachedAt = time.Now()
	return view
}

func (s *Server) updateRackNerd(w http.ResponseWriter, r *http.Request) {
	if !validMutation(r) {
		writeError(w, http.StatusForbidden, "请求来源无效")
		return
	}
	var input rackNerdSettingsRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "RackNerd 配置请求无效")
		return
	}
	current, err := s.store.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取配置失败")
		return
	}
	next := *current
	if input.Clear {
		next.RackNerd = config.RackNerd{}
	} else {
		if value := strings.TrimSpace(input.APIKey); value != "" {
			next.RackNerd.APIKey = value
		}
		if value := strings.TrimSpace(input.APIHash); value != "" {
			next.RackNerd.APIHash = value
		}
	}
	if !next.RackNerd.Configured() && !input.Clear {
		writeError(w, http.StatusBadRequest, "请输入完整的 RackNerd API Key 和 Hash")
		return
	}
	if err := next.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.Save(&next); err != nil {
		writeError(w, http.StatusInternalServerError, "保存 RackNerd 配置失败")
		return
	}
	s.rackNerdMu.Lock()
	s.rackNerdCache = RackNerdView{}
	s.rackNerdCachedAt = time.Time{}
	s.rackNerdMu.Unlock()
	writeJSON(w, http.StatusOK, s.makeState(r.Context(), &next))
}

func (s *Server) makeState(ctx context.Context, cfg *config.Config) stateResponse {
	return stateResponse{
		Config:    cfg.Public(),
		Links:     cfg.ClientLinks(),
		Status:    readStatus(ctx),
		RackNerd:  s.rackNerdState(ctx, cfg),
		SpeedTest: s.speedtest.State(),
	}
}
