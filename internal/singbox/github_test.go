package singbox

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func expectedGitHubVersion() Version {
	return validManifest().Versions[0]
}

func githubReleasePayload(expected Version) map[string]any {
	return map[string]any{
		"tag_name":   expected.Tag,
		"draft":      false,
		"prerelease": false,
		"assets": []any{
			map[string]any{
				"name":                 expected.AssetName,
				"size":                 expected.Size,
				"digest":               "sha256:" + expected.SHA256,
				"browser_download_url": "https://github.com/SagerNet/sing-box/releases/download/" + expected.Tag + "/" + expected.AssetName,
			},
		},
	}
}

func githubTestServer(t *testing.T, handler func(http.ResponseWriter, *http.Request)) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(server.Close)
	return server
}

func writeJSONResponse(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatal(err)
	}
}

func TestGitHubResolveValidRelease(t *testing.T) {
	expected := expectedGitHubVersion()
	server := githubTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/releases/tags/v1.14.4" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/vnd.github+json" || r.Header.Get("User-Agent") == "" {
			t.Errorf("missing GitHub headers: %#v", r.Header)
		}
		writeJSONResponse(t, w, githubReleasePayload(expected))
	})

	resolved, err := (GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}).Resolve(context.Background(), expected)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Version != expected.Version || resolved.Tag != expected.Tag || resolved.Name != expected.AssetName || resolved.Size != expected.Size || resolved.SHA256 != expected.SHA256 {
		t.Fatalf("unexpected resolved asset: %#v", resolved)
	}
	if resolved.DownloadURL != "https://github.com/SagerNet/sing-box/releases/download/v1.14.4/sing-box-1.14.4-linux-amd64.tar.gz" {
		t.Fatalf("unexpected download URL %q", resolved.DownloadURL)
	}
}

func TestGitHubResolveAcceptsNullOrEmptyDigest(t *testing.T) {
	expected := expectedGitHubVersion()
	for _, test := range []struct {
		name   string
		digest any
	}{
		{name: "null", digest: nil},
		{name: "empty", digest: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			payload := githubReleasePayload(expected)
			payload["assets"].([]any)[0].(map[string]any)["digest"] = test.digest
			server := githubTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(t, w, payload)
			})
			if _, err := (GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}).Resolve(context.Background(), expected); err != nil {
				t.Fatalf("null/empty digest should defer to post-download verification: %v", err)
			}
		})
	}
}

func TestGitHubResolveRejectsReleaseAndAssetMismatch(t *testing.T) {
	expected := expectedGitHubVersion()
	tests := map[string]func(map[string]any){
		"tag mismatch": func(payload map[string]any) { payload["tag_name"] = "v1.14.3" },
		"empty tag":    func(payload map[string]any) { payload["tag_name"] = "" },
		"draft":        func(payload map[string]any) { payload["draft"] = true },
		"prerelease":   func(payload map[string]any) { payload["prerelease"] = true },
		"missing asset": func(payload map[string]any) {
			payload["assets"].([]any)[0].(map[string]any)["name"] = "other.tar.gz"
		},
		"size mismatch": func(payload map[string]any) {
			payload["assets"].([]any)[0].(map[string]any)["size"] = expected.Size + 1
		},
		"digest mismatch": func(payload map[string]any) {
			payload["assets"].([]any)[0].(map[string]any)["digest"] = "sha256:" + strings.Repeat("0", 64)
		},
		"digest algorithm": func(payload map[string]any) {
			payload["assets"].([]any)[0].(map[string]any)["digest"] = "sha512:" + expected.SHA256
		},
		"duplicate asset": func(payload map[string]any) {
			asset := payload["assets"].([]any)[0]
			payload["assets"] = append(payload["assets"].([]any), asset)
		},
		"untrusted host": func(payload map[string]any) {
			payload["assets"].([]any)[0].(map[string]any)["browser_download_url"] = "https://github.com.evil.example/SagerNet/sing-box/releases/download/v1.14.4/" + expected.AssetName
		},
		"wrong repository": func(payload map[string]any) {
			payload["assets"].([]any)[0].(map[string]any)["browser_download_url"] = "https://github.com/attacker/sing-box/releases/download/v1.14.4/" + expected.AssetName
		},
		"wrong URL tag": func(payload map[string]any) {
			payload["assets"].([]any)[0].(map[string]any)["browser_download_url"] = "https://github.com/SagerNet/sing-box/releases/download/v1.14.3/" + expected.AssetName
		},
		"wrong URL asset": func(payload map[string]any) {
			payload["assets"].([]any)[0].(map[string]any)["browser_download_url"] = "https://github.com/SagerNet/sing-box/releases/download/v1.14.4/other.tar.gz"
		},
		"URL query": func(payload map[string]any) {
			asset := payload["assets"].([]any)[0].(map[string]any)
			asset["browser_download_url"] = asset["browser_download_url"].(string) + "?next=evil"
		},
		"HTTP URL": func(payload map[string]any) {
			asset := payload["assets"].([]any)[0].(map[string]any)
			asset["browser_download_url"] = strings.Replace(asset["browser_download_url"].(string), "https://", "http://", 1)
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			payload := githubReleasePayload(expected)
			mutate(payload)
			server := githubTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(t, w, payload)
			})
			if _, err := (GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}).Resolve(context.Background(), expected); err == nil {
				t.Fatal("expected resolution to fail")
			}
		})
	}
}

