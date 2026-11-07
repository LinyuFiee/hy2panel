package server

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"hy2panel/internal/config"
)

type DiagnosticCheck struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

type DiagnosticsResponse struct {
	CheckedAt time.Time         `json:"checked_at"`
	Checks    []DiagnosticCheck `json:"checks"`
	Pass      int               `json:"pass"`
	Warn      int               `json:"warn"`
	Fail      int               `json:"fail"`
}

func (s *Server) diagnostics(w http.ResponseWriter, r *http.Request) {
	if !validMutation(r) {
		writeError(w, http.StatusForbidden, "请求来源无效")
		return
	}
	cfg, err := s.store.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取配置失败")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	response := runDiagnostics(ctx, cfg)
	writeJSON(w, http.StatusOK, response)
}

func runDiagnostics(ctx context.Context, cfg *config.Config) DiagnosticsResponse {
	response := DiagnosticsResponse{CheckedAt: time.Now().UTC(), Checks: make([]DiagnosticCheck, 0, 9)}
	add := func(check DiagnosticCheck) {
		response.Checks = append(response.Checks, check)
		switch check.Status {
		case "pass":
			response.Pass++
		case "warn":
			response.Warn++
		case "fail":
			response.Fail++
		}
	}
	if err := checkSingBoxConfig(ctx); err != nil {
		add(DiagnosticCheck{ID: "config", Name: "sing-box 配置", Status: "fail", Message: redactSecrets(err.Error(), cfg), Suggestion: "在节点管理中检查参数，恢复上一版有效配置。"})
	} else {
		add(DiagnosticCheck{ID: "config", Name: "sing-box 配置", Status: "pass", Message: "配置语法与协议字段校验通过"})
	}
	if err := checkSingBoxService(ctx); err != nil {
		add(DiagnosticCheck{ID: "service", Name: "核心服务", Status: "fail", Message: err.Error(), Suggestion: "查看 sing-box 日志；看门狗会自动尝试重启。"})
	} else {
		add(DiagnosticCheck{ID: "service", Name: "核心服务", Status: "pass", Message: "sing-box.service 正在运行"})
	}
	if cfg.Reality.Enabled {
		if err := checkListener(ctx, "tcp", cfg.Reality.Port); err != nil {
			add(DiagnosticCheck{ID: "reality-listen", Name: "Reality 监听", Status: "fail", Message: err.Error(), Suggestion: "检查端口占用、防火墙和服务日志。"})
		} else {
			add(DiagnosticCheck{ID: "reality-listen", Name: "Reality 监听", Status: "pass", Message: fmt.Sprintf("TCP %d 正在监听", cfg.Reality.Port)})
		}
	}
	if cfg.Hysteria2.Enabled {
		if err := checkListener(ctx, "udp", cfg.Hysteria2.Port); err != nil {
			add(DiagnosticCheck{ID: "hy2-listen", Name: "Hysteria2 监听", Status: "fail", Message: err.Error(), Suggestion: "检查 UDP 端口、证书和服务日志。"})
		} else {
			add(DiagnosticCheck{ID: "hy2-listen", Name: "Hysteria2 监听", Status: "pass", Message: fmt.Sprintf("UDP %d 正在监听", cfg.Hysteria2.Port)})
		}
		if err := checkCertificate(cfg.Hysteria2.CertFile, time.Now()); err != nil {
			add(DiagnosticCheck{ID: "certificate", Name: "节点证书", Status: "warn", Message: err.Error(), Suggestion: "检查 certbot.timer 和 TCP 80 的续期条件。"})
		} else {
			add(DiagnosticCheck{ID: "certificate", Name: "节点证书", Status: "pass", Message: "Hysteria2 证书有效"})
		}
		if err := checkDNS(ctx, cfg.Hysteria2.Domain); err != nil {
			add(DiagnosticCheck{ID: "dns", Name: "域名解析", Status: "warn", Message: err.Error(), Suggestion: "检查域名 A/AAAA 记录和本机 DNS。"})
		} else {
			add(DiagnosticCheck{ID: "dns", Name: "域名解析", Status: "pass", Message: "Hysteria2 域名解析正常"})
		}
	}
	if cfg.Reality.Enabled {
		if err := checkRealityTarget(ctx, cfg.Reality.ServerName); err != nil {
			add(DiagnosticCheck{ID: "reality-target", Name: "Reality 握手目标", Status: "warn", Message: err.Error(), Suggestion: "更换可访问且支持 TLS 1.3 的目标网站。"})
		} else {
			add(DiagnosticCheck{ID: "reality-target", Name: "Reality 握手目标", Status: "pass", Message: "可以连接 Reality 握手目标"})
		}
	}
	if checkBBR() {
		add(DiagnosticCheck{ID: "bbr", Name: "BBR 网络优化", Status: "pass", Message: "BBR + fq 已启用"})
	} else {
		add(DiagnosticCheck{ID: "bbr", Name: "BBR 网络优化", Status: "warn", Message: "BBR + fq 未完整启用", Suggestion: "在系统设置中重新应用固定模板。"})
	}
	if status := resourceStatus(); status != "" {
		add(DiagnosticCheck{ID: "resources", Name: "系统资源", Status: "warn", Message: status, Suggestion: "释放内存或磁盘空间，避免 OOM 和证书写入失败。"})
	} else {
		add(DiagnosticCheck{ID: "resources", Name: "系统资源", Status: "pass", Message: "内存和磁盘余量正常"})
	}
	add(DiagnosticCheck{ID: "external", Name: "外部连通性", Status: "info", Message: "本机无法判断运营商或 GFW 是否封堵端口", Suggestion: "使用客户端或外部 TCP/UDP 探测复核，不要仅凭本机结果频繁换端口。"})
	return response
}

