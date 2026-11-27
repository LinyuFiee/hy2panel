package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hy2panel/internal/config"
)

const subscriptionAdminPath = "/api/subscription"

func newSubscriptionTestServer(t *testing.T) (*Server, http.Handler, *config.Config) {
	t.Helper()
	s, _, h, _, cfg := newTestServer(t)
	s.subscriptions = &subscriptionStore{path: filepath.Join(t.TempDir(), "private", "subscription.json")}
	return s, h, cfg
}

func subscriptionRequest(method, path, body string, cookie *http.Cookie) *http.Request {
	r := adminRequest(method, path, []byte(body))
	r.Host = "untrusted.example:9000"
	r.Header.Set("X-Forwarded-Host", "forwarded.example:1234")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if method == http.MethodPost {
		r.Header.Set("X-Requested-With", "fetch")
		r.Header.Set("Origin", "https://"+r.Host)
		r.Header.Set("Content-Type", "application/json")
	}
	return r
}

func subscriptionHTTP(t *testing.T, h http.Handler, r *http.Request, want int) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != want || w.Header().Get("Cache-Control") != "no-store" || len(w.Header().Values("Set-Cookie")) != 0 {
		t.Fatalf("response: status %d (want %d), cache control %q, set cookie %t", w.Code, want, w.Header().Get("Cache-Control"), len(w.Header().Values("Set-Cookie")) != 0)
	}
	return w
}

func subscriptionMutation(action string, cookie *http.Cookie) *http.Request {
	return subscriptionRequest(http.MethodPost, subscriptionAdminPath, `{"action":"`+action+`"}`, cookie)
}

// Assert the exact management JSON shape, not just fields known to the client.
func checkSubscriptionResponse(t *testing.T, w *httptest.ResponseRecorder, enabled, hasURL bool) string {
	t.Helper()
	var fields map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &fields); err != nil || fields["enabled"] != enabled {
		t.Fatalf("invalid subscription response; want enabled=%t", enabled)
	}
	if !hasURL {
		if len(fields) != 1 {
			t.Fatal("unexpected fields in a status-only response")
		}
		return ""
	}
	link, ok := fields["url"].(string)
	if !ok || link == "" || len(fields) != 2 {
		t.Fatal("expected only enabled and a URL")
	}
	return link
}

func subscriptionAction(t *testing.T, h http.Handler, cookie *http.Cookie, action string) string {
	t.Helper()
	w := subscriptionHTTP(t, h, subscriptionMutation(action, cookie), http.StatusOK)
	return checkSubscriptionResponse(t, w, action != "revoke", action != "revoke")
}

func checkSubscriptionStatus(t *testing.T, h http.Handler, cookie *http.Cookie, enabled bool) {
	t.Helper()
	w := subscriptionHTTP(t, h, subscriptionRequest(http.MethodGet, subscriptionAdminPath, "", cookie), http.StatusOK)
	checkSubscriptionResponse(t, w, enabled, false)
}

func TestSubscriptionManagementProtection(t *testing.T) {
	s, h, _ := newSubscriptionTestServer(t)
	subscriptionHTTP(t, h, subscriptionRequest(http.MethodGet, subscriptionAdminPath, "", nil), http.StatusUnauthorized)
	cookie := loginCookie(t, h)
	for _, action := range []string{"create", "copy", "rotate", "revoke"} {
		subscriptionHTTP(t, h, subscriptionMutation(action, nil), http.StatusUnauthorized)
		r := subscriptionMutation(action, cookie)
		r.Header.Del("X-Requested-With")
		subscriptionHTTP(t, h, r, http.StatusForbidden)
		r = subscriptionMutation(action, cookie)
		r.Header.Set("Origin", "https://foreign.example")
		subscriptionHTTP(t, h, r, http.StatusForbidden)
	}
	for _, body := range []string{"", `{}`, `{"action":"unknown"}`, `{"action":"create","token":"preset"}`, `{"action":"create"} {}`} {
		subscriptionHTTP(t, h, subscriptionRequest(http.MethodPost, subscriptionAdminPath, body, cookie), http.StatusBadRequest)
	}
	checkSubscriptionStatus(t, h, cookie, false)
	subscriptionHTTP(t, h, subscriptionMutation("copy", cookie), http.StatusConflict)
	if _, err := os.Lstat(filepath.Dir(s.subscriptions.path)); !os.IsNotExist(err) {
		t.Fatal("disabled reads or rejected mutations initialized the store")
	}
}

