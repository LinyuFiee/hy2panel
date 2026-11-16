package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hy2panel/internal/config"
)

func TestFileApplier(t *testing.T) {
	dir := t.TempDir()
	expected := []byte("{\"inbounds\":[]}")
	done := make(chan error, 1)
	go func() {
		request := filepath.Join(dir, "apply.request")
		deadline := time.Now().Add(3 * time.Second)
		for {
			if _, err := os.Stat(request); err == nil {
				break
			}
			if time.Now().After(deadline) {
				done <- os.ErrDeadlineExceeded
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		requestData, err := os.ReadFile(request)
		if err != nil {
			done <- err
			return
		}
		var requestPayload struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(requestData, &requestPayload) != nil || requestPayload.ID == "" {
			done <- os.ErrInvalid
			return
		}
		data, err := os.ReadFile(filepath.Join(dir, "sing-box.pending.json"))
		if err != nil {
			done <- err
			return
		}
		if string(data) != string(expected) {
			done <- os.ErrInvalid
			return
		}
		panelData, err := os.ReadFile(filepath.Join(dir, "panel.pending.json"))
		if err != nil || len(panelData) == 0 {
			done <- os.ErrInvalid
			return
		}
		result, err := json.Marshal(map[string]any{"schema": 1, "id": requestPayload.ID, "status": "succeeded", "message": ""})
		if err != nil {
			done <- err
			return
		}
		done <- os.WriteFile(filepath.Join(dir, "apply.result"), result, 0o600)
	}()
	cfg, err := config.Initial("panel.example.com", "", "203.0.113.10", "", "test-password-123456")
	if err != nil {
		t.Fatal(err)
	}
	applier := FileApplier{Dir: dir}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := applier.Apply(ctx, cfg, expected); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
