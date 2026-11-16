package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"hy2panel/internal/config"
	"hy2panel/internal/store"
	"hy2panel/internal/version"
)

type fakeApplier struct {
	calls int
	data  []byte
	err   error
	store *store.Store
}

func (f *fakeApplier) Apply(_ context.Context, cfg *config.Config, data []byte) error {
	f.calls++
	f.data = append([]byte(nil), data...)
	if f.err != nil {
		return f.err
	}
	if f.store != nil {
		return f.store.Save(cfg)
	}
	return nil
}

func newTestServer(t *testing.T) (*Server, *fakeApplier, http.Handler, *store.Store, *config.Config) {
	t.Helper()
	cfg, err := config.Initial("panel.example.com", "hy2.example.com", "203.0.113.10", "198.51.100.20", "test-password-123456")
	if err != nil {
		t.Fatal(err)
	}
	panelStore := store.New(filepath.Join(t.TempDir(), "panel.json"))
	if err := panelStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	applier := &fakeApplier{store: panelStore}
	panelServer := New(panelStore, applier)
	handler := panelServer.routes()
	return panelServer, applier, handler, panelStore, cfg
}

func adminRequest(method, target string, body []byte) *http.Request {
	request := httptest.NewRequest(method, target, bytes.NewReader(body))
	request.RemoteAddr = "198.51.100.20:12345"
	request.Host = "panel.example.com:8443"
	return request
}

func loginCookie(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()
	body := []byte(`{"password":"test-password-123456"}`)
	request := adminRequest(http.MethodPost, "/api/login", body)
	request.Header.Set("X-Requested-With", "fetch")
	request.Header.Set("Origin", "https://panel.example.com:8443")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", response.Code, response.Body.String())
	}
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == "__Host-hy2panel" {
			if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
				t.Fatal("session cookie missing security attributes")
			}
			return cookie
		}
	}
	t.Fatal("session cookie not found")
	return nil
}

