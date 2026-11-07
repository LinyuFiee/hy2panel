# hy2panel

一个面向单 VPS 的轻量节点面板。使用 Go + Vue，将 VLESS Reality、Hysteria2、RackNerd 套餐流量、实时系统指标、BBR 和 LibreSpeed 测速放在一个小清新控制台中。

![hy2panel 概览](docs/images/dashboard.png)

## 功能

- VLESS Reality Vision 与 Hysteria2 配置、客户端链接和本地二维码
- 实时上下行速率、曲线、累计流量、CPU、内存、磁盘和运行时间
- RackNerd 官方 API 月度流量展示，成功结果缓存 5 分钟
- BBR + fq 状态展示和固定模板一键应用
- LibreSpeed 按需测速，展示延迟、抖动、上下行和测试耗流
- Hairline 指针互动图形、浅色响应式界面
- 节点配置预检、原子应用和启动失败回滚
- HTTPS、管理 IP 白名单、登录限流、HttpOnly/SameSite Strict 会话
- 面板与 sing-box 非 root 运行，仅固定系统操作通过受控 root 单元执行

## 环境要求

- Debian 12，x86_64
- 建议至少 1 核 CPU、1 GB 内存
- 面板和 Hysteria2 两个域名已解析到 VPS
- ACME 验证期间可从公网访问 TCP 80
- 放行 TCP 443、UDP 443；TCP 8443 由安装器限制为管理 IP
- 使用 root 执行安装

## 交互式一键安装

直接执行一行命令：

```bash
curl -fsSL https://raw.githubusercontent.com/LinyuFiee/hy2panel/main/scripts/bootstrap.sh | bash
```

脚本会通过 `/dev/tty` 依次询问：

1. 面板域名
2. Hysteria2 证书域名
3. Reality 公网地址
4. 允许访问面板的公网 IP 或 CIDR
5. ACME 邮箱，可留空
6. RackNerd API Key/Hash，可留空且不回显，稍后可在面板设置

确认摘要后自动下载对应 Release、校验 SHA-256、申请证书、配置 BBR 并启动服务。安装完成后，凭据保存在：

```text
/root/hy2panel-credentials
```

面板地址为 `https://面板域名:8443`。

## 无人值守安装

所有必填值均可通过环境变量提供。设置 `HY2PANEL_NONINTERACTIVE=1` 后不会读取终端：

```bash
export HY2PANEL_NONINTERACTIVE=1
export PANEL_DOMAIN="panel.example.com"
export HY2_DOMAIN="hy2.example.com"
export PUBLIC_HOST="203.0.113.10"
export ADMIN_IP="198.51.100.20"
export ACME_EMAIL="admin@example.com"
export RACKNERD_API_KEY=""
export RACKNERD_API_HASH=""
curl -fsSL https://raw.githubusercontent.com/LinyuFiee/hy2panel/main/scripts/bootstrap.sh | bash
```

默认安装最新 Release。固定版本：

```bash
export HY2PANEL_VERSION="v0.2.0"
```

## RackNerd 流量配置

在 RackNerd 对应 VPS 的控制页面启用 API，获取该 VPS 独立的 API Key 和 API Hash，然后在「系统设置 → RackNerd 官方流量」填写。

面板只调用：

```text
action=info
bw=1
status=1
rdtype=json
```

`bw` 按 `total_bytes,used_bytes,free_bytes,percent_used` 解析。不会调用 boot、reboot、shutdown 等电源操作。凭据仅保存在 `/etc/hy2panel/panel.json`，该文件权限为 600；状态接口不会返回 Key 或 Hash。

API 请求失败时，错误信息仅显示在管理面板。成功结果缓存 5 分钟，失败结果缓存 1 分钟，避免频繁请求官方接口。

## BBR 与测速

「系统设置」中的 BBR 按钮只应用项目内置的 `/etc/hy2panel/99-hy2panel.conf` 模板，不接收任意 sysctl 参数。BBR 主要优化 Reality 的 TCP 连接；Hysteria2 使用 QUIC 自有拥塞控制。

测速使用 LibreSpeed CLI，默认 3 并发、下载和上传各 8 秒、禁用 ICMP、使用 HTTPS。只有点击「开始测速」才会运行，不会后台定时测速。测速会短暂占用网络并消耗套餐流量。

## 安全说明

- 域名不等于隐藏节点 IP；Reality 和 Hysteria2 客户端最终都会连接节点地址。
- 面板使用独立 TCP 8443，不占用 Reality 的 TCP 443。
- nftables 与后端同时检查管理 IP；不要随意将白名单改为 `0.0.0.0/0`。
- 客户端链接和二维码包含完整连接凭据，请勿公开分享。
- 初始密码只显示在 root 凭据文件中，不要提交到 issue 或聊天截图。
- 安装器检测到已有 `/etc/hy2panel/panel.json` 时会停止，避免覆盖现有密钥。

## 本地开发

```bash
cd web
npm ci
npm run build
cd ..
go test ./...
go vet ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o dist/hy2panel-linux-amd64 ./cmd/hy2panel
```

UI 视觉冒烟测试：

```bash
cd web
npx playwright install chromium
npm run test:ui
```

模拟 API 仅在 `MOCK_UI=1` 的 Vite 开发服务器中启用，不进入生产后端。

## 发布

推送 `v*` 标签后，GitHub Actions 会自动构建前端、执行 Go 测试与脚本检查、生成 Linux 安装包、计算 SHA-256 并创建 Release。

## 第三方项目

- [Hairline](https://github.com/lucasmarkes/hairline)：MIT，指针互动线条图形
- [LibreSpeed CLI](https://github.com/librespeed/speedtest-cli)：LGPL-3.0，按需服务器测速
- [Lucide](https://github.com/lucide-icons/lucide)：ISC，界面图标

详细许可见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。