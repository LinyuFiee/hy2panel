package config

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/curve25519"
)

var hostLabel = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)

const (
	SchemaVersion          = 1
	defaultTelemetryListen = "127.0.0.1:19090"
	adminPasswordMinLength = 12
	adminPasswordMaxLength = 128
)

type Panel struct {
	Listen     string `json:"listen"`
	Domain     string `json:"domain"`
	AdminCIDR  string `json:"admin_cidr,omitempty"`
	PublicHost string `json:"public_host"`
	CertFile   string `json:"cert_file"`
	KeyFile    string `json:"key_file"`
}

type Telemetry struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"`
	Secret  string `json:"secret,omitempty"`
}

type Reality struct {
	Enabled    bool   `json:"enabled"`
	Port       int    `json:"port"`
	ServerName string `json:"server_name"`
	UUID       string `json:"uuid"`
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
	ShortID    string `json:"short_id"`
}

type Hysteria2 struct {
	Enabled  bool   `json:"enabled"`
	Port     int    `json:"port"`
	Domain   string `json:"domain"`
	Password string `json:"password"`
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
	UpMbps   int    `json:"up_mbps"`
	DownMbps int    `json:"down_mbps"`
}

type RackNerd struct {
	APIKey  string `json:"api_key,omitempty"`
	APIHash string `json:"api_hash,omitempty"`
}

func (r RackNerd) Configured() bool {
	return r.APIKey != "" && r.APIHash != ""
}

type Recovery struct {
	AutoPort       bool  `json:"auto_port"`
	LastPortChange int64 `json:"last_port_change"`
}

type Config struct {
	Version           int       `json:"version"`
	AdminPasswordHash string    `json:"admin_password_hash,omitempty"`
	Panel             Panel     `json:"panel"`
	Telemetry         Telemetry `json:"telemetry"`
	Reality           Reality   `json:"reality"`
	Hysteria2         Hysteria2 `json:"hysteria2"`
	RackNerd          RackNerd  `json:"racknerd"`
	Recovery          Recovery  `json:"recovery"`
}

