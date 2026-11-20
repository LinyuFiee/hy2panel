package config

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func newTestConfig(t *testing.T) *Config {
	t.Helper()
	cfg, err := Initial("panel.example.com", "hy2.example.com", "203.0.113.10", "", "test-password-123456")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestInitialConfigAndSingBoxJSON(t *testing.T) {
	cfg := newTestConfig(t)
	if cfg.Version != SchemaVersion {
		t.Fatalf("unexpected schema version: %d", cfg.Version)
	}
	if SchemaVersion != 1 {
		t.Fatalf("schema version changed: %d", SchemaVersion)
	}
	if cfg.Panel.AdminCIDR != "" {
		t.Fatalf("new config retained deprecated admin CIDR: %q", cfg.Panel.AdminCIDR)
	}
	secret, err := base64.RawURLEncoding.DecodeString(cfg.Telemetry.Secret)
	if err != nil || len(secret) != 32 {
		t.Fatalf("unexpected telemetry secret: %q", cfg.Telemetry.Secret)
	}
	if !cfg.Telemetry.Enabled || cfg.Telemetry.Listen != "127.0.0.1:19090" {
		t.Fatalf("unexpected telemetry config: %#v", cfg.Telemetry)
	}
	data, err := cfg.SingBoxJSON()
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Inbounds     []map[string]any `json:"inbounds"`
		Experimental struct {
			ClashAPI struct {
				ExternalController string `json:"external_controller"`
				Secret             string `json:"secret"`
			} `json:"clash_api"`
		} `json:"experimental"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Inbounds) != 2 {
		t.Fatalf("expected 2 inbounds, got %d", len(document.Inbounds))
	}
	if document.Inbounds[0]["type"] != "vless" || document.Inbounds[1]["type"] != "hysteria2" {
		t.Fatalf("unexpected inbounds: %#v", document.Inbounds)
	}
	if document.Experimental.ClashAPI.ExternalController != "127.0.0.1:19090" || document.Experimental.ClashAPI.Secret != cfg.Telemetry.Secret {
		t.Fatalf("unexpected clash API config: %#v", document.Experimental.ClashAPI)
	}
	if strings.Contains(string(data), cfg.AdminPasswordHash) {
		t.Fatal("administrator password hash leaked into sing-box config")
	}
}

func TestInitialIgnoresDeprecatedAdminIP(t *testing.T) {
	cfg, err := Initial("panel.example.com", "", "203.0.113.10", "not-an-ip", "test-password-123456")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Panel.AdminCIDR != "" {
		t.Fatalf("deprecated admin IP affected config: %q", cfg.Panel.AdminCIDR)
	}
}

func TestClientLinks(t *testing.T) {
	cfg := newTestConfig(t)
	links := cfg.ClientLinks()
	reality, err := url.Parse(links["reality"])
	if err != nil {
		t.Fatal(err)
	}
	if reality.Scheme != "vless" || reality.User.Username() != cfg.Reality.UUID || reality.Host != "203.0.113.10:443" {
		t.Fatalf("unexpected Reality link: %s", reality.String())
	}
	query := reality.Query()
	if query.Get("security") != "reality" || query.Get("flow") != "xtls-rprx-vision" || query.Get("pbk") != cfg.Reality.PublicKey {
		t.Fatalf("unexpected Reality query: %s", reality.RawQuery)
	}
	hy2, err := url.Parse(links["hysteria2"])
	if err != nil {
		t.Fatal(err)
	}
	if hy2.Scheme != "hysteria2" || hy2.User.Username() != cfg.Hysteria2.Password || hy2.Host != "hy2.example.com:443" {
		t.Fatalf("unexpected Hysteria2 link: %s", hy2.String())
	}
	if hy2.Query().Get("insecure") != "0" || hy2.Query().Get("sni") != cfg.Hysteria2.Domain {
		t.Fatalf("unexpected Hysteria2 query: %s", hy2.RawQuery)
	}
}

func TestPublicConfigClearsPrivateAndDeprecatedFields(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Panel.AdminCIDR = "198.51.100.20/32"
	cfg.RackNerd.APIKey = "key"
	cfg.RackNerd.APIHash = "hash"
	public := cfg.Public()
	if public.AdminPasswordHash != "" || public.Panel.AdminCIDR != "" || public.Telemetry.Secret != "" || public.RackNerd.APIKey != "" || public.RackNerd.APIHash != "" {
		t.Fatalf("public config retained private fields: %#v", public)
	}
	if cfg.Panel.AdminCIDR == "" || cfg.Telemetry.Secret == "" {
		t.Fatal("Public mutated the source config")
	}
	data, err := json.Marshal(public)
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(data)
	for _, forbidden := range []string{"admin_password_hash", "admin_cidr", cfg.AdminPasswordHash, cfg.Telemetry.Secret} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("public config serialized %q", forbidden)
		}
	}
}

func TestDeprecatedAdminCIDRRoundTrip(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Panel.AdminCIDR = "198.51.100.20/32"
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Panel.AdminCIDR != cfg.Panel.AdminCIDR {
		t.Fatalf("admin CIDR was not preserved: %q", decoded.Panel.AdminCIDR)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateAddsTelemetryIdempotentlyWithoutVersionBump(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Panel.AdminCIDR = "198.51.100.20/32"
	cfg.Telemetry = Telemetry{}
	changed, err := cfg.Migrate()
	if err != nil {
		t.Fatal(err)
	}
	if !changed || cfg.Version != 1 || !cfg.Telemetry.Enabled || cfg.Telemetry.Listen != "127.0.0.1:19090" || cfg.Telemetry.Secret == "" {
		t.Fatalf("unexpected migrated config: %#v", cfg)
	}
	secret := cfg.Telemetry.Secret
	changed, err = cfg.Migrate()
	if err != nil {
		t.Fatal(err)
	}
	if changed || cfg.Telemetry.Secret != secret || cfg.Panel.AdminCIDR != "198.51.100.20/32" {
		t.Fatalf("migration was not idempotent: %#v", cfg)
	}
}

func TestLegacyConfigWithoutTelemetryCanBeLoadedForMigration(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Telemetry = Telemetry{}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("legacy config should remain readable before explicit migration: %v", err)
	}
	changed, err := cfg.Migrate()
	if err != nil || !changed || cfg.Telemetry.Secret == "" {
		t.Fatalf("legacy migration failed: changed=%v err=%v", changed, err)
	}
}

func TestEnsureTelemetryPreservesExplicitDisabledState(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Telemetry.Enabled = false
	cfg.Telemetry.Secret = ""
	changed, err := cfg.EnsureTelemetry()
	if err != nil {
		t.Fatal(err)
	}
	if !changed || cfg.Telemetry.Enabled || cfg.Telemetry.Secret == "" {
		t.Fatalf("explicit disabled state was not preserved: %#v", cfg.Telemetry)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestHashAdminPasswordUsesSharedPolicy(t *testing.T) {
	if _, err := HashAdminPassword(strings.Repeat("x", 11)); err == nil {
		t.Fatal("short password accepted")
	}
	if _, err := HashAdminPassword(strings.Repeat("x", 129)); err == nil {
		t.Fatal("long password accepted")
	}
	hash, err := HashAdminPassword("new-password-123456")
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("new-password-123456")); err != nil {
		t.Fatal(err)
	}
}

func TestRealityKeypairMismatchRejected(t *testing.T) {
	cfg := newTestConfig(t)
	other := newTestConfig(t)
	cfg.Reality.PublicKey = other.Reality.PublicKey
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "不匹配") {
		t.Fatalf("expected key mismatch error, got %v", err)
	}
}

func TestInvalidConfigRejected(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"public host", func(c *Config) { c.Panel.PublicHost = "bad host" }},
		{"deprecated admin CIDR", func(c *Config) { c.Panel.AdminCIDR = "invalid" }},
		{"panel port conflict", func(c *Config) { c.Reality.Port = 8443 }},
		{"short ID", func(c *Config) { c.Reality.ShortID = "xyz" }},
		{"certificate traversal", func(c *Config) { c.Panel.CertFile = "/etc/hy2panel/certs/../panel.json" }},
		{"Hysteria2 bandwidth", func(c *Config) { c.Hysteria2.UpMbps = 10001 }},
		{"Hysteria2 password", func(c *Config) { c.Hysteria2.Password = "short" }},
		{"telemetry wildcard", func(c *Config) { c.Telemetry.Listen = ":19090" }},
		{"telemetry public", func(c *Config) { c.Telemetry.Listen = "0.0.0.0:19090" }},
		{"telemetry hostname", func(c *Config) { c.Telemetry.Listen = "localhost:19090" }},
		{"telemetry panel conflict", func(c *Config) { c.Telemetry.Listen = "127.0.0.1:8443" }},
		{"telemetry proxy conflict", func(c *Config) { c.Telemetry.Listen = "127.0.0.1:443" }},
		{"telemetry bad secret", func(c *Config) { c.Telemetry.Secret = "not-a-secret" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := newTestConfig(t)
			test.mutate(cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestTelemetryCanBeDisabled(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Telemetry.Enabled = false
	data, err := cfg.SingBoxJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "clash_api") || strings.Contains(string(data), cfg.Telemetry.Secret) {
		t.Fatal("disabled telemetry was rendered")
	}
}

func TestRealityOnly(t *testing.T) {
	cfg, err := Initial("panel.example.com", "", "203.0.113.10", "", "test-password-123456")
	if err != nil {
		t.Fatal(err)
	}
	data, err := cfg.SingBoxJSON()
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Inbounds []map[string]any `json:"inbounds"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Inbounds) != 1 || document.Inbounds[0]["type"] != "vless" {
		t.Fatalf("unexpected Reality-only config: %#v", document.Inbounds)
	}
	if _, ok := cfg.ClientLinks()["hysteria2"]; ok {
		t.Fatal("disabled Hysteria2 generated a client link")
	}
}

