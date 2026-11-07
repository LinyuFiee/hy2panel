package racknerd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.FormValue("key") != "test-key" || r.FormValue("hash") != "test-hash" {
			t.Fatalf("unexpected credentials or method")
		}
		if r.FormValue("action") != "info" || r.FormValue("bw") != "1" || r.FormValue("rdtype") != "json" {
			t.Fatalf("unexpected form: %v", r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","hostname":"test-vps","vmstat":"online","bw":"3221225472000,536870912000,2684354560000,16.67"}`))
	}))
	defer server.Close()
	client := &Client{Endpoint: server.URL, HTTPClient: server.Client()}
	info, err := client.Info(context.Background(), "test-key", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	if info.BandwidthTotal != 3221225472000 || info.BandwidthUsed != 536870912000 || info.BandwidthFree != 2684354560000 || info.PercentUsed != 16.67 {
		t.Fatalf("unexpected bandwidth info: %#v", info)
	}
	if info.Hostname != "test-vps" || info.VMState != "online" || info.UpdatedAt.IsZero() {
		t.Fatalf("unexpected VPS info: %#v", info)
	}
}

func TestClientAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"error","statusmsg":"Invalid key or hash"}`))
	}))
	defer server.Close()
	client := &Client{Endpoint: server.URL, HTTPClient: server.Client()}
	if _, err := client.Info(context.Background(), "bad", "bad"); err == nil || !strings.Contains(err.Error(), "Invalid key") {
		t.Fatalf("expected API error, got %v", err)
	}
}

func TestInvalidUsageRejected(t *testing.T) {
	for _, value := range []string{"", "1,2,3", "1,2,3,4,5", "100,200,0,50", "100,0,200,50", "100,20,80,nope"} {
		if _, _, _, _, err := parseUsage(value); err == nil {
			t.Fatalf("expected %q to fail", value)
		}
	}
}
