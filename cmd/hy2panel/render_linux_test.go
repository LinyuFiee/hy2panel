package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestManagedRenderInputMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "panel.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	uid, gid := os.Geteuid(), os.Getegid()
	if _, err := readManagedRenderConfig(path, uid, gid); err != nil {
		t.Fatal(err)
	}
	if _, err := readManagedRenderConfig(path, uid+1, gid); err == nil {
		t.Fatal("accepted unexpected owner")
	}
	if _, err := readManagedRenderConfig(path, uid, gid+1); err == nil {
		t.Fatal("accepted unexpected group")
	}
	link, fifo := filepath.Join(dir, "link"), filepath.Join(dir, "fifo")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []string{link, fifo, dir} {
		if _, err := readManagedRenderConfig(candidate, uid, gid); err == nil {
			t.Errorf("accepted unsafe input %q", candidate)
		}
	}
	if err := os.Link(path, filepath.Join(dir, "other-link")); err != nil {
		t.Fatal(err)
	}
	if _, err := readManagedRenderConfig(path, uid, gid); err == nil {
		t.Fatal("accepted a hard-linked input")
	}
	if err := os.Remove(filepath.Join(dir, "other-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readManagedRenderConfig(path, uid, gid); err == nil {
		t.Fatal("accepted a world-readable input")
	}
}