func TestGitHubResolveRejectsBadHTTPResponses(t *testing.T) {
	expected := expectedGitHubVersion()
	tests := map[string]func(http.ResponseWriter){
		"status":               func(w http.ResponseWriter) { http.Error(w, "not found", http.StatusNotFound) },
		"malformed JSON":       func(w http.ResponseWriter) { _, _ = io.WriteString(w, `{`) },
		"multiple JSON values": func(w http.ResponseWriter) { _, _ = io.WriteString(w, `{}`+"\n"+`{}`) },
		"oversized":            func(w http.ResponseWriter) { _, _ = io.WriteString(w, strings.Repeat(" ", maxGitHubResponseBytes+1)) },
	}
	for name, respond := range tests {
		t.Run(name, func(t *testing.T) {
			server := githubTestServer(t, func(w http.ResponseWriter, _ *http.Request) { respond(w) })
			if _, err := (GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}).Resolve(context.Background(), expected); err == nil {
				t.Fatal("expected resolution to fail")
			}
		})
	}
}

func TestGitHubResolveTimeoutAndContext(t *testing.T) {
	expected := expectedGitHubVersion()
	blockingTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	client := GitHubClient{
		HTTPClient: &http.Client{Transport: blockingTransport},
		BaseURL:    "https://api.github.test/repository",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := client.Resolve(ctx, expected); err == nil {
		t.Fatal("expected context timeout")
	}
	if _, err := client.Resolve(nil, expected); err == nil {
		t.Fatal("expected nil context rejection")
	}
}

func TestGitHubResolveRejectsInvalidExpectedEntryAndBaseURL(t *testing.T) {
	expected := expectedGitHubVersion()
	invalid := expected
	invalid.AssetName = "user-controlled.tar.gz"
	if _, err := (GitHubClient{}).Resolve(context.Background(), invalid); err == nil {
		t.Fatal("expected invalid manifest entry rejection before network access")
	}
	for _, baseURL := range []string{"", "://bad", "file:///tmp/release", "https://user@example.com/repo", "https://example.com/repo?x=1"} {
		if baseURL == "" {
			continue // empty is the valid production default
		}
		if _, err := (GitHubClient{BaseURL: baseURL}).Resolve(context.Background(), expected); err == nil {
			t.Fatalf("expected base URL %q to fail", baseURL)
		}
	}
	for _, tag := range []string{"1.14.4", "v", "vv1.14.4", "v1.14.4/extra"} {
		if _, err := releaseEndpoint("https://api.github.com/repository", tag); err == nil {
			t.Fatalf("expected tag %q to fail", tag)
		}
	}
}

func TestValidateDownloadURLObjectHost(t *testing.T) {
	expected := expectedGitHubVersion()
	// Signed object URLs do not always retain the asset name in an opaque path;
	// this resolver accepts only the stricter form that remains bound by name.
	valid := "https://objects.githubusercontent.com/releases/" + expected.AssetName + "?X-Amz-Signature=abc"
	if err := ValidateDownloadURL(valid, expected); err != nil {
		t.Fatalf("valid official object URL rejected: %v", err)
	}
	invalid := []string{
		"https://objects.githubusercontent.com/releases/" + expected.AssetName,
		"https://objects.githubusercontent.com/releases/" + expected.AssetName + "?%zz",
		"https://objects.githubusercontent.com/releases/other.tar.gz?signature=x",
		"https://objects.githubusercontent.com.evil.example/releases/" + expected.AssetName + "?signature=x",
		"https://GITHUB.COM/SagerNet/sing-box/releases/download/" + expected.Tag + "/" + expected.AssetName,
		"https://user@github.com/SagerNet/sing-box/releases/download/" + expected.Tag + "/" + expected.AssetName,
		"https://github.com:443/SagerNet/sing-box/releases/download/" + expected.Tag + "/" + expected.AssetName,
	}
	for _, rawURL := range invalid {
		if err := ValidateDownloadURL(rawURL, expected); err == nil {
			t.Fatalf("expected URL %q to fail", rawURL)
		}
	}
}

func TestValidateDownloadURLRejectsInvalidExpectedEntry(t *testing.T) {
	expected := expectedGitHubVersion()
	expected.AssetName = "user-controlled.tar.gz"
	if err := ValidateDownloadURL("https://github.com/SagerNet/sing-box/releases/download/v1.14.4/user-controlled.tar.gz", expected); err == nil {
		t.Fatal("expected invalid manifest entry rejection")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
