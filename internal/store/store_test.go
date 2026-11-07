package store

import (
	"os"
	"path/filepath"
	"testing"

	"hy2panel/internal/config"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Initial("panel.example.com", "hy2.example.com", "203.0.113.10", "198.51.100.20", "test-password-123456")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestSaveLoadAndUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "panel.json")
	panelStore := New(path)
	original := testConfig(t)
	originalHash := original.AdminPasswordHash
	if err := panelStore.Save(original); err != nil {
		t.Fatal(err)
	}
	loaded, err := panelStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Reality.UUID != original.Reality.UUID || loaded.AdminPasswordHash != originalHash {
		t.Fatal("loaded config differs from original")
	}
	updated, err := panelStore.Update(func(cfg *config.Config) error {
		cfg.Panel.PublicHost = "203.0.113.20"
		cfg.AdminPasswordHash = ""
		return nil
	})
	if err == nil || updated != nil {
		t.Fatal("invalid update should be rejected")
	}
	loaded, err = panelStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Panel.PublicHost != "203.0.113.10" || loaded.AdminPasswordHash != originalHash {
		t.Fatal("failed update modified stored config")
	}
	updated, err = panelStore.Update(func(cfg *config.Config) error {
		cfg.Panel.PublicHost = "203.0.113.20"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Panel.PublicHost != "203.0.113.20" || updated.AdminPasswordHash != originalHash {
		t.Fatal("valid update did not preserve credentials")
	}
}

func TestCorruptedConfigRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "panel.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(path).Load(); err == nil {
		t.Fatal("expected corrupted config to fail")
	}
}