func hysteriaInbound(t *testing.T, cfg *Config) map[string]any {
	t.Helper()
	data, err := cfg.SingBoxJSON()
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Inbounds []map[string]any `json:"inbounds"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	for _, inbound := range document.Inbounds {
		if inbound["type"] == "hysteria2" {
			return inbound
		}
	}
	t.Fatal("hysteria2 inbound missing")
	return nil
}

func TestHysteria2TuningRendering(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Hysteria2.UpMbps = 0
	cfg.Hysteria2.DownMbps = 0
	cfg.Hysteria2.BBRProfile = "aggressive"
	cfg.Hysteria2.ObfsType = "salamander"
	cfg.Hysteria2.ObfsPassword = "obfs-password-123456"

	inbound := hysteriaInbound(t, cfg)
	if _, ok := inbound["up_mbps"]; ok {
		t.Fatal("up_mbps should be omitted when unlimited")
	}
	if _, ok := inbound["down_mbps"]; ok {
		t.Fatal("down_mbps should be omitted when unlimited")
	}
	if inbound["bbr_profile"] != "aggressive" {
		t.Fatalf("unexpected bbr_profile: %v", inbound["bbr_profile"])
	}
	obfs, ok := inbound["obfs"].(map[string]any)
	if !ok {
		t.Fatalf("obfs missing: %#v", inbound)
	}
	if obfs["type"] != "salamander" || obfs["password"] != "obfs-password-123456" {
		t.Fatalf("unexpected obfs: %#v", obfs)
	}

	link := cfg.ClientLinks()["hysteria2"]
	for _, want := range []string{"obfs=salamander", "obfs-password=obfs-password-123456"} {
		if !strings.Contains(link, want) {
			t.Fatalf("client link missing %q: %s", want, link)
		}
	}
}

func TestHysteria2TuningDefaults(t *testing.T) {
	cfg := newTestConfig(t)
	if cfg.Hysteria2.BBRProfile != "standard" {
		t.Fatalf("unexpected default bbr profile: %q", cfg.Hysteria2.BBRProfile)
	}
	inbound := hysteriaInbound(t, cfg)
	if inbound["up_mbps"] != float64(100) || inbound["down_mbps"] != float64(100) {
		t.Fatalf("default bandwidth missing: %#v", inbound)
	}
	if _, ok := inbound["obfs"]; ok {
		t.Fatal("obfs should be disabled by default")
	}
	if strings.Contains(cfg.ClientLinks()["hysteria2"], "obfs") {
		t.Fatal("default client link should not carry obfs parameters")
	}
}

func TestHysteria2TuningValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		ok     bool
	}{
		{"unlimited", func(c *Config) { c.Hysteria2.UpMbps = 0; c.Hysteria2.DownMbps = 0 }, true},
		{"asymmetric", func(c *Config) { c.Hysteria2.UpMbps = 100; c.Hysteria2.DownMbps = 1000 }, true},
		{"negative bandwidth", func(c *Config) { c.Hysteria2.UpMbps = -1 }, false},
		{"oversized bandwidth", func(c *Config) { c.Hysteria2.DownMbps = 10001 }, false},
		{"unknown bbr profile", func(c *Config) { c.Hysteria2.BBRProfile = "turbo" }, false},
		{"empty bbr profile", func(c *Config) { c.Hysteria2.BBRProfile = "" }, true},
		{"obfs without password", func(c *Config) { c.Hysteria2.ObfsType = "salamander" }, false},
		{"short obfs password", func(c *Config) { c.Hysteria2.ObfsType = "salamander"; c.Hysteria2.ObfsPassword = "short" }, false},
		{"obfs password without type", func(c *Config) { c.Hysteria2.ObfsPassword = "obfs-password-123456" }, false},
		{"unknown obfs type", func(c *Config) { c.Hysteria2.ObfsType = "rc4"; c.Hysteria2.ObfsPassword = "obfs-password-123456" }, false},
	}
	for _, test := range cases {
		cfg := newTestConfig(t)
		test.mutate(cfg)
		err := cfg.Validate()
		if test.ok && err != nil {
			t.Fatalf("%s: unexpected error: %v", test.name, err)
		}
		if !test.ok && err == nil {
			t.Fatalf("%s: expected validation error", test.name)
		}
	}
}
