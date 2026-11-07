package server

import (
	"context"
	"net/http"
	"os/exec"
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
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "journalctl", "-q", "-u", service, "-n", strconv.Itoa(lines), "--no-pager", "-o", "short-iso").CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		writeError(w, http.StatusBadGateway, "读取日志失败: "+message)
		return
	}
	cfg, loadErr := s.store.Load()
	if loadErr == nil {
		output = []byte(redactSecrets(string(output), cfg))
	}
	trimmed := strings.TrimSpace(string(output))
	entries := []string{}
	if trimmed != "" {
		entries = strings.Split(trimmed, "\n")
	}
	writeJSON(w, http.StatusOK, map[string]any{"unit": unit, "entries": entries})
}

func redactSecrets(value string, cfg *config.Config) string {
	secrets := []string{
		cfg.AdminPasswordHash,
		cfg.RackNerd.APIKey,
		cfg.RackNerd.APIHash,
		cfg.Reality.UUID,
		cfg.Reality.PrivateKey,
		cfg.Reality.PublicKey,
		cfg.Reality.ShortID,
		cfg.Hysteria2.Password,
	}
	for _, secret := range secrets {
		if len(secret) >= 6 {
			value = strings.ReplaceAll(value, secret, "[已隐藏]")
		}
	}
	return value
}
