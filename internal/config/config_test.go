package config

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func newTestConfig(t *testing.T) *Config {
	t.Helper()
	cfg, err := Initial("panel.example.com", "hy2.example.com", "203.0.113.10", "198.51.100.20", "test-password-123456")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestInitialConfigAndSingBoxJSON(t *testing.T) {
	cfg := newTestConfig(t)
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
	if len(document.Inbounds) != 2 {
		t.Fatalf("expected 2 inbounds, got %d", len(document.Inbounds))
	}
	if document.Inbounds[0]["type"] != "vless" || document.Inbounds[1]["type"] != "hysteria2" {
		t.Fatalf("unexpected inbounds: %#v", document.Inbounds)
	}
	if strings.Contains(string(data), cfg.AdminPasswordHash) {
		t.Fatal("administrator password hash leaked into sing-box config")
	}
	if cfg.Panel.AdminCIDR != "198.51.100.20/32" {
		t.Fatalf("unexpected admin CIDR: %s", cfg.Panel.AdminCIDR)
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

func TestPublicConfigOmitsPasswordHash(t *testing.T) {
	cfg := newTestConfig(t)
	public := cfg.Public()
	if public.AdminPasswordHash != "" {
		t.Fatal("public config retained password hash")
	}
	data, err := json.Marshal(public)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "admin_password_hash") || strings.Contains(string(data), cfg.AdminPasswordHash) {
		t.Fatal("password hash serialized")
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
		{"admin CIDR", func(c *Config) { c.Panel.AdminCIDR = "invalid" }},
		{"panel port conflict", func(c *Config) { c.Reality.Port = 8443 }},
		{"short ID", func(c *Config) { c.Reality.ShortID = "xyz" }},
		{"certificate traversal", func(c *Config) { c.Panel.CertFile = "/etc/hy2panel/certs/../panel.json" }},
		{"Hysteria2 bandwidth", func(c *Config) { c.Hysteria2.UpMbps = 0 }},
		{"Hysteria2 password", func(c *Config) { c.Hysteria2.Password = "short" }},
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

func TestRealityOnly(t *testing.T) {
	cfg, err := Initial("panel.example.com", "", "203.0.113.10", "198.51.100.20", "test-password-123456")
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