func TestSubscriptionLifecycleAcrossRestarts(t *testing.T) {
	s, h, cfg := newSubscriptionTestServer(t)
	now := time.Now()
	s.auth = newAuthenticatorWithClock(func() time.Time { return now })
	cookie := loginCookie(t, h)
	cfg.Panel.Listen = "[::1]:9443"
	if err := s.store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	restart := func() {
		next := New(s.store, s.applier)
		next.subscriptions = &subscriptionStore{path: s.subscriptions.path}
		s, h = next, next.routes()
	}
	link := subscriptionAction(t, h, cookie, "create")
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "https" || u.Host != "panel.example.com:9443" || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, subscriptionPrefix) {
		t.Fatal("link did not use only the configured HTTPS panel authority and bearer path")
	}
	token := strings.TrimPrefix(u.Path, subscriptionPrefix)
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) != 32 {
		t.Fatal("bearer is not 32 bytes of unpadded URL-safe base64")
	}
	checkSubscriptionStatus(t, h, cookie, true)
	for _, action := range []string{"copy", "create", "copy"} {
		if subscriptionAction(t, h, cookie, action) != link {
			t.Fatalf("%s changed the active URL", action)
		}
	}
	// Exercise the actual regular state endpoint without external lookups or
	// service commands. Neither its response nor panel.json may gain the bearer.
	s.nodeMetaHost, s.nodeMetaCachedAt, s.nodeMetaTTL = cfg.Panel.PublicHost, time.Now(), time.Hour
	t.Setenv("PATH", t.TempDir())
	state := subscriptionHTTP(t, h, subscriptionRequest(http.MethodGet, "/api/state", "", cookie), http.StatusOK)
	panel, err := os.ReadFile(s.store.Path())
	if err != nil || bytes.Contains(panel, []byte(token)) || strings.Contains(state.Body.String(), token) || strings.Contains(state.Body.String(), subscriptionPrefix) {
		t.Fatal("subscription leaked into regular state/config or config could not be read")
	}
	// Bearer and admin credentials are independent in both directions.
	subscriptionHTTP(t, h, subscriptionRequest(http.MethodGet, subscriptionAdminPath, "", &http.Cookie{Name: cookie.Name, Value: token}), http.StatusUnauthorized)
	notFound := subscriptionHTTP(t, h, subscriptionRequest(http.MethodGet, subscriptionPrefix+cookie.Value, "", cookie), http.StatusNotFound).Body.String()
	public := func(path string, want int) *httptest.ResponseRecorder {
		t.Helper()
		return subscriptionHTTP(t, h, subscriptionRequest(http.MethodGet, path, "", nil), want)
	}
	assertMissing := func(path string) {
		t.Helper()
		if public(path, http.StatusNotFound).Body.String() != notFound {
			t.Fatal("absent, malformed, rotated and revoked bearers must look identical")
		}
	}
	for _, path := range []string{
		"/api/sub", subscriptionPrefix, subscriptionPrefix + "malformed", u.Path + "/extra", u.Path + "=",
		subscriptionPrefix + "/" + token, u.Path + "/../" + token, subscriptionPrefix + "%41" + token[1:],
	} {
		assertMissing(path)
	}
	subscriptionHTTP(t, h, subscriptionRequest(http.MethodPost, u.Path, "", cookie), http.StatusMethodNotAllowed)
	public(u.Path, http.StatusOK)
	now = now.Add(sessionLifetime)
	subscriptionHTTP(t, h, subscriptionRequest(http.MethodGet, subscriptionAdminPath, "", cookie), http.StatusUnauthorized)
	subscriptionHTTP(t, h, subscriptionRequest(http.MethodGet, u.Path, "", cookie), http.StatusOK)
	restart()
	public(u.Path, http.StatusOK)
	cookie = loginCookie(t, h)
	if subscriptionAction(t, h, cookie, "copy") != link {
		t.Fatal("copy after restart changed the URL")
	}
	rotated := subscriptionAction(t, h, cookie, "rotate")
	if rotated == link {
		t.Fatal("rotation reused a bearer")
	}
	assertMissing(u.Path)
	restart()
	assertMissing(u.Path)
	public(rotated, http.StatusOK)
	cookie = loginCookie(t, h)
	if subscriptionAction(t, h, cookie, "copy") != rotated {
		t.Fatal("rotated bearer did not persist")
	}
	for range 2 {
		subscriptionAction(t, h, cookie, "revoke")
	}
	assertMissing(rotated)
	restart()
	assertMissing(rotated)
	cookie = loginCookie(t, h)
	checkSubscriptionStatus(t, h, cookie, false)
	subscriptionHTTP(t, h, subscriptionMutation("copy", cookie), http.StatusConflict)
	third := subscriptionAction(t, h, cookie, "rotate")
	if third == link || third == rotated {
		t.Fatal("rotation from disabled state revived an old bearer")
	}
	public(third, http.StatusOK)
	assertMissing(rotated)
	if err := os.Remove(s.subscriptions.path); err != nil {
		t.Fatal(err)
	}
	assertMissing(third) // Restores can invalidate a URL without any cache flush.
}

