package singbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
)

const (
	ManifestSchema   = 1
	ManifestPlatform = "linux-amd64"

	StatusRecommended = "recommended"
	StatusSupported   = "supported"

	maxManifestBytes = 1 << 20
	minAssetSize     = 1 << 20
	maxAssetSize     = 1 << 30
)

var (
	digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	assetPattern  = regexp.MustCompile(`^sing-box-([0-9]+\.[0-9]+\.[0-9]+)-linux-amd64\.tar\.gz$`)
)

// Manifest is the trusted sing-box compatibility allowlist. Unknown JSON fields
// are rejected by Load and LoadFile so schema changes cannot be silently ignored.
type Manifest struct {
	Schema   int       `json:"schema"`
	Platform string    `json:"platform"`
	Versions []Version `json:"versions"`
}

// Version identifies one immutable upstream release asset.
type Version struct {
	Version    string `json:"version"`
	Tag        string `json:"tag"`
	Status     string `json:"status"`
	AssetName  string `json:"asset_name"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
	ReleaseURL string `json:"release_url"`
}

// PublicVersion is the non-sensitive representation intended for API clients.
// It excludes any resolved download URL; downloads must be resolved from GitHub
// and revalidated for each operation.
type PublicVersion struct {
	Version    string `json:"version"`
	Tag        string `json:"tag"`
	Status     string `json:"status"`
	AssetName  string `json:"asset_name"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
	ReleaseURL string `json:"release_url"`
}

// PublicManifest is the API-safe view of a validated manifest.
type PublicManifest struct {
	Schema             int             `json:"schema"`
	Platform           string          `json:"platform"`
	RecommendedVersion string          `json:"recommended_version"`
	Versions           []PublicVersion `json:"versions"`
}

// Load parses and validates a manifest from r.
func Load(r io.Reader) (*Manifest, error) {
	if r == nil {
		return nil, errors.New("sing-box manifest reader is nil")
	}

	limited := &io.LimitedReader{R: r, N: maxManifestBytes + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read sing-box manifest: %w", err)
	}
	if len(data) > maxManifestBytes {
		return nil, fmt.Errorf("sing-box manifest exceeds %d bytes", maxManifestBytes)
	}

	var manifest Manifest
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return nil, fmt.Errorf("decode sing-box manifest: %w", err)
	}
	if err := decodeManifestJSON(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode sing-box manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// LoadFile opens, parses, and validates a manifest. The packaging manifest is
// outside this internal package's directory tree, so it cannot be embedded
// directly with go:embed; callers should pass its installed or repository path.
func LoadFile(path string) (*Manifest, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("sing-box manifest path is empty")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open sing-box manifest: %w", err)
	}
	defer file.Close()
	return Load(file)
}

func decodeManifestJSON(data []byte, manifest *Manifest) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(manifest); err != nil {
		return err
	}
	return ensureJSONEOF(decoder)
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	return ensureJSONEOF(decoder)
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return nil
	}

	switch delimiter {
	case '{':
		keys := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("JSON object key is not a string")
			}
			if _, exists := keys[key]; exists {
				return fmt.Errorf("duplicate JSON object key %q", key)
			}
			keys[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim('}') {
			return errors.New("JSON object is not properly terminated")
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim(']') {
			return errors.New("JSON array is not properly terminated")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("decode sing-box manifest: multiple JSON values")
		}
		return fmt.Errorf("decode sing-box manifest: %w", err)
	}
	return nil
}

