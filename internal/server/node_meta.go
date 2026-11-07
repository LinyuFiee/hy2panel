package server

import (
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

type NodeMeta struct {
	CountryCode   string `json:"country_code"`
	CountryName   string `json:"country_name"`
	Flag          string `json:"flag"`
	RealityName   string `json:"reality_name"`
	Hysteria2Name string `json:"hysteria2_name"`
	Error         string `json:"error,omitempty"`
}

type geoIPLookup interface {
	Lookup(context.Context, string) (NodeMeta, error)
}

type geoIPClient struct {
	Endpoint   string
	HTTPClient *http.Client
}

func defaultNodeMeta() NodeMeta {
	return NodeMeta{Flag: "🌐", RealityName: "🌐 Reality", Hysteria2Name: "🌐 Hysteria2"}
}

func (c *geoIPClient) Lookup(ctx context.Context, host string) (NodeMeta, error) {
	result := defaultNodeMeta()
	address, err := publicHostIP(ctx, host)
	if err != nil {
		return result, err
	}
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = "https://ipwho.is"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(endpoint, "/")+"/"+address.String(), nil)
	if err != nil {
		return result, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "hy2panel")
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 6 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return result, fmt.Errorf("地理位置服务返回 HTTP %d", response.StatusCode)
	}
	var payload struct {
		Success     bool   `json:"success"`
		Message     string `json:"message"`
		Country     string `json:"country"`
		CountryCode string `json:"country_code"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 128<<10)).Decode(&payload); err != nil {
		return result, errors.New("地理位置服务响应不是有效 JSON")
	}
	if !payload.Success {
		return result, fmt.Errorf("地理位置查询失败: %s", payload.Message)
	}
	code := strings.ToUpper(strings.TrimSpace(payload.CountryCode))
	flag := countryFlag(code)
	if flag == "" {
		return result, errors.New("地理位置服务未返回有效国家代码")
	}
	result.CountryCode = code
	result.CountryName = payload.Country
	result.Flag = flag
	result.RealityName = flag + " " + code + "-Reality"
	result.Hysteria2Name = flag + " " + code + "-Hysteria2"
	return result, nil
}

func (s *Server) nodeMeta(ctx context.Context, host string) NodeMeta {
	s.nodeMetaMu.Lock()
	defer s.nodeMetaMu.Unlock()
	if host == s.nodeMetaHost && !s.nodeMetaCachedAt.IsZero() && time.Since(s.nodeMetaCachedAt) < s.nodeMetaTTL {
		return s.nodeMetaCache
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	meta, err := s.nodeMetaClient.Lookup(ctx, host)
	if err != nil {
		meta = defaultNodeMeta()
		meta.Error = err.Error()
		s.nodeMetaTTL = 10 * time.Minute
	} else {
		s.nodeMetaTTL = 24 * time.Hour
	}
	s.nodeMetaCache = meta
	s.nodeMetaHost = host
	s.nodeMetaCachedAt = time.Now()
	return meta
}

func namedClientLinks(cfgLinks map[string]string, meta NodeMeta) map[string]string {
	links := make(map[string]string, len(cfgLinks))
	for name, value := range cfgLinks {
		parsed, err := url.Parse(value)
		if err != nil {
			links[name] = value
			continue
		}
		if name == "reality" {
			parsed.Fragment = meta.RealityName
		} else if name == "hysteria2" {
			parsed.Fragment = meta.Hysteria2Name
		}
		links[name] = parsed.String()
	}
	return links
}

func countryFlag(code string) string {
	if len(code) != 2 {
		return ""
	}
	runes := []rune(code)
	if runes[0] < 'A' || runes[0] > 'Z' || runes[1] < 'A' || runes[1] > 'Z' {
		return ""
	}
	return string([]rune{rune(0x1F1E6 + runes[0] - 'A'), rune(0x1F1E6 + runes[1] - 'A')})
}

func publicHostIP(ctx context.Context, host string) (netip.Addr, error) {
	if address, err := netip.ParseAddr(host); err == nil {
		if err := validatePublicAddress(address); err != nil {
			return netip.Addr{}, err
		}
		return address, nil
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return netip.Addr{}, errors.New("节点地址无法解析")
	}
	for _, item := range addresses {
		address, ok := netip.ParseAddr(item.IP.String())
		if ok == nil && validatePublicAddress(address) == nil {
			return address, nil
		}
	}
	return netip.Addr{}, errors.New("节点地址没有可查询的公网 IP")
}

func validatePublicAddress(address netip.Addr) error {
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsMulticast() {
		return errors.New("非公网地址不执行地理位置查询")
	}
	for _, prefixText := range []string{"192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24"} {
		prefix := netip.MustParsePrefix(prefixText)
		if prefix.Contains(address) {
			return errors.New("文档保留地址不执行地理位置查询")
		}
	}
	return nil
}