func TestNoManagementIPRestrictionAndHealthMetadata(t *testing.T) {
	_, _, handler, _, _ := newTestServer(t)
	for _, remote := range []string{
		"198.51.100.20:1000",
		"[::ffff:198.51.100.20]:1000",
		"203.0.113.99:1000",
	} {
		request := adminRequest(http.MethodGet, "/api/health", nil)
		request.RemoteAddr = remote
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("remote %s: got %d", remote, response.Code)
		}
		var health struct {
			Status        string `json:"status"`
			Version       string `json:"version"`
			ConfigVersion int    `json:"config_version"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &health); err != nil {
			t.Fatal(err)
		}
		if health.Status != "ok" || health.Version != version.Version || health.ConfigVersion != config.SchemaVersion {
			t.Fatalf("unexpected health response: %#v", health)
		}
	}
}

func loginRequest(remote, password string) *http.Request {
	request := adminRequest(http.MethodPost, "/api/login", []byte(`{"password":"`+password+`"}`))
	request.RemoteAddr = remote
	request.Header.Set("X-Requested-With", "fetch")
	return request
}

func TestLoginRateLimitUsesRemoteAddrOnly(t *testing.T) {
	panelServer, _, handler, _, _ := newTestServer(t)
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	panelServer.auth = newAuthenticatorWithClock(func() time.Time { return now })
	for i := 0; i < maxLoginFailures; i++ {
		request := loginRequest("198.51.100.7:1234", "wrong-password")
		request.Header.Set("X-Forwarded-For", "203.0.113."+strconv.Itoa(i+1))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		expected := http.StatusUnauthorized
		if i == maxLoginFailures-1 {
			expected = http.StatusTooManyRequests
		}
		if response.Code != expected {
			t.Fatalf("failure %d: expected %d, got %d: %s", i+1, expected, response.Code, response.Body.String())
		}
		if expected == http.StatusTooManyRequests && response.Header().Get("Retry-After") != strconv.Itoa(int(loginWindow.Seconds())) {
			t.Fatalf("unexpected Retry-After: %q", response.Header().Get("Retry-After"))
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, loginRequest("203.0.113.7:1234", "test-password-123456"))
	if response.Code != http.StatusOK {
		t.Fatalf("different source was limited: %d %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, loginRequest("[::ffff:198.51.100.7]:4321", "test-password-123456"))
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("mapped source bypassed limit: %d %s", response.Code, response.Body.String())
	}
	now = now.Add(loginWindow)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, loginRequest("198.51.100.7:1234", "test-password-123456"))
	if response.Code != http.StatusOK {
		t.Fatalf("expired rate limit remained active: %d %s", response.Code, response.Body.String())
	}
}

func TestLoginRejectsInvalidRemoteAddress(t *testing.T) {
	_, _, handler, _, _ := newTestServer(t)
	request := loginRequest("not-an-address", "test-password-123456")
	request.Header.Set("X-Forwarded-For", "198.51.100.20")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid RemoteAddr was accepted via forwarding header: %d", response.Code)
	}
}

func TestLoginAndStateDoNotExposeHash(t *testing.T) {
	_, _, handler, _, cfg := newTestServer(t)
	cookie := loginCookie(t, handler)
	request := adminRequest(http.MethodGet, "/api/state", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("state failed: %d %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if strings.Contains(body, cfg.AdminPasswordHash) || strings.Contains(body, "admin_password_hash") {
		t.Fatal("state response exposed password hash")
	}
	if strings.Contains(body, cfg.Telemetry.Secret) || strings.Contains(body, "admin_cidr") {
		t.Fatal("state response exposed telemetry secret or deprecated admin CIDR")
	}
	if !strings.Contains(body, "vless://") || !strings.Contains(body, "hysteria2://") {
		t.Fatal("state response missing client links")
	}
}

func TestAuthenticationAndMutationChecks(t *testing.T) {
	_, applier, handler, _, _ := newTestServer(t)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, adminRequest(http.MethodGet, "/api/state", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", response.Code)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, adminRequest(http.MethodPost, "/api/login", []byte(`{"password":"wrong-password"}`)))
	if response.Code != http.StatusForbidden {
		t.Fatalf("mutation without custom header should fail, got %d", response.Code)
	}
	cookie := loginCookie(t, handler)
	request := adminRequest(http.MethodPost, "/api/config", []byte(`{}`))
	request.AddCookie(cookie)
	request.Header.Set("X-Requested-With", "fetch")
	request.Header.Set("Origin", "https://evil.example")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || applier.calls != 0 {
		t.Fatalf("cross-origin update was not blocked: %d", response.Code)
	}
}

func TestUpdatePreservesSecurityFields(t *testing.T) {
	_, applier, handler, panelStore, original := newTestServer(t)
	cookie := loginCookie(t, handler)
	input := updateRequest{
		PublicHost: "203.0.113.30",
		Reality:    original.Reality,
		Hysteria2:  original.Hysteria2,
	}
	input.Hysteria2.Domain = "evil.example"
	input.Hysteria2.CertFile = "/etc/hy2panel/certs/evil.crt"
	input.Hysteria2.KeyFile = "/etc/hy2panel/certs/evil.key"
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	request := adminRequest(http.MethodPost, "/api/config", body)
	request.AddCookie(cookie)
	request.Header.Set("X-Requested-With", "fetch")
	request.Header.Set("Origin", "https://panel.example.com:8443")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("update failed: %d %s", response.Code, response.Body.String())
	}
	if applier.calls != 1 || strings.Contains(string(applier.data), "evil.example") {
		t.Fatal("unsafe certificate fields reached sing-box")
	}
	stored, err := panelStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if stored.Panel.PublicHost != "203.0.113.30" || stored.Hysteria2.Domain != original.Hysteria2.Domain || stored.AdminPasswordHash != original.AdminPasswordHash {
		t.Fatal("update did not preserve protected fields")
	}
}

func TestFailedApplyDoesNotSaveConfig(t *testing.T) {
	_, applier, handler, panelStore, original := newTestServer(t)
	applier.err = errors.New("simulated failure")
	cookie := loginCookie(t, handler)
	body, err := json.Marshal(updateRequest{PublicHost: "203.0.113.30", Reality: original.Reality, Hysteria2: original.Hysteria2})
	if err != nil {
		t.Fatal(err)
	}
	request := adminRequest(http.MethodPost, "/api/config", body)
	request.AddCookie(cookie)
	request.Header.Set("X-Requested-With", "fetch")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected bad gateway, got %d", response.Code)
	}
	stored, err := panelStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if stored.Panel.PublicHost != original.Panel.PublicHost {
		t.Fatal("failed apply changed stored config")
	}
}

func TestStaticUIAvailableToAdmin(t *testing.T) {
	_, _, handler, _, _ := newTestServer(t)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, adminRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `<div id="app"></div>`) {
		t.Fatalf("static UI unavailable: %d", response.Code)
	}
}
