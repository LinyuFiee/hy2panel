package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BBRApplier interface {
	ApplyBBR(context.Context) error
}

type FileBBRApplier struct {
	Dir string
}

func (a FileBBRApplier) ApplyBBR(ctx context.Context) error {
	request := filepath.Join(a.Dir, "bbr.request")
	result := filepath.Join(a.Dir, "bbr.result")
	if err := os.Remove(result); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Remove(request); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := atomicWrite(request, []byte("bbr-fq\n"), 0o600); err != nil {
		return err
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return errors.New("等待 BBR 应用结果超时")
		case <-ticker.C:
			data, err := os.ReadFile(result)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			message := strings.TrimSpace(string(data))
			if message == "ok" {
				return nil
			}
			if len(message) > 4000 {
				message = message[:4000]
			}
			return fmt.Errorf("BBR 应用失败: %s", message)
		}
	}
}

func (s *Server) applyBBR(w http.ResponseWriter, r *http.Request) {
	if !validMutation(r) {
		writeError(w, http.StatusForbidden, "请求来源无效")
		return
	}
	s.bbrMu.Lock()
	defer s.bbrMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	if err := s.bbr.ApplyBBR(ctx); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	cfg, err := s.store.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取配置失败")
		return
	}
	writeJSON(w, http.StatusOK, s.makeState(r.Context(), cfg))
}
