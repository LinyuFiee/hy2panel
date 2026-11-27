package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"hy2panel/internal/version"
)

func TestNewUpdateAfterPreviousSuccessfulUpgrade(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "upgrade.result"), []byte(`{"schema":1,"id":"previous-upgrade","status":"succeeded","phase":"committed","target_version":"0.8.0"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v9.0.0","html_url":"https://example.test/release","assets":[{"name":"hy2panel-bundle-linux-amd64.tar.gz","size":1,"browser_download_url":"https://github.com/LinyuFiee/hy2panel/releases/download/v9.0.0/hy2panel-bundle-linux-amd64.tar.gz"}]}`))
	}))
	defer upstream.Close()
	manager := NewUpdateManager(dir)
	manager.apiURL = upstream.URL
	view, err := manager.Check(t.Context())
	if err != nil || view.CurrentVersion != version.Version || !view.Available || !manager.State().Available {
		t.Fatalf("a previous result masked the next update: %+v, %v", view, err)
	}
	if view, err = manager.StartUpgrade(); err != nil || !view.Updating || view.OperationID == "previous-upgrade" {
		t.Fatalf("new upgrade not requested: %+v, %v", view, err)
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		left, right string
		expected    int
	}{
		{"0.4.0", "0.3.1", 1},
		{"v0.4.0", "0.4.0", 0},
		{"0.3.1", "0.4.0", -1},
		{"1.0.0", "0.9.9", 1},
		{"0.4", "0.4.0", 0},
	}
	for _, test := range tests {
		if result := compareVersions(test.left, test.right); result != test.expected {
			t.Fatalf("compareVersions(%q, %q) = %d, expected %d", test.left, test.right, result, test.expected)
		}
	}
}
