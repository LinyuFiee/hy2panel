package singbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultGitHubAPIBaseURL = "https://api.github.com/repos/SagerNet/sing-box"
	defaultGitHubTimeout    = 10 * time.Second
	maxGitHubResponseBytes  = 2 << 20
)

// GitHubClient resolves a manifest entry against its exact upstream GitHub
// release. BaseURL exists for tests and trusted infrastructure configuration;
// it is never accepted from an end user. Returned asset URLs are independently
// constrained to official HTTPS GitHub download hosts.
type GitHubClient struct {
	HTTPClient *http.Client
	BaseURL    string
}

// ResolvedAsset is an upstream asset proven to match a validated manifest
// entry. DownloadURL is safe to hand to a downloader that still verifies size
// and SHA-256 after download.
type ResolvedAsset struct {
	Version     string
	Tag         string
	Name        string
	Size        int64
	SHA256      string
	DownloadURL string
}

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string  `json:"name"`
	Size               int64   `json:"size"`
	Digest             *string `json:"digest"`
	BrowserDownloadURL string  `json:"browser_download_url"`
}

// Resolve fetches the exact release-by-tag API endpoint and returns its one
// matching asset only when all manifest metadata agrees.
func (c GitHubClient) Resolve(ctx context.Context, expected Version) (ResolvedAsset, error) {
	if ctx == nil {
		return ResolvedAsset{}, errors.New("GitHub request context is nil")
	}
	if err := validateManifestVersion(expected); err != nil {
		return ResolvedAsset{}, fmt.Errorf("invalid expected sing-box release: %w", err)
	}

	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = DefaultGitHubAPIBaseURL
	}
	endpoint, err := releaseEndpoint(baseURL, expected.Tag)
	if err != nil {
		return ResolvedAsset{}, err
	}

	requestContext, cancel := context.WithTimeout(ctx, defaultGitHubTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, endpoint, nil)
	if err != nil {
		return ResolvedAsset{}, fmt.Errorf("create GitHub release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "hy2panel-sing-box-resolver")

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultGitHubTimeout}
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return ResolvedAsset{}, fmt.Errorf("fetch GitHub release %q: %w", expected.Tag, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ResolvedAsset{}, fmt.Errorf("GitHub release %q returned HTTP %d", expected.Tag, response.StatusCode)
	}

	var release githubRelease
	if err := decodeLimitedJSON(response.Body, maxGitHubResponseBytes, &release); err != nil {
		return ResolvedAsset{}, fmt.Errorf("decode GitHub release %q: %w", expected.Tag, err)
	}
	if release.TagName != expected.Tag {
		return ResolvedAsset{}, fmt.Errorf("GitHub release tag %q does not match expected %q", release.TagName, expected.Tag)
	}
	if release.Draft {
		return ResolvedAsset{}, fmt.Errorf("GitHub release %q is a draft", expected.Tag)
	}
	if release.Prerelease {
		return ResolvedAsset{}, fmt.Errorf("GitHub release %q is a prerelease", expected.Tag)
	}

	var match *githubAsset
	for index := range release.Assets {
		if release.Assets[index].Name != expected.AssetName {
			continue
		}
		if match != nil {
			return ResolvedAsset{}, fmt.Errorf("GitHub release %q contains duplicate asset %q", expected.Tag, expected.AssetName)
		}
		match = &release.Assets[index]
	}
	if match == nil {
		return ResolvedAsset{}, fmt.Errorf("GitHub release %q does not contain asset %q", expected.Tag, expected.AssetName)
	}
	if match.Size != expected.Size {
		return ResolvedAsset{}, fmt.Errorf("GitHub asset %q size %d does not match expected %d", expected.AssetName, match.Size, expected.Size)
	}
	if match.Digest != nil && *match.Digest != "" && *match.Digest != "sha256:"+expected.SHA256 {
		return ResolvedAsset{}, fmt.Errorf("GitHub asset %q digest does not match expected SHA-256", expected.AssetName)
	}
	if err := ValidateDownloadURL(match.BrowserDownloadURL, expected); err != nil {
		return ResolvedAsset{}, err
	}

	return ResolvedAsset{
		Version:     expected.Version,
		Tag:         expected.Tag,
		Name:        expected.AssetName,
		Size:        expected.Size,
		SHA256:      expected.SHA256,
		DownloadURL: match.BrowserDownloadURL,
	}, nil
}

func releaseEndpoint(baseURL, tag string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid GitHub API base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("GitHub API base URL must use HTTP or HTTPS")
	}
	if parsed.Scheme == "https" && parsed.Host != strings.ToLower(parsed.Host) {
		return "", errors.New("GitHub API base URL host must be canonical lowercase")
	}
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.RawPath != "" || parsed.ForceQuery {
		return "", errors.New("GitHub API base URL is malformed")
	}
	if len(tag) < 2 || tag[0] != 'v' || tag[1] == 'v' {
		return "", errors.New("GitHub release tag is malformed")
	}
	if _, err := NormalizeVersion(tag[1:]); err != nil {
		return "", errors.New("GitHub release tag is malformed")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/releases/tags/" + tag
	return parsed.String(), nil
}

func decodeLimitedJSON(reader io.Reader, limit int64, destination any) error {
	limited := &io.LimitedReader{R: reader, N: limit + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if int64(len(data)) > limit {
		return fmt.Errorf("response exceeds %d bytes", limit)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("response contains multiple JSON values")
		}
		return err
	}
	return nil
}

// ValidateDownloadURL rejects user-controlled or lookalike URLs. The normal
// GitHub release URL must exactly encode the expected repository, tag, and
// asset name. objects.githubusercontent.com is accepted only for the signed
// redirect form returned by GitHub's own API; callers must never feed this
// function a URL supplied by an API user as a substitute for Resolve.
func ValidateDownloadURL(rawURL string, expected Version) error {
	if err := validateManifestVersion(expected); err != nil {
		return fmt.Errorf("invalid expected sing-box release: %w", err)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid GitHub asset download URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.User != nil || parsed.Port() != "" || parsed.Fragment != "" || parsed.RawPath != "" || parsed.ForceQuery {
		return errors.New("GitHub asset download URL must be an uncredentialed HTTPS URL on the default port")
	}

	host := parsed.Host
	switch host {
	case "github.com":
		expectedPath := "/SagerNet/sing-box/releases/download/" + expected.Tag + "/" + expected.AssetName
		if parsed.Path != expectedPath || parsed.RawQuery != "" {
			return fmt.Errorf("GitHub asset download URL does not match expected release asset %q", expected.AssetName)
		}
	case "objects.githubusercontent.com":
		query, err := url.ParseQuery(parsed.RawQuery)
		if err != nil || len(query) == 0 || !strings.HasSuffix(parsed.Path, "/"+expected.AssetName) {
			return fmt.Errorf("GitHub object URL does not identify expected asset %q", expected.AssetName)
		}
	default:
		return fmt.Errorf("untrusted GitHub asset download host %q", parsed.Hostname())
	}
	return nil
}
