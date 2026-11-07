package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestCountryFlag(t *testing.T) {
	if countryFlag("US") != "🇺🇸" || countryFlag("JP") != "🇯🇵" || countryFlag("X") != "" || countryFlag("U1") != "" {
		t.Fatal("unexpected country flag conversion")
	}
}

func TestGeoIPLookup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/8.8.8.8" {
			t.Fatalf("unexpected lookup path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"country":"United States","country_code":"US"}`))
	}))
	defer server.Close()
	client := &geoIPClient{Endpoint: server.URL, HTTPClient: server.Client()}
	meta, err := client.Lookup(context.Background(), "8.8.8.8")
	if err != nil {
		t.Fatal(err)
	}
	if meta.CountryCode != "US" || meta.RealityName != "🇺🇸 US-Reality" || meta.Hysteria2Name != "🇺🇸 US-Hysteria2" {
		t.Fatalf("unexpected node meta: %#v", meta)
	}
}

func TestReservedHostDoesNotLookup(t *testing.T) {
	client := &geoIPClient{Endpoint: "http://127.0.0.1:1"}
	if _, err := client.Lookup(context.Background(), "203.0.113.10"); err == nil {
		t.Fatal("reserved address should not be queried")
	}
}

func TestNamedClientLinks(t *testing.T) {
	meta := NodeMeta{RealityName: "🇺🇸 US-Reality", Hysteria2Name: "🇺🇸 US-Hysteria2"}
	links := namedClientLinks(map[string]string{"reality": "vless://uuid@example.com:443?security=reality#Reality", "hysteria2": "hysteria2://password@example.com:443#Hysteria2"}, meta)
	reality, err := url.Parse(links["reality"])
	if err != nil {
		t.Fatal(err)
	}
	hy2, err := url.Parse(links["hysteria2"])
	if err != nil {
		t.Fatal(err)
	}
	if reality.Fragment != meta.RealityName || hy2.Fragment != meta.Hysteria2Name {
		t.Fatalf("unexpected link names: %s %s", reality.Fragment, hy2.Fragment)
	}
}
