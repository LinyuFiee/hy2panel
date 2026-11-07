package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	_, _, _, _, cfg := newTestServer(t)
	cfg.RackNerd.APIKey = "racknerd-secret"
	cfg.RackNerd.APIHash = "racknerd-hash"
	input := strings.Join([]string{cfg.AdminPasswordHash, cfg.Reality.UUID, cfg.Reality.PrivateKey, cfg.Reality.PublicKey, cfg.Reality.ShortID, cfg.Hysteria2.Password, cfg.RackNerd.APIKey, cfg.RackNerd.APIHash}, " ")
	output := redactSecrets(input, cfg)
	for _, secret := range []string{cfg.AdminPasswordHash, cfg.Reality.UUID, cfg.Reality.PrivateKey, cfg.Reality.PublicKey, cfg.Reality.ShortID, cfg.Hysteria2.Password, cfg.RackNerd.APIKey, cfg.RackNerd.APIHash} {
		if strings.Contains(output, secret) {
			t.Fatalf("secret was not redacted: %s", secret)
		}
	}
	if !strings.Contains(output, "[已隐藏]") {
		t.Fatal("redaction marker missing")
	}
}

func TestListenerPorts(t *testing.T) {
	output := []byte("LISTEN 0 4096 0.0.0.0:443 0.0.0.0:*\nLISTEN 0 4096 [::]:2053 [::]:*\nUNCONN 0 0 0.0.0.0:2083 0.0.0.0:*\n")
	ports := listenerPorts(output)
	if !ports[443] || !ports[2053] || !ports[2083] || ports[80] {
		t.Fatalf("unexpected listeners: %#v", ports)
	}
}

func TestUpdateRecovery(t *testing.T) {
	_, _, handler, panelStore, _ := newTestServer(t)
	cookie := loginCookie(t, handler)
	request := adminRequest(http.MethodPost, "/api/settings/recovery", []byte(`{"auto_port":true}`))
	request.AddCookie(cookie)
	request.Header.Set("X-Requested-With", "fetch")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("recovery update failed: %d %s", response.Code, response.Body.String())
	}
	cfg, err := panelStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Recovery.AutoPort || cfg.Recovery.LastPortChange != 0 {
		t.Fatalf("unexpected recovery config: %#v", cfg.Recovery)
	}
}
