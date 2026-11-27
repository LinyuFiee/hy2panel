package config

import (
	"encoding/json"
	"errors"
)

// SingBoxClientProfile builds a mobile TUN profile. Its node credentials are
// for the client; never marshal the panel or server config into this document.
func (c *Config) SingBoxClientProfile() ([]byte, error) {
	if c == nil {
		return nil, errors.New("配置不能为空")
	}
	if !c.Reality.Enabled && !c.Hysteria2.Enabled {
		return nil, errors.New("没有已启用的节点，无法生成客户端配置")
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}

	type object = map[string]any
	var nodes []object
	var tags []string
	if r := c.Reality; r.Enabled {
		// The 1.14 Reality client decodes short_id into an 8-byte buffer.
		if len(r.ShortID) > 16 {
			return nil, errors.New("Reality 客户端 Short ID 不能超过 8 字节")
		}
		tags = append(tags, "reality")
		nodes = append(nodes, object{
			"type": "vless", "tag": "reality",
			"server": c.Panel.PublicHost, "server_port": r.Port,
			"uuid": r.UUID, "flow": "xtls-rprx-vision",
			"tls": object{
				"enabled": true, "server_name": r.ServerName,
				"utls":    object{"enabled": true, "fingerprint": "chrome"},
				"reality": object{"enabled": true, "public_key": r.PublicKey, "short_id": r.ShortID},
			},
		})
	}
	if h := c.Hysteria2; h.Enabled {
		// Server send/receive limits do not describe this device's uplink or
		// downlink. Leave client rates and BBR tuning at the client defaults.
		node := object{
			"type": "hysteria2", "tag": "hysteria2",
			"server": h.Domain, "server_port": h.Port, "password": h.Password,
			"tls": object{"enabled": true, "server_name": h.Domain, "alpn": []string{"h3"}},
		}
		if h.ObfsType != "" {
			node["obfs"] = object{"type": h.ObfsType, "password": h.ObfsPassword}
		}
		tags = append(tags, "hysteria2")
		nodes = append(nodes, node)
	}
	outbounds := []object{
		{"type": "selector", "tag": "proxy", "outbounds": append([]string{"auto"}, tags...), "default": "auto"},
		{"type": "urltest", "tag": "auto", "outbounds": tags, "url": "https://www.gstatic.com/generate_204", "interval": "3m", "tolerance": 50},
	}
	return json.MarshalIndent(object{
		"inbounds": []object{{
			"type": "tun", "tag": "tun-in", "auto_route": true, "dns_mode": "hijack",
			"address": []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"},
		}},
		"outbounds": append(outbounds, nodes...),
		"dns": object{
			"servers": []object{
				{"type": "local", "tag": "bootstrap"},
				{"type": "https", "tag": "remote", "server": "1.1.1.1", "detour": "proxy"},
			},
			"final": "remote",
		},
		"route": object{
			// Resolve proxy endpoints before dialing them, never via the DoH
			// transport that itself needs a working proxy connection.
			"default_domain_resolver": "bootstrap", "auto_detect_interface": true,
			"rules": []object{{"inbound": "tun-in", "port": 53, "action": "hijack-dns"}},
			"final": "proxy",
		},
	}, "", "  ")
}
