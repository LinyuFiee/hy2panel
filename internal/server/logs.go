package server

import (
	"context"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"hy2panel/internal/config"
)

var allowedLogUnits = map[string]string{
	"sing-box": "sing-box.service",
	"panel":    "hy2panel.service",
	"watchdog": "hy2panel-watchdog.service",
	"apply":    "hy2panel-apply.service",
}

func (s *Server) logs(w http.ResponseWriter, r *http.Request) {
	unit := r.URL.Query().Get("unit")
	if unit == "" {
		unit = "sing-box"
	}
	service, ok := allowedLogUnits[unit]
	if !ok {
		writeError(w, http.StatusBadRequest, "不支持的日志单元")
		return
	}
	lines := 100
	if value := r.URL.Query().Get("lines"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 20 || parsed > 300 {
			writeError(w, http.StatusBadRequest, "日志行数必须在 20 到 300 之间")
			return
		}
		lines = parsed
	}
	cfg, err := s.store.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "无法读取配置以隐藏日志凭据")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "journalctl", "-q", "-u", service, "-n", strconv.Itoa(lines), "--no-pager", "-o", "short-iso").CombinedOutput()
	trimmed := strings.TrimSpace(redactSecrets(string(output), cfg))
	if err != nil {
		if trimmed == "" {
			trimmed = err.Error()
		}
		writeError(w, http.StatusBadGateway, "读取日志失败: "+trimmed)
		return
	}
	entries := []string{}
	if trimmed != "" {
		entries = strings.Split(trimmed, "\n")
	}
	writeJSON(w, http.StatusOK, map[string]any{"unit": unit, "entries": entries})
}

// Match the path, not the stored token: rotated and revoked URLs are secrets too.
var subscriptionLogPath = regexp.MustCompile(`/api/sub/[^/\s?#"'<>]+`)

func redactSecrets(value string, cfg *config.Config) string {
	value = subscriptionLogPath.ReplaceAllString(value, subscriptionPrefix+"[已隐藏]")
	secrets := []string{
		cfg.AdminPasswordHash,
		cfg.RackNerd.APIKey,
		cfg.RackNerd.APIHash,
		cfg.Telemetry.Secret,
		cfg.Reality.UUID,
		cfg.Reality.PrivateKey,
		cfg.Reality.PublicKey,
		cfg.Reality.ShortID,
		cfg.Hysteria2.Password,
		cfg.Hysteria2.ObfsPassword,
	}
	for _, secret := range secrets {
		if len(secret) >= 6 {
			value = strings.ReplaceAll(value, secret, "[已隐藏]")
		}
	}
	return value
}
