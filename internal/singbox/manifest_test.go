package singbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func repositoryManifestPath(t *testing.T) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test source path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "packaging", "sing-box-versions.json"))
}

func validManifest() Manifest {
	return Manifest{
		Schema:   ManifestSchema,
		Platform: ManifestPlatform,
		Versions: []Version{
			{
				Version:    "1.14.4",
				Tag:        "v1.14.4",
				Status:     StatusRecommended,
				AssetName:  "sing-box-1.14.4-linux-amd64.tar.gz",
				Size:       31710753,
				SHA256:     "699a1b510aa4396b2176d32d1181920399ebf1e267835da6f3b598d0f7c24968",
				ReleaseURL: "https://github.com/SagerNet/sing-box/releases/tag/v1.14.4",
			},
			{
				Version:    "1.14.3",
				Tag:        "v1.14.3",
				Status:     StatusSupported,
				AssetName:  "sing-box-1.14.3-linux-amd64.tar.gz",
				Size:       31678266,
				SHA256:     "e2bdf179c15a3652955dc44867e33ca0965c9c9b91b34267dcc5a5d639a5feee",
				ReleaseURL: "https://github.com/SagerNet/sing-box/releases/tag/v1.14.3",
			},
		},
	}
}

func cloneManifest(t *testing.T, source Manifest) Manifest {
	t.Helper()
	data, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	var clone Manifest
	if err := json.Unmarshal(data, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func TestLoadRepositoryManifest(t *testing.T) {
	manifest, err := LoadFile(repositoryManifestPath(t))
	if err != nil {
		t.Fatal(err)
	}
	expectedVersions := []Version{
		{
			Version:    "1.14.4",
			Tag:        "v1.14.4",
			Status:     StatusRecommended,
			AssetName:  "sing-box-1.14.4-linux-amd64.tar.gz",
			Size:       31710753,
			SHA256:     "699a1b510aa4396b2176d32d1181920399ebf1e267835da6f3b598d0f7c24968",
			ReleaseURL: "https://github.com/SagerNet/sing-box/releases/tag/v1.14.4",
		},
		{
			Version:    "1.14.3",
			Tag:        "v1.14.3",
			Status:     StatusSupported,
			AssetName:  "sing-box-1.14.3-linux-amd64.tar.gz",
			Size:       31678266,
			SHA256:     "e2bdf179c15a3652955dc44867e33ca0965c9c9b91b34267dcc5a5d639a5feee",
			ReleaseURL: "https://github.com/SagerNet/sing-box/releases/tag/v1.14.3",
		},
		{
			Version:    "1.14.2",
			Tag:        "v1.14.2",
			Status:     StatusSupported,
			AssetName:  "sing-box-1.14.2-linux-amd64.tar.gz",
			Size:       31680686,
			SHA256:     "a684484d7477d1437282ee411f4d131d0340aaad60a7868841ebd5d87dd8a0c6",
			ReleaseURL: "https://github.com/SagerNet/sing-box/releases/tag/v1.14.2",
		},
	}
	if !reflect.DeepEqual(manifest.Versions, expectedVersions) {
		t.Fatalf("repository manifest versions = %#v, want %#v", manifest.Versions, expectedVersions)
	}
	recommended, err := manifest.Recommended()
	if err != nil {
		t.Fatal(err)
	}
	if recommended.Version != "1.14.4" || recommended.Size != 31710753 {
		t.Fatalf("unexpected recommended release: %#v", recommended)
	}
	if entry, ok := manifest.Find("v1.14.2"); !ok || entry.SHA256 != "a684484d7477d1437282ee411f4d131d0340aaad60a7868841ebd5d87dd8a0c6" {
		t.Fatalf("unexpected 1.14.2 release: %#v, found=%v", entry, ok)
	}
	if _, ok := manifest.Find("1.14"); ok {
		t.Fatal("partial version unexpectedly matched")
	}

	public, err := manifest.Public()
	if err != nil {
		t.Fatal(err)
	}
	if public.RecommendedVersion != "1.14.4" || len(public.Versions) != 3 || public.Versions[0].AssetName != recommended.AssetName {
		t.Fatalf("unexpected public view: %#v", public)
	}
	public.Versions[0].Version = "mutated"
	if manifest.Versions[0].Version != "1.14.4" {
		t.Fatal("public view aliases the manifest")
	}
}

func TestLoadRejectsInvalidJSONEnvelope(t *testing.T) {
	valid := validManifest()
	data, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]string{
		"unknown root field":  strings.TrimSuffix(string(data), "}") + `,"unexpected":true}`,
		"unknown entry field": strings.Replace(string(data), `"version":"1.14.4"`, `"version":"1.14.4","unexpected":true`, 1),
		"duplicate root key":  strings.Replace(string(data), `"schema":1`, `"schema":1,"schema":1`, 1),
		"duplicate entry key": strings.Replace(string(data), `"version":"1.14.4"`, `"version":"1.14.4","version":"1.14.4"`, 1),
		"multiple values":     string(data) + `{}`,
		"empty":               "",
		"malformed":           `{`,
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(strings.NewReader(payload)); err == nil {
				t.Fatal("expected load to fail")
			}
		})
	}

	t.Run("oversized", func(t *testing.T) {
		if _, err := Load(strings.NewReader(strings.Repeat(" ", maxManifestBytes+1))); err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("expected size error, got %v", err)
		}
	})
	t.Run("nil reader", func(t *testing.T) {
		if _, err := Load(nil); err == nil {
			t.Fatal("expected nil reader to fail")
		}
	})
}

