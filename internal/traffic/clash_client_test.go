package traffic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClashClientFetch(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.Method != http.MethodGet || request.URL.Path != "/api/connections" {
			t.Errorf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-secret" {
			t.Errorf("unexpected authorization header")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{
			"downloadTotal": 22,
			"uploadTotal": 11,
			"connections": [{
				"id": "flow-1",
				"metadata": {
					"type": "vless/reality",
					"sourceIP": "192.0.2.10",
					"host": "private.example"
				},
				"upload": 7,
				"download": 9,
				"start": "2026-11-16T01:02:03.123Z",
				"rule": "private-rule"
			}]
		}`)
	}))
	defer server.Close()

	client, err := NewClashClientWithSource(server.URL+"/api", "test-secret", SourceIdentity{Instance: "node", Epoch: "epoch-1"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := client.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 || snapshot.UploadTotal != 11 || snapshot.DownloadTotal != 22 || len(snapshot.Connections) != 1 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	connection := snapshot.Connections[0]
	if connection.ID != "flow-1" || connection.Metadata.Type != "vless/reality" || connection.Upload != 7 || connection.Download != 9 {
		t.Fatalf("unexpected connection: %#v", connection)
	}
	if snapshot.Source.Epoch != "epoch-1" {
		t.Fatalf("source identity was not attached: %#v", snapshot.Source)
	}
}

func TestClashClientRejectsUnsafeBaseURLs(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "empty", url: ""},
		{name: "remote IPv4", url: "http://192.0.2.1:9090"},
		{name: "remote IPv6", url: "http://[2001:db8::1]:9090"},
		{name: "hostname", url: "http://localhost:9090"},
		{name: "userinfo", url: "http://user@127.0.0.1:9090"},
		{name: "query", url: "http://127.0.0.1:9090?x=1"},
		{name: "fragment", url: "http://127.0.0.1:9090#x"},
		{name: "unsupported scheme", url: "file:///connections"},
		{name: "malformed port", url: "http://127.0.0.1:bad"},
		{name: "zero port", url: "http://127.0.0.1:0"},
		{name: "large port", url: "http://127.0.0.1:65536"},
		{name: "empty port", url: "http://127.0.0.1:"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewClashClient(test.url, "secret"); err == nil {
				t.Fatalf("expected %q to be rejected", test.url)
			}
		})
	}
	for _, value := range []string{"http://127.0.0.1:9090", "https://[::1]:9090/api"} {
		t.Run("accept "+value, func(t *testing.T) {
			if _, err := NewClashClient(value, "secret"); err != nil {
				t.Fatalf("expected loopback URL to pass: %v", err)
			}
		})
	}
	if _, err := NewClashClient("http://127.0.0.1:9090", " "); err == nil {
		t.Fatal("expected empty bearer token to fail")
	}
	if _, err := NewClashClientWithSource("http://127.0.0.1:9090", "secret", SourceIdentity{Epoch: strings.Repeat("x", 257)}); err == nil {
		t.Fatal("expected oversized source identity to fail")
	}
	if _, err := NewClashClient("http://127.0.0.1:9090", strings.Repeat("x", 4097)); err == nil {
		t.Fatal("expected oversized bearer token to fail")
	}
}

func TestClashClientDisablesEnvironmentProxy(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("environment proxy must not receive Clash API requests")
	}))
	defer proxy.Close()
	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("HTTPS_PROXY", proxy.URL)
	t.Setenv("NO_PROXY", "")

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, `{"downloadTotal":0,"uploadTotal":0,"connections":[]}`)
	}))
	defer server.Close()
	client, err := NewClashClient(server.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestClashClientRejectsRedirect(t *testing.T) {
	var targetRequests atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		targetRequests.Add(1)
		_, _ = io.WriteString(writer, `{"downloadTotal":0,"uploadTotal":0,"connections":[]}`)
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL+"/connections", http.StatusFound)
	}))
	defer redirector.Close()
	client, err := NewClashClient(redirector.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Fetch(context.Background()); err == nil || !strings.Contains(err.Error(), "HTTP 302") {
		t.Fatalf("expected redirect rejection, got %v", err)
	}
	if targetRequests.Load() != 0 {
		t.Fatal("redirect target was contacted")
	}
}

func TestClashClientResponseValidation(t *testing.T) {
	validStart := "2026-11-16T01:02:03Z"
	tests := []struct {
		name string
		body string
	}{
		{name: "negative global", body: `{"downloadTotal":0,"uploadTotal":-1,"connections":[]}`},
		{name: "negative connection", body: fmt.Sprintf(`{"downloadTotal":0,"uploadTotal":0,"connections":[{"id":"x","metadata":{"type":"vless/reality"},"upload":-1,"download":0,"start":%q}]}`, validStart)},
		{name: "float", body: `{"downloadTotal":0,"uploadTotal":1.2,"connections":[]}`},
		{name: "numeric string", body: `{"downloadTotal":0,"uploadTotal":"1","connections":[]}`},
		{name: "missing total", body: `{"downloadTotal":0,"connections":[]}`},
		{name: "missing connections", body: `{"downloadTotal":0,"uploadTotal":0}`},
		{name: "null connections", body: `{"downloadTotal":0,"uploadTotal":0,"connections":null}`},
		{name: "duplicate id", body: fmt.Sprintf(`{"downloadTotal":0,"uploadTotal":0,"connections":[{"id":"x","metadata":{"type":"vless/reality"},"upload":0,"download":0,"start":%q},{"id":"x","metadata":{"type":"hysteria2/hysteria2"},"upload":0,"download":0,"start":%q}]}`, validStart, validStart)},
		{name: "bad start", body: `{"downloadTotal":0,"uploadTotal":0,"connections":[{"id":"x","metadata":{"type":"vless/reality"},"upload":0,"download":0,"start":"yesterday"}]}`},
		{name: "empty id", body: fmt.Sprintf(`{"downloadTotal":0,"uploadTotal":0,"connections":[{"id":"","metadata":{"type":"vless/reality"},"upload":0,"download":0,"start":%q}]}`, validStart)},
		{name: "missing type", body: fmt.Sprintf(`{"downloadTotal":0,"uploadTotal":0,"connections":[{"id":"x","metadata":{},"upload":0,"download":0,"start":%q}]}`, validStart)},
		{name: "trailing JSON", body: `{"downloadTotal":0,"uploadTotal":0,"connections":[]} {}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(writer, test.body)
			}))
			defer server.Close()
			client, err := NewClashClient(server.URL, "secret")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.Fetch(context.Background()); err == nil {
				t.Fatal("expected invalid response to fail")
			}
		})
	}
}

func TestClashClientResponseLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(make([]byte, maxClashResponse+1))
	}))
	defer server.Close()
	client, err := NewClashClient(server.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Fetch(context.Background()); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected response size failure, got %v", err)
	}
}

func TestClashClientTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(3 * time.Second)
		_, _ = io.WriteString(writer, `{"downloadTotal":0,"uploadTotal":0,"connections":[]}`)
	})}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()
	client, err := NewClashClient("http://"+listener.Addr().String(), "secret")
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if _, err := client.Fetch(context.Background()); err == nil {
		t.Fatal("expected timeout")
	}
	if elapsed := time.Since(started); elapsed > 2800*time.Millisecond {
		t.Fatalf("timeout took too long: %v", elapsed)
	}
}

func TestClashClientHTTPErrorDoesNotExposeBody(t *testing.T) {
	secretBody := "private.example source=192.0.2.1"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, secretBody, http.StatusInternalServerError)
	}))
	defer server.Close()
	client, err := NewClashClient(server.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Fetch(context.Background())
	if err == nil || strings.Contains(err.Error(), secretBody) || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("unexpected safe HTTP error: %v", err)
	}
}

func TestDecodeClashSnapshotNoSensitiveModelFields(t *testing.T) {
	snapshot, err := decodeClashSnapshot([]byte(`{
		"downloadTotal": 2,
		"uploadTotal": 1,
		"connections": [{
			"id":"id", "metadata":{"type":"vless/reality","sourceIP":"192.0.2.1","host":"secret.example"},
			"upload":1,"download":2,"start":"2026-11-16T00:00:00Z","rule":"secret-rule"
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	file := NewStore(t.TempDir() + string(os.PathSeparator) + "traffic.json")
	state := newPersistentState()
	state.Initialized = true
	state.Open = snapshotOpenFlows(snapshot)
	if err := file.Save(state); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file.Path())
	if err != nil {
		t.Fatal(err)
	}
	for _, sensitive := range []string{"192.0.2.1", "secret.example", "secret-rule", "sourceIP", "host", "rule"} {
		if strings.Contains(string(data), sensitive) {
			t.Fatalf("persisted sensitive field %q: %s", sensitive, data)
		}
	}
}

func TestClashClientContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	defer server.Close()
	client, err := NewClashClient(server.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.Fetch(ctx)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context, got %v", err)
	}
}