func TestSubscriptionServesCurrentNativeProfile(t *testing.T) {
	s, h, cfg := newSubscriptionTestServer(t)
	cookie := loginCookie(t, h)
	link := subscriptionAction(t, h, cookie, "create")
	for _, enabled := range [][2]bool{{true, true}, {false, true}, {false, false}, {true, false}} {
		cfg.Reality.Enabled, cfg.Hysteria2.Enabled = enabled[0], enabled[1]
		cfg.Panel.PublicHost = "203.0.113.72"
		if err := s.store.Save(cfg); err != nil {
			t.Fatal(err)
		}
		r := subscriptionRequest(http.MethodGet, link, "", nil)
		if !enabled[0] && !enabled[1] {
			subscriptionHTTP(t, h, r, http.StatusServiceUnavailable)
			subscriptionHTTP(t, h, subscriptionRequest(http.MethodGet, subscriptionPrefix+strings.Repeat("A", 43), "", cookie), http.StatusNotFound)
			continue
		}
		w := subscriptionHTTP(t, h, r, http.StatusOK)
		want, err := cfg.SingBoxClientProfile()
		var profile struct{ Outbounds []json.RawMessage }
		if err != nil || json.Unmarshal(w.Body.Bytes(), &profile) != nil || len(profile.Outbounds) == 0 || !bytes.Equal(want, w.Body.Bytes()) || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
			t.Fatal("response is not the current native JSON profile")
		}
		for _, secret := range []string{cfg.AdminPasswordHash, cfg.Telemetry.Secret, cfg.Reality.PrivateKey, link} {
			if strings.Contains(w.Body.String(), secret) {
				t.Fatal("public profile exposed a server-only secret")
			}
		}
	}
	if err := os.Remove(s.store.Path()); err != nil {
		t.Fatal(err)
	}
	subscriptionHTTP(t, h, subscriptionRequest(http.MethodGet, link, "", nil), http.StatusServiceUnavailable)
	subscriptionHTTP(t, h, subscriptionMutation("rotate", cookie), http.StatusInternalServerError)
	subscriptionAction(t, h, cookie, "revoke") // Broken panel config must not prevent revocation.
	subscriptionHTTP(t, h, subscriptionRequest(http.MethodGet, link, "", nil), http.StatusNotFound)
}

func TestSubscriptionConcurrentCreate(t *testing.T) {
	_, h, _ := newSubscriptionTestServer(t)
	cookie := loginCookie(t, h)
	const count = 32
	start := make(chan struct{})
	results := make(chan *httptest.ResponseRecorder, count)
	for range count {
		go func() {
			<-start
			w := httptest.NewRecorder()
			h.ServeHTTP(w, subscriptionMutation("create", cookie))
			results <- w
		}()
	}
	close(start)
	var first string
	for range count {
		w := <-results
		if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("concurrent create returned %d or cacheable response", w.Code)
		}
		link := checkSubscriptionResponse(t, w, true, true)
		if first != "" && first != link {
			t.Fatal("concurrent create issued different URLs")
		}
		first = link
	}
	if subscriptionAction(t, h, cookie, "copy") != first {
		t.Fatal("returned link differs from the durable token")
	}
}

func TestSubscriptionRejectsDamagedStorage(t *testing.T) {
	s, h, _ := newSubscriptionTestServer(t)
	cookie := loginCookie(t, h)
	link := subscriptionAction(t, h, cookie, "create")
	path := s.subscriptions.path
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Include partially usable input: a valid token with unsupported state
	// or trailing garbage must never authorize a download or be overwritten.
	for _, data := range []string{"", "{", "null", "{}", `{"token":"invalid"}`, strings.TrimSpace(string(original)) + "{}", strings.Replace(string(original), "{", `{"enabled":false,`, 1), string(original) + strings.Repeat(" ", 1024)} {
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		subscriptionHTTP(t, h, subscriptionRequest(http.MethodGet, link, "", nil), http.StatusNotFound)
		subscriptionHTTP(t, h, subscriptionRequest(http.MethodGet, subscriptionAdminPath, "", cookie), http.StatusInternalServerError)
		for _, action := range []string{"copy", "create", "rotate", "revoke"} {
			w := subscriptionHTTP(t, h, subscriptionMutation(action, cookie), http.StatusInternalServerError)
			if strings.Contains(w.Body.String(), filepath.Base(path)) || strings.Contains(w.Body.String(), subscriptionPrefix) {
				t.Fatal("storage error disclosed internal data")
			}
		}
		if current, err := os.ReadFile(path); err != nil || string(current) != data {
			t.Fatal("corrupt state was changed")
		}
	}
}

func TestSubscriptionRedactsBearerPaths(t *testing.T) {
	first, second := strings.Repeat("B", 42)+"A", strings.Repeat("-", 42)+"_"
	input := `GET /api/sub/` + first + ` HTTP/1.1` + "\n" + `{"url":"https://panel.example:8443/api/sub/` + second + `?v=1"}` + "\nGET /api/subscription"
	want := `GET /api/sub/[已隐藏] HTTP/1.1` + "\n" + `{"url":"https://panel.example:8443/api/sub/[已隐藏]?v=1"}` + "\nGET /api/subscription"
	if got := redactSecrets(input, &config.Config{}); got != want {
		t.Fatal("failed to redact current/historical bearer paths without changing unrelated log text")
	}
}
