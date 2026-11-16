package traffic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const (
	defaultClashTimeout = 2 * time.Second
	maxClashResponse    = 4 << 20
)

// ClashClient safely reads sing-box's loopback-only Clash /connections API.
type ClashClient struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
	source     SourceIdentity
}

// NewClashClient builds a client whose transport never consults proxy
// environment variables and whose redirects are always rejected.
func NewClashClient(baseURL, token string) (*ClashClient, error) {
	return NewClashClientWithSource(baseURL, token, SourceIdentity{})
}

// NewClashClientWithSource is NewClashClient plus caller-provided source
// identity used to detect a process or invocation change.
func NewClashClientWithSource(baseURL, token string, source SourceIdentity) (*ClashClient, error) {
	endpoint, err := validateLoopbackBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if !validSourceIdentity(source) {
		return nil, errors.New("Clash API source identity is too long")
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("Clash API bearer token is required")
	}
	if len(token) > 4096 {
		return nil, errors.New("Clash API bearer token is too long")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = (&net.Dialer{Timeout: defaultClashTimeout}).DialContext
	return &ClashClient{
		baseURL: endpoint,
		token:   token,
		source:  source,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   defaultClashTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

// Fetch implements Source.
func (c *ClashClient) Fetch(ctx context.Context) (ClashSnapshot, error) {
	if c == nil || c.baseURL == nil || c.httpClient == nil {
		return ClashSnapshot{}, errors.New("Clash API client is not initialized")
	}
	if ctx == nil {
		return ClashSnapshot{}, errors.New("Clash API context is nil")
	}
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/connections"
	endpoint.RawPath = ""
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return ClashSnapshot{}, err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return ClashSnapshot{}, fmt.Errorf("read Clash connections: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ClashSnapshot{}, fmt.Errorf("Clash connections returned HTTP %d", response.StatusCode)
	}

	limited := io.LimitReader(response.Body, maxClashResponse+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return ClashSnapshot{}, fmt.Errorf("read Clash connections response: %w", err)
	}
	if len(data) > maxClashResponse {
		return ClashSnapshot{}, errors.New("Clash connections response is too large")
	}
	snapshot, err := decodeClashSnapshot(data)
	if err != nil {
		return ClashSnapshot{}, err
	}
	snapshot.Source = c.source
	return snapshot, nil
}

func validateLoopbackBaseURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("invalid Clash API base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("Clash API base URL must use http or https")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("Clash API base URL must not contain credentials, query, or fragment")
	}
	if parsed.Host == "" {
		return nil, errors.New("Clash API base URL must include a host")
	}
	host := parsed.Hostname()
	address, err := netip.ParseAddr(host)
	if err != nil || !address.Unmap().IsLoopback() {
		return nil, errors.New("Clash API base URL host must be a loopback IP address")
	}
	port := parsed.Port()
	if parsed.Host[len(parsed.Host)-1] == ':' || strings.HasSuffix(parsed.Host, "]:") {
		return nil, errors.New("Clash API base URL has an invalid port")
	}
	if port != "" {
		value, err := parseJSONUint(port)
		if err != nil || value == 0 || value > 65535 {
			return nil, errors.New("Clash API base URL has an invalid port")
		}
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed, nil
}

type clashWireSnapshot struct {
	DownloadTotal json.RawMessage       `json:"downloadTotal"`
	UploadTotal   json.RawMessage       `json:"uploadTotal"`
	Connections   []clashWireConnection `json:"connections"`
}

type clashWireConnection struct {
	ID       json.RawMessage `json:"id"`
	Metadata struct {
		Type json.RawMessage `json:"type"`
	} `json:"metadata"`
	Upload   json.RawMessage `json:"upload"`
	Download json.RawMessage `json:"download"`
	Start    json.RawMessage `json:"start"`
}

func decodeClashSnapshot(data []byte) (ClashSnapshot, error) {
	var wire clashWireSnapshot
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&wire); err != nil {
		return ClashSnapshot{}, errors.New("Clash connections response is not valid JSON")
	}
	if err := requireEOF(decoder); err != nil {
		return ClashSnapshot{}, err
	}
	uploadTotal, err := decodeUint64(wire.UploadTotal, "uploadTotal")
	if err != nil {
		return ClashSnapshot{}, err
	}
	downloadTotal, err := decodeUint64(wire.DownloadTotal, "downloadTotal")
	if err != nil {
		return ClashSnapshot{}, err
	}
	if wire.Connections == nil {
		return ClashSnapshot{}, errors.New("Clash connections field is missing or null")
	}
	if len(wire.Connections) > MaxOpenFlows {
		return ClashSnapshot{}, errors.New("Clash connections snapshot has too many flows")
	}

	result := ClashSnapshot{
		UploadTotal:   uploadTotal,
		DownloadTotal: downloadTotal,
		Connections:   make([]ClashConnection, 0, len(wire.Connections)),
	}
	seen := make(map[string]struct{}, len(wire.Connections))
	for index, connection := range wire.Connections {
		id, err := decodeRequiredString(connection.ID, "connection id")
		if err != nil {
			return ClashSnapshot{}, fmt.Errorf("connection %d: %w", index, err)
		}
		if len(id) > 256 {
			return ClashSnapshot{}, fmt.Errorf("connection %d: id is too long", index)
		}
		if _, exists := seen[id]; exists {
			return ClashSnapshot{}, fmt.Errorf("connection %d: duplicate id", index)
		}
		seen[id] = struct{}{}
		connectionType, err := decodeRequiredString(connection.Metadata.Type, "metadata.type")
		if err != nil {
			return ClashSnapshot{}, fmt.Errorf("connection %d: %w", index, err)
		}
		if len(connectionType) > 256 {
			return ClashSnapshot{}, fmt.Errorf("connection %d: metadata.type is too long", index)
		}
		upload, err := decodeUint64(connection.Upload, "upload")
		if err != nil {
			return ClashSnapshot{}, fmt.Errorf("connection %d: %w", index, err)
		}
		download, err := decodeUint64(connection.Download, "download")
		if err != nil {
			return ClashSnapshot{}, fmt.Errorf("connection %d: %w", index, err)
		}
		start, err := decodeTime(connection.Start)
		if err != nil {
			return ClashSnapshot{}, fmt.Errorf("connection %d: %w", index, err)
		}
		result.Connections = append(result.Connections, ClashConnection{
			ID:       id,
			Metadata: ClashMetadata{Type: connectionType},
			Upload:   upload,
			Download: download,
			Start:    start,
		})
	}
	return result, nil
}

func requireEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("Clash connections response contains trailing data")
	}
	return nil
}

