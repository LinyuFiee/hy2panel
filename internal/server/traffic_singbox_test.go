package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hy2panel/internal/traffic"
)

type trafficSourceFunc func(context.Context) (traffic.ClashSnapshot, error)

func (f trafficSourceFunc) Fetch(ctx context.Context) (traffic.ClashSnapshot, error) {
	return f(ctx)
}

func TestTrafficEndpointsRequireAuthenticationAndReturnSnapshots(t *testing.T) {
	panelServer, _, handler, _, _ := newTestServer(t)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, adminRequest(http.MethodGet, "/api/traffic/live", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", response.Code)
	}

	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	calls := 0
	source := trafficSourceFunc(func(context.Context) (traffic.ClashSnapshot, error) {
		calls++
		return traffic.ClashSnapshot{
			UploadTotal:   uint64(100 + calls*40),
			DownloadTotal: uint64(200 + calls*80),
			Connections: []traffic.ClashConnection{{
				ID: "reality-flow", Metadata: traffic.ClashMetadata{Type: "vless/reality"}, Upload: uint64(10 + calls*40), Download: uint64(20 + calls*80), Start: now.Add(-time.Minute),
			}},
		}, nil
	})
	collector, err := traffic.NewCollector(source, nil, traffic.WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = collector.Run(ctx) }()
	t.Cleanup(cancel)
	time.Sleep(20 * time.Millisecond)
	now = now.Add(time.Second)
	panelServer.traffic = collector
	panelServer.trafficErr = ""

	cookie := loginCookie(t, handler)
	request := adminRequest(http.MethodGet, "/api/traffic/live", nil)
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("live failed: %d %s", response.Code, response.Body.String())
	}
	var live trafficLiveResponse
	if err := json.Unmarshal(response.Body.Bytes(), &live); err != nil {
		t.Fatal(err)
	}
	if !live.Available {
		t.Fatalf("collector unavailable: %#v", live)
	}
}

func TestSingBoxVersionEndpointsUseAllowlist(t *testing.T) {
	_, _, handler, _, _ := newTestServer(t)
	temp := t.TempDir()
	manifest := `{"schema":1,"platform":"linux-amd64","versions":[{"version":"1.14.4","tag":"v1.14.4","status":"recommended","asset_name":"sing-box-1.14.4-linux-amd64.tar.gz","size":31710753,"sha256":"699a1b510aa4396b2176d32d1181920399ebf1e267835da6f3b598d0f7c24968","release_url":"https://github.com/SagerNet/sing-box/releases/tag/v1.14.4"}]}`
	path := filepath.Join(temp, "versions.json")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HY2PANEL_SING_BOX_MANIFEST", path)
	cookie := loginCookie(t, handler)

	request := adminRequest(http.MethodGet, "/api/sing-box/versions", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"recommended_version":"1.14.4"`)) {
		t.Fatalf("version list failed: %d %s", response.Code, response.Body.String())
	}

	request = adminRequest(http.MethodPost, "/api/sing-box/switch", []byte(`{"version":"9.9.9"}`))
	request.AddCookie(cookie)
	request.Header.Set("X-Requested-With", "fetch")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unsupported version accepted: %d %s", response.Code, response.Body.String())
	}
}