func HashAdminPassword(password string) (string, error) {
	if len(password) < adminPasswordMinLength || len(password) > adminPasswordMaxLength {
		return "", fmt.Errorf("管理员密码长度必须为 %d 到 %d 个字符", adminPasswordMinLength, adminPasswordMaxLength)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func Initial(panelDomain, hy2Domain, publicHost, _ string, password string) (*Config, error) {
	hash, err := HashAdminPassword(password)
	if err != nil {
		return nil, err
	}
	uuid, err := newUUID()
	if err != nil {
		return nil, err
	}
	privateKey, publicKey, err := newRealityKeypair()
	if err != nil {
		return nil, err
	}
	shortID, err := randomHex(8)
	if err != nil {
		return nil, err
	}
	hy2Password, err := randomToken(24)
	if err != nil {
		return nil, err
	}
	telemetrySecret, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	c := &Config{
		Version:           SchemaVersion,
		AdminPasswordHash: hash,
		Panel: Panel{
			Listen:     ":8443",
			Domain:     panelDomain,
			PublicHost: publicHost,
			CertFile:   "/etc/hy2panel/certs/panel.crt",
			KeyFile:    "/etc/hy2panel/certs/panel.key",
		},
		Telemetry: Telemetry{
			Enabled: true,
			Listen:  defaultTelemetryListen,
			Secret:  telemetrySecret,
		},
		Reality: Reality{
			Enabled:    true,
			Port:       443,
			ServerName: "www.microsoft.com",
			UUID:       uuid,
			PrivateKey: privateKey,
			PublicKey:  publicKey,
			ShortID:    shortID,
		},
		Hysteria2: Hysteria2{
			Enabled:  hy2Domain != "",
			Port:     443,
			Domain:   hy2Domain,
			Password: hy2Password,
			CertFile: "/etc/hy2panel/certs/hy2.crt",
			KeyFile:  "/etc/hy2panel/certs/hy2.key",
			UpMbps:   100,
			DownMbps: 100,
		},
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) Public() Config {
	copy := *c
	copy.AdminPasswordHash = ""
	copy.Panel.AdminCIDR = ""
	copy.Telemetry.Secret = ""
	copy.RackNerd.APIKey = ""
	copy.RackNerd.APIHash = ""
	return copy
}

func (c *Config) EnsureTelemetry() (bool, error) {
	if c == nil {
		return false, errors.New("配置不能为空")
	}
	changed := false
	missing := c.Telemetry == (Telemetry{})
	if c.Telemetry.Listen == "" {
		c.Telemetry.Listen = defaultTelemetryListen
		changed = true
	}
	if missing {
		c.Telemetry.Enabled = true
		changed = true
	}
	if c.Telemetry.Secret == "" {
		secret, err := randomToken(32)
		if err != nil {
			return false, err
		}
		c.Telemetry.Secret = secret
		changed = true
	}
	return changed, nil
}

func (c *Config) Migrate() (bool, error) {
	if c == nil {
		return false, errors.New("配置不能为空")
	}
	if c.Version != SchemaVersion {
		return false, errors.New("不支持的配置版本")
	}
	changed, err := c.EnsureTelemetry()
	if err != nil {
		return false, err
	}
	if err := c.Validate(); err != nil {
		return false, err
	}
	return changed, nil
}

func (c *Config) Validate() error {
	if c.Version != SchemaVersion {
		return errors.New("不支持的配置版本")
	}
	if !strings.HasPrefix(c.AdminPasswordHash, "$2") {
		return errors.New("管理员密码哈希无效")
	}
	if err := validateHost(c.Panel.Domain, false); err != nil {
		return fmt.Errorf("面板域名无效: %w", err)
	}
	if err := validateHost(c.Panel.PublicHost, true); err != nil {
		return fmt.Errorf("节点地址无效: %w", err)
	}
	if c.Panel.AdminCIDR != "" {
		if _, err := parsePrefix(c.Panel.AdminCIDR); err != nil {
			return err
		}
	}
	panelHost, panelPort, err := splitListen(c.Panel.Listen)
	if err != nil || (panelHost.IsValid() && !panelHost.IsLoopback()) {
		return errors.New("面板监听地址无效")
	}
	if err := validateCertPaths(c.Panel.CertFile, c.Panel.KeyFile); err != nil {
		return err
	}
	if c.Telemetry != (Telemetry{}) {
		telemetryHost, telemetryPort, err := splitListen(c.Telemetry.Listen)
		if err != nil || !telemetryHost.IsValid() || !telemetryHost.IsLoopback() {
			return errors.New("Telemetry 监听地址必须是 loopback 地址")
		}
		if telemetryPort == panelPort || (c.Reality.Enabled && telemetryPort == c.Reality.Port) || (c.Hysteria2.Enabled && telemetryPort == c.Hysteria2.Port) {
			return errors.New("Telemetry 端口与其他服务冲突")
		}
		if err := validateSecret(c.Telemetry.Secret); err != nil {
			return fmt.Errorf("Telemetry secret 无效: %w", err)
		}
	}
	if c.Reality.Enabled {
		if !validPort(c.Reality.Port) || c.Reality.Port == panelPort {
			return errors.New("Reality 端口无效或与面板冲突")
		}
		if err := validateHost(c.Reality.ServerName, false); err != nil {
			return errors.New("Reality SNI 无效")
		}
		if !validUUID(c.Reality.UUID) {
			return errors.New("Reality UUID 无效")
		}
		if err := validateRealityKey(c.Reality.PrivateKey); err != nil {
			return fmt.Errorf("Reality 私钥无效: %w", err)
		}
		if err := validateRealityKey(c.Reality.PublicKey); err != nil {
			return fmt.Errorf("Reality 公钥无效: %w", err)
		}
		privateKey, _ := base64.RawURLEncoding.DecodeString(c.Reality.PrivateKey)
		expectedPublicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
		actualPublicKey, _ := base64.RawURLEncoding.DecodeString(c.Reality.PublicKey)
		if err != nil || !bytes.Equal(expectedPublicKey, actualPublicKey) {
			return errors.New("Reality 公私钥不匹配")
		}
		shortID, err := hex.DecodeString(c.Reality.ShortID)
		if err != nil || len(shortID) < 1 || len(shortID) > 16 {
			return errors.New("Reality Short ID 必须为 1 到 16 字节的十六进制字符串")
		}
	}
	if c.Hysteria2.Enabled {
		if !validPort(c.Hysteria2.Port) {
			return errors.New("Hysteria2 端口无效")
		}
		if err := validateHost(c.Hysteria2.Domain, false); err != nil {
			return errors.New("Hysteria2 域名无效")
		}
		if len(c.Hysteria2.Password) < 16 || len(c.Hysteria2.Password) > 256 {
			return errors.New("Hysteria2 密码长度必须为 16 到 256 个字符")
		}
		if c.Hysteria2.UpMbps < 1 || c.Hysteria2.UpMbps > 10000 || c.Hysteria2.DownMbps < 1 || c.Hysteria2.DownMbps > 10000 {
			return errors.New("Hysteria2 带宽必须在 1 到 10000 Mbps 之间")
		}
		if err := validateCertPaths(c.Hysteria2.CertFile, c.Hysteria2.KeyFile); err != nil {
			return err
		}
	}
	if (c.RackNerd.APIKey == "") != (c.RackNerd.APIHash == "") {
		return errors.New("RackNerd API Key 和 Hash 必须同时设置")
	}
	for _, credential := range []string{c.RackNerd.APIKey, c.RackNerd.APIHash} {
		if len(credential) > 512 || strings.TrimSpace(credential) != credential || strings.ContainsAny(credential, "\r\n\x00") {
			return errors.New("RackNerd API 凭据格式无效")
		}
	}
	if c.Recovery.LastPortChange < 0 {
		return errors.New("上次端口切换时间无效")
	}
	return nil
}

func (c *Config) SingBoxJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	inbounds := make([]map[string]any, 0, 2)
	if c.Reality.Enabled {
		inbounds = append(inbounds, map[string]any{
			"type":        "vless",
			"tag":         "reality",
			"listen":      "::",
			"listen_port": c.Reality.Port,
			"users": []map[string]any{{
				"uuid": c.Reality.UUID,
				"flow": "xtls-rprx-vision",
			}},
			"tls": map[string]any{
				"enabled":     true,
				"server_name": c.Reality.ServerName,
				"reality": map[string]any{
					"enabled": true,
					"handshake": map[string]any{
						"server":      c.Reality.ServerName,
						"server_port": 443,
					},
					"private_key": c.Reality.PrivateKey,
					"short_id":    []string{c.Reality.ShortID},
				},
			},
		})
	}
	if c.Hysteria2.Enabled {
		inbounds = append(inbounds, map[string]any{
			"type":        "hysteria2",
			"tag":         "hysteria2",
			"listen":      "::",
			"listen_port": c.Hysteria2.Port,
			"users": []map[string]any{{
				"password": c.Hysteria2.Password,
			}},
			"up_mbps":   c.Hysteria2.UpMbps,
			"down_mbps": c.Hysteria2.DownMbps,
			"tls": map[string]any{
				"enabled":          true,
				"certificate_path": c.Hysteria2.CertFile,
				"key_path":         c.Hysteria2.KeyFile,
				"alpn":             []string{"h3"},
			},
		})
	}
	doc := map[string]any{
		"log":       map[string]any{"level": "warn", "timestamp": true},
		"inbounds":  inbounds,
		"outbounds": []map[string]any{{"type": "direct", "tag": "direct"}},
	}
	if c.Telemetry.Enabled {
		doc["experimental"] = map[string]any{
			"clash_api": map[string]any{
				"external_controller": c.Telemetry.Listen,
				"secret":              c.Telemetry.Secret,
			},
		}
	}
	return json.MarshalIndent(doc, "", "  ")
}

func (c *Config) ClientLinks() map[string]string {
	links := make(map[string]string)
	if c.Reality.Enabled {
		q := url.Values{}
		q.Set("encryption", "none")
		q.Set("flow", "xtls-rprx-vision")
		q.Set("security", "reality")
		q.Set("sni", c.Reality.ServerName)
		q.Set("fp", "chrome")
		q.Set("pbk", c.Reality.PublicKey)
		q.Set("sid", c.Reality.ShortID)
		q.Set("type", "tcp")
		u := url.URL{
			Scheme:   "vless",
			User:     url.User(c.Reality.UUID),
			Host:     net.JoinHostPort(c.Panel.PublicHost, strconv.Itoa(c.Reality.Port)),
			RawQuery: q.Encode(),
			Fragment: "Reality",
		}
		links["reality"] = u.String()
	}
	if c.Hysteria2.Enabled {
		q := url.Values{}
		q.Set("sni", c.Hysteria2.Domain)
		q.Set("insecure", "0")
		u := url.URL{
			Scheme:   "hysteria2",
			User:     url.User(c.Hysteria2.Password),
			Host:     net.JoinHostPort(c.Hysteria2.Domain, strconv.Itoa(c.Hysteria2.Port)),
			RawQuery: q.Encode(),
			Fragment: "Hysteria2",
		}
		links["hysteria2"] = u.String()
	}
	return links
}

func parsePrefix(value string) (netip.Prefix, error) {
	value = strings.TrimSpace(value)
	if prefix, err := netip.ParsePrefix(value); err == nil && prefix.IsValid() {
		return prefix.Masked(), nil
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Prefix{}, errors.New("管理 IP 或 CIDR 无效")
	}
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

func splitListen(value string) (netip.Addr, int, error) {
	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		return netip.Addr{}, 0, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil || !validPort(port) {
		return netip.Addr{}, 0, errors.New("监听端口无效")
	}
	if host == "" {
		return netip.Addr{}, port, nil
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, 0, err
	}
	return addr.Unmap(), port, nil
}

func validateSecret(value string) error {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) != 32 || base64.RawURLEncoding.EncodeToString(decoded) != value {
		return errors.New("secret 必须是 32 字节 RawURL Base64")
	}
	return nil
}

func validateHost(value string, allowIP bool) error {
	if value == "" || len(value) > 253 || strings.TrimSpace(value) != value {
		return errors.New("主机名为空或包含非法字符")
	}
	if addr, err := netip.ParseAddr(value); err == nil {
		if allowIP && addr.IsValid() {
			return nil
		}
		return errors.New("此处必须使用域名")
	}
	for _, label := range strings.Split(value, ".") {
		if !hostLabel.MatchString(label) {
			return errors.New("域名格式错误")
		}
	}
	return nil
}

func validateCertPaths(certFile, keyFile string) error {
	for _, filePath := range []string{certFile, keyFile} {
		if !path.IsAbs(filePath) || path.Clean(filePath) != filePath || !strings.HasPrefix(filePath, "/etc/hy2panel/certs/") {
			return errors.New("证书路径必须位于 /etc/hy2panel/certs")
		}
	}
	return nil
}

func validateRealityKey(value string) error {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) != 32 {
		return errors.New("密钥必须是 32 字节 RawURL Base64")
	}
	return nil
}

func validUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	_, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil
}

func validPort(port int) bool {
	return port > 0 && port <= 65535
}

func newUUID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	encoded := hex.EncodeToString(value)
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:]), nil
}

func newRealityKeypair() (string, string, error) {
	privateKey := make([]byte, 32)
	if _, err := rand.Read(privateKey); err != nil {
		return "", "", err
	}
	publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return "", "", err
	}
	return base64.RawURLEncoding.EncodeToString(privateKey), base64.RawURLEncoding.EncodeToString(publicKey), nil
}

func randomHex(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
