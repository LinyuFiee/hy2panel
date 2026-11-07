package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Applier interface {
	Apply(context.Context, []byte) error
}

type FileApplier struct {
	Dir string
}

func (a FileApplier) Apply(ctx context.Context, singBoxConfig []byte) error {
	pending := filepath.Join(a.Dir, "sing-box.pending.json")
	request := filepath.Join(a.Dir, "apply.request")
	result := filepath.Join(a.Dir, "apply.result")
	if err := os.Remove(result); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Remove(request); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := atomicWrite(pending, singBoxConfig, 0o600); err != nil {
		return err
	}
	if err := atomicWrite(request, []byte("apply\n"), 0o600); err != nil {
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
	return os.Rename(tempPath, path)
}
