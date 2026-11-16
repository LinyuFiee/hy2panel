package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"hy2panel/internal/config"
	"hy2panel/internal/store"
	"hy2panel/internal/version"
)

func captureCLI(t *testing.T, input string) (*bytes.Buffer, func()) {
	t.Helper()
	oldStdin, oldStdout, oldStderr := stdin, stdout, stderr
	out := &bytes.Buffer{}
	stdin = strings.NewReader(input)
	stdout = out
	stderr = &bytes.Buffer{}
	return out, func() {
		stdin, stdout, stderr = oldStdin, oldStdout, oldStderr
	}
}

func TestRunVersionJSON(t *testing.T) {
	out, restore := captureCLI(t, "")
	defer restore()
	if err := runVersion([]string{"--json"}); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Version       string `json:"version"`
		ConfigVersion int    `json:"config_version"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Version != version.Version || result.ConfigVersion != 1 {
		t.Fatalf("unexpected version output: %#v", result)
	}
}

func TestRunInitAdminIPIsOptionalAndIgnored(t *testing.T) {
	for _, args := range [][]string{
		{"--panel-domain", "panel.example.com", "--public-host", "203.0.113.10"},
		{"--panel-domain", "panel.example.com", "--public-host", "203.0.113.10", "--admin-ip", "legacy-invalid-value"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out, restore := captureCLI(t, "test-password-123456\n")
			defer restore()
			_ = out
			path := filepath.Join(t.TempDir(), "panel.json")
			commandArgs := append([]string{"--config", path}, args...)
			if err := runInit(commandArgs); err != nil {
				t.Fatal(err)
			}
			cfg, err := store.New(path).Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Panel.AdminCIDR != "" || cfg.Telemetry.Secret == "" || cfg.Version != 1 {
				t.Fatalf("unexpected initialized config: %#v", cfg)
			}
		})
	}
}

func TestRunMigrateIsIdempotent(t *testing.T) {
	cfg, err := config.Initial("panel.example.com", "", "203.0.113.10", "", "test-password-123456")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Panel.AdminCIDR = "198.51.100.20/32"
	cfg.Telemetry = config.Telemetry{}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "panel.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	out, restore := captureCLI(t, "")
	defer restore()
	if err := runMigrate([]string{"--config", path}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "迁移完成") {
		t.Fatalf("unexpected first migrate output: %q", out.String())
	}
	loaded, err := store.New(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != 1 || loaded.Panel.AdminCIDR != "198.51.100.20/32" || loaded.Telemetry.Secret == "" {
		t.Fatalf("unexpected migration result: %#v", loaded)
	}
	secret := loaded.Telemetry.Secret
	out.Reset()
	if err := runMigrate([]string{"--config", path}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "无需迁移") {
		t.Fatalf("unexpected second migrate output: %q", out.String())
	}
	again, err := store.New(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if again.Telemetry.Secret != secret {
		t.Fatal("idempotent CLI migration rotated telemetry secret")
	}
}

func TestRunResetPasswordReadsStdinAndPreservesConfig(t *testing.T) {
	cfg, err := config.Initial("panel.example.com", "hy2.example.com", "203.0.113.10", "", "test-password-123456")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "panel.json")
	panelStore := store.New(path)
	if err := panelStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	oldHash := cfg.AdminPasswordHash
	oldSecret := cfg.Telemetry.Secret
	_, restore := captureCLI(t, "replacement-password-123456\r\n")
	defer restore()
	if err := runResetPassword([]string{"--config", path}); err != nil {
		t.Fatal(err)
	}
	updated, err := panelStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if updated.AdminPasswordHash == oldHash {
		t.Fatal("password hash did not change")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(updated.AdminPasswordHash), []byte("replacement-password-123456")); err != nil {
		t.Fatal(err)
	}
	if updated.Telemetry.Secret != oldSecret || updated.Reality.UUID != cfg.Reality.UUID {
		t.Fatal("reset-password changed unrelated configuration")
	}
}

func TestRunResetPasswordRejectsPolicyViolationWithoutWriting(t *testing.T) {
	cfg, err := config.Initial("panel.example.com", "", "203.0.113.10", "", "test-password-123456")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "panel.json")
	panelStore := store.New(path)
	if err := panelStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_, restore := captureCLI(t, "too-short\n")
	defer restore()
	if err := runResetPassword([]string{"--config", path}); err == nil {
		t.Fatal("short password accepted")
	}
	loaded, err := panelStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AdminPasswordHash != cfg.AdminPasswordHash {
		t.Fatal("rejected reset changed stored password hash")
	}
}
