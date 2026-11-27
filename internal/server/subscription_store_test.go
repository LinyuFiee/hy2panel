package server

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSubscriptionStorePrivateAtomicWrites(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	store := &subscriptionStore{path: filepath.Join(dir, "subscription.json")}
	first, err := store.change("create")
	if err != nil {
		t.Fatal(err)
	}
	stat := func(path string, mode os.FileMode) os.FileInfo {
		t.Helper()
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != mode {
			t.Fatalf("unexpected mode: %v", info.Mode().Perm())
		}
		return info
	}
	stat(dir, 0o700)
	original := stat(store.path, 0o600)
	for _, action := range []string{"copy", "create"} {
		token, err := store.change(action)
		if err != nil || token != first || !os.SameFile(original, stat(store.path, 0o600)) {
			t.Fatalf("%s replaced or changed an existing token", action)
		}
	}
	next, err := store.change("rotate")
	if err != nil || next == first {
		t.Fatal("failed to rotate")
	}
	replacement := stat(store.path, 0o600)
	if runtime.GOOS != "windows" && os.SameFile(original, replacement) {
		t.Fatal("updated in place instead of replacing the secret file")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(store.path) {
		t.Fatal("unexpected temporary state after committing the secret")
	}
	if _, err := store.change("revoke"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(store.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("revocation left the secret file on disk")
	}
}

func TestSubscriptionStoreFailedRenameCleansUp(t *testing.T) {
	dir := t.TempDir()
	s := &subscriptionStore{path: filepath.Join(dir, "blocked")}
	// An existing directory is not a replaceable secret file, even for root.
	if err := os.Mkdir(s.path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := s.save("temporary-test-value"); err == nil {
		t.Fatal("unexpectedly replaced the destination directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || !entries[0].IsDir() || entries[0].Name() != "blocked" {
		t.Fatal("failed write left a secret or modified the destination")
	}
	if token, err := s.change("create"); token != "" || err == nil {
		t.Fatal("failed create returned a token or reported success")
	}
}

func TestSubscriptionStoreRejectsUnsafePaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX modes and symlinks")
	}
	dir := filepath.Join(t.TempDir(), "private")
	store := &subscriptionStore{path: filepath.Join(dir, "subscription.json")}
	active, err := store.change("create")
	if err != nil {
		t.Fatal(err)
	}
	checkUnsafe := func() {
		t.Helper()
		if _, err := store.current(); err == nil {
			t.Fatal("served state from an unsafe path")
		}
		if token, err := store.change("rotate"); token != "" || err == nil {
			t.Fatal("wrote to an unsafe path")
		}
	}
	if err := os.Chmod(store.path, 0o644); err != nil {
		t.Fatal(err)
	}
	checkUnsafe()
	if err := os.Chmod(store.path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	checkUnsafe()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	real := store.path + ".real"
	if err := os.Rename(store.path, real); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, store.path); err != nil {
		t.Fatal(err)
	}
	checkUnsafe()
	other := &subscriptionStore{path: real}
	if token, err := other.current(); err != nil || token != active {
		t.Fatal("symlink target was modified")
	}
	linked := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(dir, linked); err != nil {
		t.Fatal(err)
	}
	store.path = filepath.Join(linked, filepath.Base(real))
	checkUnsafe()
}
