package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
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
		data, err := os.ReadFile(filepath.Join(dir, "sing-box.pending.json"))
		if err != nil {
			done <- err
			return
		}
		if string(data) != string(expected) {
			done <- os.ErrInvalid
			return
		}
		done <- os.WriteFile(filepath.Join(dir, "apply.result"), []byte("ok\n"), 0o600)
	}()
	applier := FileApplier{Dir: dir}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := applier.Apply(ctx, expected); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
