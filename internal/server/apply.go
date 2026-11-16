package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"hy2panel/internal/config"
)

type Applier interface {
	Apply(context.Context, *config.Config, []byte) error
}

type FileApplier struct {
	Dir string
}

func (a FileApplier) Apply(ctx context.Context, panelConfig *config.Config, singBoxConfig []byte) error {
	if panelConfig == nil {
		return errors.New("面板候选配置不能为空")
	}
	panelData, err := json.MarshalIndent(panelConfig, "", "  ")
	if err != nil {
		return err
	}
	panelData = append(panelData, '\n')
	panelPending := filepath.Join(a.Dir, "panel.pending.json")
	pending := filepath.Join(a.Dir, "sing-box.pending.json")
	request := filepath.Join(a.Dir, "apply.request")
	result := filepath.Join(a.Dir, "apply.result")
	if err := os.Remove(result); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Remove(request); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := atomicWrite(panelPending, panelData, 0o600); err != nil {
		return err
	}
	if err := atomicWrite(pending, singBoxConfig, 0o600); err != nil {
		return err
	}
	operationID, err := newOperationID()
	if err != nil {
		return err
	}
	requestData, err := json.Marshal(map[string]any{"schema": 1, "id": operationID, "action": "apply"})
	if err != nil {
		return err
	}
	requestData = append(requestData, '\n')
	if err := atomicWrite(request, requestData, 0o600); err != nil {
		return err
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(45 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return errors.New("等待配置应用结果超时")
		case <-ticker.C:
			data, err := os.ReadFile(result)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			message := strings.TrimSpace(string(data))
			var structured struct {
				ID      string `json:"id"`
				Status  string `json:"status"`
				Message string `json:"message"`
			}
			if json.Unmarshal(data, &structured) == nil && structured.ID != "" {
				if structured.ID != operationID {
					continue
				}
				if structured.Status == "succeeded" {
					return nil
				}
				message = structured.Message
			}
			if message == "ok" {
				return nil
			}
			if len(message) > 8000 {
				message = message[:8000]
			}
			if message == "" {
				message = "未知应用错误"
			}
			return fmt.Errorf("配置应用失败: %s", message)
		}
	}
}

func newOperationID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".apply-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