func TestLoadFileErrors(t *testing.T) {
	if _, err := LoadFile(""); err == nil {
		t.Fatal("expected empty path to fail")
	}
	if _, err := LoadFile(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected missing file to fail")
	}

	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(path); err == nil {
		t.Fatal("expected invalid file to fail")
	}
}

func TestManifestValidateRejectsInvalidCases(t *testing.T) {
	upperDigest := "699A1B510AA4396B2176D32D1181920399EBF1E267835DA6F3B598D0F7C24968"
	third := validManifest().Versions[1]
	third.Version = "1.14.2"
	third.Tag = "v1.14.2"
	third.AssetName = "sing-box-1.14.2-linux-amd64.tar.gz"
	third.Size = 31680686
	third.SHA256 = "a684484d7477d1437282ee411f4d131d0340aaad60a7868841ebd5d87dd8a0c6"
	third.ReleaseURL = "https://github.com/SagerNet/sing-box/releases/tag/v1.14.2"
	tests := map[string]func(*Manifest){
		"schema":                  func(m *Manifest) { m.Schema = 2 },
		"platform":                func(m *Manifest) { m.Platform = "linux-arm64" },
		"null versions":           func(m *Manifest) { m.Versions = nil },
		"empty versions":          func(m *Manifest) { m.Versions = []Version{} },
		"no recommended":          func(m *Manifest) { m.Versions[0].Status = StatusSupported },
		"two recommended":         func(m *Manifest) { m.Versions[1].Status = StatusRecommended },
		"status allowlist":        func(m *Manifest) { m.Versions[1].Status = "deprecated" },
		"version missing part":    func(m *Manifest) { m.Versions[1].Version = "1.14" },
		"version leading v":       func(m *Manifest) { m.Versions[1].Version = "v1.14.3" },
		"version leading zero":    func(m *Manifest) { m.Versions[1].Version = "1.014.3" },
		"version prerelease":      func(m *Manifest) { m.Versions[1].Version = "1.14.3-rc.1" },
		"tag mismatch":            func(m *Manifest) { m.Versions[1].Tag = "v1.14.2" },
		"asset version mismatch":  func(m *Manifest) { m.Versions[1].AssetName = "sing-box-1.14.2-linux-amd64.tar.gz" },
		"asset platform mismatch": func(m *Manifest) { m.Versions[1].AssetName = "sing-box-1.14.3-linux-arm64.tar.gz" },
		"asset path":              func(m *Manifest) { m.Versions[1].AssetName = "../sing-box-1.14.3-linux-amd64.tar.gz" },
		"size zero":               func(m *Manifest) { m.Versions[1].Size = 0 },
		"size negative":           func(m *Manifest) { m.Versions[1].Size = -1 },
		"size too small":          func(m *Manifest) { m.Versions[1].Size = minAssetSize - 1 },
		"size too large":          func(m *Manifest) { m.Versions[1].Size = maxAssetSize + 1 },
		"digest short":            func(m *Manifest) { m.Versions[1].SHA256 = "abcd" },
		"digest uppercase":        func(m *Manifest) { m.Versions[0].SHA256 = upperDigest },
		"digest nonhex":           func(m *Manifest) { m.Versions[1].SHA256 = strings.Repeat("z", 64) },
		"release scheme": func(m *Manifest) {
			m.Versions[1].ReleaseURL = "http://github.com/SagerNet/sing-box/releases/tag/v1.14.3"
		},
		"release host": func(m *Manifest) {
			m.Versions[1].ReleaseURL = "https://github.com.example/SagerNet/sing-box/releases/tag/v1.14.3"
		},
		"release path": func(m *Manifest) { m.Versions[1].ReleaseURL = "https://github.com/attacker/repo/releases/tag/v1.14.3" },
		"release tag": func(m *Manifest) {
			m.Versions[1].ReleaseURL = "https://github.com/SagerNet/sing-box/releases/tag/v1.14.2"
		},
		"release query":    func(m *Manifest) { m.Versions[1].ReleaseURL += "?redirect=evil" },
		"release fragment": func(m *Manifest) { m.Versions[1].ReleaseURL += "#asset" },
		"duplicate version": func(m *Manifest) {
			duplicate := third
			duplicate.Version = m.Versions[0].Version
			duplicate.Tag = m.Versions[0].Tag
			duplicate.AssetName = "sing-box-1.14.2-linux-amd64.tar.gz"
			duplicate.ReleaseURL = m.Versions[0].ReleaseURL
			m.Versions = append(m.Versions, duplicate)
		},
		"duplicate asset": func(m *Manifest) {
			duplicate := third
			duplicate.AssetName = m.Versions[1].AssetName
			m.Versions = append(m.Versions, duplicate)
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			manifest := cloneManifest(t, validManifest())
			mutate(&manifest)
			err := manifest.Validate()
			if err == nil {
				t.Fatalf("expected invalid manifest: %#v", manifest)
			}
			if strings.HasPrefix(name, "duplicate ") && !strings.Contains(err.Error(), name) {
				t.Fatalf("expected explicit %s error, got %v", name, err)
			}
		})
	}
}

func TestManifestAPIsRejectUnvalidatedManifest(t *testing.T) {
	manifest := validManifest()
	manifest.Platform = "windows-amd64"
	if _, err := manifest.Recommended(); err == nil {
		t.Fatal("Recommended accepted invalid manifest")
	}
	if _, err := manifest.Public(); err == nil {
		t.Fatal("Public accepted invalid manifest")
	}
	if _, ok := manifest.Find("1.14.4"); ok {
		t.Fatal("Find accepted invalid manifest")
	}
}