func checkSingBoxConfig(ctx context.Context) error {
	output, err := exec.CommandContext(ctx, "/usr/local/bin/sing-box", "check", "-c", "/etc/hy2panel/sing-box.json").CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("配置校验失败: %s", message)
	}
	return nil
}

func checkSingBoxService(ctx context.Context) error {
	output, err := exec.CommandContext(ctx, "systemctl", "is-active", "sing-box.service").CombinedOutput()
	if err != nil {
		return fmt.Errorf("sing-box 未处于 active 状态: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func checkListener(ctx context.Context, network string, port int) error {
	command := "-lnt"
	if network == "udp" {
		command = "-lnu"
	}
	output, err := exec.CommandContext(ctx, "ss", "-H", command).CombinedOutput()
	if err != nil {
		return fmt.Errorf("无法读取 %s 监听端口", strings.ToUpper(network))
	}
	if !listenerPorts(output)[port] {
		return fmt.Errorf("%s %d 未监听", strings.ToUpper(network), port)
	}
	return nil
}

func listenerPorts(output []byte) map[int]bool {
	ports := make(map[int]bool)
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		address := fields[3]
		index := strings.LastIndex(address, ":")
		if index < 0 {
			continue
		}
		port, err := strconv.Atoi(address[index+1:])
		if err == nil {
			ports[port] = true
		}
	}
	return ports
}

func checkCertificate(path string, now time.Time) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("无法读取证书: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return fmt.Errorf("证书 PEM 格式无效")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("证书解析失败: %w", err)
	}
	if now.Before(certificate.NotBefore) {
		return fmt.Errorf("证书尚未生效")
	}
	if now.After(certificate.NotAfter) {
		return fmt.Errorf("证书已于 %s 过期", certificate.NotAfter.Format("2006-01-02"))
	}
	days := int(certificate.NotAfter.Sub(now).Hours() / 24)
	if days < 14 {
		return fmt.Errorf("证书将在 %d 天后到期", days)
	}
	return nil
}

func checkDNS(ctx context.Context, domain string) error {
	if _, err := net.DefaultResolver.LookupIPAddr(ctx, domain); err != nil {
		return fmt.Errorf("域名 %s 解析失败: %w", domain, err)
	}
	return nil
}

func checkRealityTarget(ctx context.Context, serverName string) error {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(serverName, "443"))
	if err != nil {
		return fmt.Errorf("无法连接 %s:443", serverName)
	}
	_ = connection.Close()
	return nil
}

func checkBBR() bool {
	congestion, err1 := os.ReadFile("/proc/sys/net/ipv4/tcp_congestion_control")
	qdisc, err2 := os.ReadFile("/proc/sys/net/core/default_qdisc")
	return err1 == nil && err2 == nil && strings.TrimSpace(string(congestion)) == "bbr" && strings.TrimSpace(string(qdisc)) == "fq"
}

func resourceStatus() string {
	status := Status{}
	readMemory(&status)
	if status.MemoryTotalMB > 0 && float64(status.MemoryUsedMB)/float64(status.MemoryTotalMB) > 0.92 {
		return "内存使用率超过 92%"
	}
	used, total := readDiskUsage()
	if total > 0 && float64(used)/float64(total) > 0.95 {
		return "磁盘使用率超过 95%"
	}
	return ""
}