func decodeUint64(raw json.RawMessage, field string) (uint64, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return 0, fmt.Errorf("Clash connections field %s is missing or null", field)
	}
	if raw[0] < '0' || raw[0] > '9' {
		return 0, fmt.Errorf("Clash connections field %s is not an unsigned integer", field)
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err != nil {
		return 0, fmt.Errorf("Clash connections field %s is not an unsigned integer", field)
	}
	value, err := parseJSONUint(number.String())
	if err != nil {
		return 0, fmt.Errorf("Clash connections field %s is not an unsigned integer", field)
	}
	return value, nil
}

func parseJSONUint(value string) (uint64, error) {
	if value == "" || strings.HasPrefix(value, "-") || strings.ContainsAny(value, ".eE+") {
		return 0, errors.New("not an unsigned integer")
	}
	var result uint64
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, errors.New("not an unsigned integer")
		}
		digit := uint64(character - '0')
		if result > (^uint64(0)-digit)/10 {
			return 0, errors.New("unsigned integer overflow")
		}
		result = result*10 + digit
	}
	return result, nil
}

func decodeRequiredString(raw json.RawMessage, field string) (string, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return "", fmt.Errorf("%s is missing or null", field)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || value == "" {
		return "", fmt.Errorf("%s is not a non-empty string", field)
	}
	return value, nil
}

func decodeTime(raw json.RawMessage) (time.Time, error) {
	value, err := decodeRequiredString(raw, "start")
	if err != nil {
		return time.Time{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, errors.New("start is not a valid RFC3339 timestamp")
	}
	return parsed.UTC(), nil
}
