package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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

func TestSavePreservesExistingMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission preservation semantics")
	}
	path := filepath.Join(t.TempDir(), "panel.json")
	panelStore := New(path)
	cfg := testConfig(t)
	if err := panelStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	cfg.Panel.PublicHost = "203.0.113.20"
	if err := panelStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("mode changed across atomic save: %o", got)
	}
}

func TestMigrateLegacyConfigPreservesDeprecatedField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "panel.json")
	cfg := testConfig(t)
	cfg.Panel.AdminCIDR = "198.51.100.20/32"
	cfg.Telemetry = config.Telemetry{}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	panelStore := New(path)
	changed, err := panelStore.Migrate()
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("legacy config was not migrated")
	}
	loaded, err := panelStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != 1 || loaded.Panel.AdminCIDR != "198.51.100.20/32" || loaded.Telemetry.Secret == "" || loaded.Telemetry.Listen != "127.0.0.1:19090" {
		t.Fatalf("unexpected migrated config: %#v", loaded)
	}
	secret := loaded.Telemetry.Secret
	changed, err = panelStore.Migrate()
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("second migration should be a no-op")
	}
	again, err := panelStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if again.Telemetry.Secret != secret {
		t.Fatal("idempotent migration rotated telemetry secret")
	}
}

func TestMigrateRejectsUnknownSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "panel.json")
	cfg := testConfig(t)
	cfg.Version = 2
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if changed, err := New(path).Migrate(); err == nil || changed {
		t.Fatalf("unknown schema migration = %v, %v", changed, err)
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
