package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hy2panel/internal/config"
	"hy2panel/internal/racknerd"
)

type fakeRackNerdClient struct {
	calls int
	info  racknerd.Info
	err   error
}

func (f *fakeRackNerdClient) Info(_ context.Context, key, hash string) (racknerd.Info, error) {
	f.calls++
	return f.info, f.err
}

func TestRackNerdStateCache(t *testing.T) {
	panelServer, _, _, panelStore, _ := newTestServer(t)
	if _, err := panelStore.Update(func(cfg *config.Config) error {
		cfg.RackNerd.APIKey = "key"
		cfg.RackNerd.APIHash = "hash"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	client := &fakeRackNerdClient{info: racknerd.Info{BandwidthTotal: 3000, BandwidthUsed: 1000, BandwidthFree: 2000}}
	panelServer.rackNerdClient = client
	cfg, err := panelStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	first := panelServer.rackNerdState(t.Context(), cfg)
	second := panelServer.rackNerdState(t.Context(), cfg)
	if client.calls != 1 || !first.Available || !second.Available || first.Info.BandwidthUsed != 1000 {
		t.Fatalf("unexpected cached state: %#v %#v, calls=%d", first, second, client.calls)
	}
}

func TestRackNerdCredentialsNotReturned(t *testing.T) {
	panelServer, _, handler, panelStore, _ := newTestServer(t)
	client := &fakeRackNerdClient{info: racknerd.Info{BandwidthTotal: 3000, BandwidthUsed: 1000, BandwidthFree: 2000}}
	panelServer.rackNerdClient = client
	cookie := loginCookie(t, handler)
	body := []byte(`{"api_key":"secret-key","api_hash":"secret-hash","clear":false}`)
	request := adminRequest(http.MethodPost, "/api/settings/racknerd", body)
	request.AddCookie(cookie)
	request.Header.Set("X-Requested-With", "fetch")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("settings failed: %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "secret-key") || strings.Contains(response.Body.String(), "secret-hash") {
		t.Fatal("credentials leaked in API response")
	}
	cfg, err := panelStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.RackNerd.Configured() || cfg.RackNerd.APIKey != "secret-key" || cfg.RackNerd.APIHash != "secret-hash" {
		t.Fatal("credentials were not stored correctly")
	}
}