// Validate enforces all invariants needed to trust the allowlist.
func (m Manifest) Validate() error {
	if m.Schema != ManifestSchema {
		return fmt.Errorf("unsupported sing-box manifest schema %d", m.Schema)
	}
	if m.Platform != ManifestPlatform {
		return fmt.Errorf("unsupported sing-box manifest platform %q", m.Platform)
	}
	if m.Versions == nil {
		return errors.New("sing-box manifest versions must be an array")
	}
	if len(m.Versions) == 0 {
		return errors.New("sing-box manifest contains no versions")
	}

	versions := make(map[string]struct{}, len(m.Versions))
	assets := make(map[string]struct{}, len(m.Versions))
	for _, entry := range m.Versions {
		if _, exists := versions[entry.Version]; exists {
			return fmt.Errorf("sing-box manifest contains duplicate version %q", entry.Version)
		}
		versions[entry.Version] = struct{}{}
		if _, exists := assets[entry.AssetName]; exists {
			return fmt.Errorf("sing-box manifest contains duplicate asset %q", entry.AssetName)
		}
		assets[entry.AssetName] = struct{}{}
	}

	recommended := 0
	for index, entry := range m.Versions {
		if err := validateManifestVersion(entry); err != nil {
			return fmt.Errorf("sing-box manifest versions[%d]: %w", index, err)
		}
		if entry.Status == StatusRecommended {
			recommended++
		}
	}
	if recommended != 1 {
		return fmt.Errorf("sing-box manifest must contain exactly one recommended version, found %d", recommended)
	}
	return nil
}

func validateManifestVersion(entry Version) error {
	canonical, err := NormalizeVersion(entry.Version)
	if err != nil || canonical != entry.Version {
		return fmt.Errorf("invalid canonical version %q", entry.Version)
	}
	if entry.Tag != "v"+entry.Version {
		return fmt.Errorf("tag %q does not match version %q", entry.Tag, entry.Version)
	}
	if entry.Status != StatusRecommended && entry.Status != StatusSupported {
		return fmt.Errorf("unsupported status %q", entry.Status)
	}

	matches := assetPattern.FindStringSubmatch(entry.AssetName)
	if len(matches) != 2 || matches[1] != entry.Version {
		return fmt.Errorf("asset name %q does not match version and platform", entry.AssetName)
	}
	if entry.Size < minAssetSize || entry.Size > maxAssetSize {
		return fmt.Errorf("asset size %d is outside allowed range [%d, %d]", entry.Size, minAssetSize, maxAssetSize)
	}
	if !digestPattern.MatchString(entry.SHA256) {
		return errors.New("sha256 must be exactly 64 lowercase hexadecimal characters")
	}
	if err := validateReleaseURL(entry.ReleaseURL, entry.Tag); err != nil {
		return err
	}
	return nil
}

func validateReleaseURL(rawURL, tag string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid release_url: %w", err)
	}
	expectedPath := "/SagerNet/sing-box/releases/tag/" + tag
	if parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.RawPath != "" || parsed.ForceQuery || parsed.Path != expectedPath {
		return fmt.Errorf("release_url %q is not the canonical GitHub release URL for %q", rawURL, tag)
	}
	return nil
}

// Recommended returns the sole recommended release from a validated manifest.
func (m Manifest) Recommended() (Version, error) {
	if err := m.Validate(); err != nil {
		return Version{}, err
	}
	for _, entry := range m.Versions {
		if entry.Status == StatusRecommended {
			return entry, nil
		}
	}
	return Version{}, errors.New("sing-box manifest has no recommended version")
}

// Find returns a compatible release by exact canonical version. A leading v is
// accepted, but aliases, partial versions, malformed input, and invalid
// manifests are rejected.
func (m Manifest) Find(version string) (Version, bool) {
	if err := m.Validate(); err != nil {
		return Version{}, false
	}
	canonical, err := NormalizeVersion(version)
	if err != nil {
		return Version{}, false
	}
	for _, entry := range m.Versions {
		if entry.Version == canonical {
			return entry, true
		}
	}
	return Version{}, false
}

// Public returns a detached, API-safe snapshot. Invalid manifests are rejected.
func (m Manifest) Public() (PublicManifest, error) {
	recommended, err := m.Recommended()
	if err != nil {
		return PublicManifest{}, err
	}
	view := PublicManifest{
		Schema:             m.Schema,
		Platform:           m.Platform,
		RecommendedVersion: recommended.Version,
		Versions:           make([]PublicVersion, len(m.Versions)),
	}
	for index, entry := range m.Versions {
		view.Versions[index] = PublicVersion{
			Version:    entry.Version,
			Tag:        entry.Tag,
			Status:     entry.Status,
			AssetName:  entry.AssetName,
			Size:       entry.Size,
			SHA256:     entry.SHA256,
			ReleaseURL: entry.ReleaseURL,
		}
	}
	return view, nil
}
