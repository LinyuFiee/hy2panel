package config

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Compare JSON values, not object key order or the Go types used to build them.
func assertProfileValue(t *testing.T, field string, got, want any) {
	t.Helper()
	g, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	w, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(g, w) {
		t.Errorf("%s: got %s, want %s", field, g, w)
	}
}

func TestSingBoxClientProfile(t *testing.T) {
	base := newTestConfig(t) // Synthetic keys and passwords; never read a deployed config.
	base.Reality.ServerName = "handshake.example.test"
	base.Reality.Port = 21443
	base.Hysteria2.Domain = "quic.example.test"
	base.Hysteria2.Port = 31443
	base.Hysteria2.Password = `synthetic-"password<&>`
	base.Hysteria2.UpMbps, base.Hysteria2.DownMbps = 50, 300
	base.Hysteria2.BBRProfile = "aggressive"
	base.RackNerd = RackNerd{APIKey: "private-api-key-fixture", APIHash: "private-api-hash-fixture"}

	for _, tc := range []struct {
		name, host, obfs string
		nodes            []string
	}{
		{"dual", "node.example.test", "", []string{"reality", "hysteria2"}},
		{"reality-ipv6", "2001:db8::11", "", []string{"reality"}},
		{"hysteria2-salamander", "192.0.2.1", "salamander", []string{"hysteria2"}},
		{"dual-gecko", "192.0.2.2", "gecko", []string{"reality", "hysteria2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := *base
			cfg.Panel.PublicHost = tc.host
			cfg.Reality.Enabled = slices.Contains(tc.nodes, "reality")
			cfg.Hysteria2.Enabled = slices.Contains(tc.nodes, "hysteria2")
			cfg.Hysteria2.ObfsType = tc.obfs
			if tc.obfs != "" {
				cfg.Hysteria2.ObfsPassword = `synthetic-obfs-"<&>`
			}
			data, err := cfg.SingBoxClientProfile()
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{cfg.Reality.PrivateKey, cfg.AdminPasswordHash, cfg.Telemetry.Secret, cfg.RackNerd.APIKey, cfg.RackNerd.APIHash, "/etc/hy2panel/", cfg.Panel.Domain, "clash_api", "external_controller"} {
				if bytes.Contains(data, []byte(forbidden)) {
					t.Fatal("client profile exposed a server-only value")
				}
			}
			type obj = map[string]any
			var p struct {
				DNS, Route          obj
				Inbounds, Outbounds []obj
			}
			if err := json.Unmarshal(data, &p); err != nil {
				t.Fatal(err)
			}
			assertProfileValue(t, "inbounds", p.Inbounds, []obj{{"type": "tun", "tag": "tun-in", "address": []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"}, "auto_route": true, "dns_mode": "hijack"}})
			assertProfileValue(t, "dns", p.DNS, obj{
				"servers": []obj{{"type": "local", "tag": "bootstrap"}, {"type": "https", "tag": "remote", "server": "1.1.1.1", "detour": "proxy"}}, "final": "remote",
			})
			assertProfileValue(t, "route", p.Route, obj{
				"default_domain_resolver": "bootstrap", "auto_detect_interface": true, "final": "proxy",
				"rules": []obj{{"inbound": "tun-in", "port": 53, "action": "hijack-dns"}},
			})
			if len(p.Outbounds) != len(tc.nodes)+2 {
				t.Fatalf("unexpected number of outbounds: %d", len(p.Outbounds))
			}
			assertProfileValue(t, "selector", p.Outbounds[0], obj{"type": "selector", "tag": "proxy", "outbounds": append([]string{"auto"}, tc.nodes...), "default": "auto"})
			assertProfileValue(t, "urltest", p.Outbounds[1], obj{"type": "urltest", "tag": "auto", "outbounds": tc.nodes, "url": "https://www.gstatic.com/generate_204", "interval": "3m", "tolerance": 50})
			for i, tag := range tc.nodes {
				var want obj
				switch tag {
				case "reality":
					r := cfg.Reality
					want = obj{
						"type": "vless", "tag": tag, "server": tc.host, "server_port": r.Port, "uuid": r.UUID, "flow": "xtls-rprx-vision",
						"tls": obj{"enabled": true, "server_name": r.ServerName, "utls": obj{"enabled": true, "fingerprint": "chrome"}, "reality": obj{"enabled": true, "public_key": r.PublicKey, "short_id": r.ShortID}},
					}
				case "hysteria2":
					h := cfg.Hysteria2
					want = obj{
						"type": "hysteria2", "tag": tag, "server": h.Domain, "server_port": h.Port, "password": h.Password,
						"tls": obj{"enabled": true, "server_name": h.Domain, "alpn": []string{"h3"}},
					}
					if tc.obfs != "" {
						want["obfs"] = obj{"type": tc.obfs, "password": h.ObfsPassword}
					}
				}
				assertProfileValue(t, tag, p.Outbounds[i+2], want)
			}

			t.Run("sing-box-check", func(t *testing.T) {
				binary := os.Getenv("SING_BOX_BINARY")
				if binary == "" {
					t.Skip("set SING_BOX_BINARY to a trusted sing-box executable for runtime validation")
				}
				dir := t.TempDir()
				profile := filepath.Join(dir, "client.json")
				if err := os.WriteFile(profile, data, 0600); err != nil {
					t.Fatal(err)
				}
				cmd := exec.CommandContext(t.Context(), binary, "check", "-c", profile)
				cmd.Dir = dir
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("sing-box check: %v\n%s", err, output)
				}
			})
		})
	}
}

func TestSingBoxClientProfileRejectsUnusableNodes(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Reality.Enabled, cfg.Hysteria2.Enabled = false, false
	if data, err := cfg.SingBoxClientProfile(); err == nil || data != nil || !strings.Contains(err.Error(), "没有已启用的节点") {
		t.Fatalf("expected explicit no-node error and no profile, got %v", err)
	}
	cfg.Reality.Enabled = true
	cfg.Reality.ShortID = strings.Repeat("12", 9)
	if data, err := cfg.SingBoxClientProfile(); err == nil || data != nil {
		t.Fatal("accepted Reality short ID too long for the client")
	}
}
