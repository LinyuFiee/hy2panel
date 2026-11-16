# hy2panel

一个面向个人、单台 VPS、单节点的轻量管理面板。项目用 Go + Vue 管理 VLESS Reality 与 Hysteria2，并集中展示协议流量、系统状态、RackNerd 套餐流量、BBR 和 LibreSpeed 测速。

它不是多用户、集群或商业计费平台，也不替代主机防火墙、外部监控和备份系统。

![hy2panel 概览](docs/images/dashboard.png)

## 功能

- VLESS Reality Vision 与 Hysteria2 配置、客户端链接和本地二维码
- Reality / Hysteria2 实时速率、累计有效载荷、活动代理流与最多 7 天分钟趋势
- 无法安全归类的代理流量单列为「未归属」
- 主机上下行、CPU、内存、磁盘和运行时间
- RackNerd 官方 API 月度套餐流量，成功结果缓存 5 分钟
- BBR + fq 固定模板与 LibreSpeed 按需测速
- 配置预检、原子应用、服务健康检查和失败回滚
- 固定单元日志、凭据脱敏和逐项服务端自检
- sing-box 版本 allowlist、资产 size/SHA-256 校验、切换与上一版本回滚
- 面板精确目标版本升级、维护互斥、升级快照和失败回滚
- `hy2` 管理命令、配置备份/恢复和保守卸载
- HTTPS、强密码策略、按直接来源 IP 登录限流、HttpOnly/SameSite Strict 会话
- 面板与 sing-box 非 root 运行；固定系统操作由受控 root 单元完成

## 环境要求

- Debian 12，x86_64（当前发布包仅提供 `linux/amd64`）
- 建议至少 1 核 CPU、1 GB 内存
- 面板域名和 Hysteria2 域名均已解析到 VPS
- ACME 验证期间可从公网访问 TCP 80
- 放行 TCP 8443、TCP 443、UDP 443；如修改节点端口，也要同步云防火墙/主机防火墙
- root 权限

面板固定使用公网 TCP 8443，**不限制来源 IP**。安全边界依赖有效 HTTPS、足够强且唯一的管理员密码，以及后端按直接来源 IP 的登录限流；建议再用云防火墙、VPN 或反向代理访问策略收窄入口。项目不会信任 `X-Forwarded-For` 来计算登录限流来源。

## 安装

### 交互式安装

```bash
curl -fsSL https://raw.githubusercontent.com/LinyuFiee/hy2panel/main/scripts/bootstrap.sh | bash
```

脚本通过 `/dev/tty` 依次询问：

1. 面板域名
2. Hysteria2 证书域名
3. Reality 公网地址
4. ACME 邮箱，可留空
5. RackNerd API Key/Hash，可同时留空，稍后在面板设置

v0.5 不再询问 `ADMIN_IP`，也不会创建面板 IP 白名单。确认摘要后，脚本下载 Release、校验 bundle SHA-256、申请证书、应用 BBR 模板并启动服务。面板地址与随机管理员密码保存在仅 root 可读的：

```text
/root/hy2panel-credentials
```

访问地址为 `https://面板域名:8443`。首次登录后建议立即执行 `hy2 passwd` 设置独立强密码。

### 无人值守安装

```bash
export HY2PANEL_NONINTERACTIVE=1
export PANEL_DOMAIN="panel.example.com"
export HY2_DOMAIN="hy2.example.com"
export PUBLIC_HOST="203.0.113.10"
export ACME_EMAIL="admin@example.com"
export RACKNERD_API_KEY=""
export RACKNERD_API_HASH=""
curl -fsSL https://raw.githubusercontent.com/LinyuFiee/hy2panel/main/scripts/bootstrap.sh | bash
```

默认安装最新稳定 Release。固定面板版本时，bootstrap 只接受完整标签：

```bash
export HY2PANEL_VERSION="v0.5.0"
```

可选的 `SING_BOX_VERSION` 必须是 bundle 中 `packaging/sing-box-versions.json` allowlist 的**精确版本**；任意版本、部分版本和未知版本都会被拒绝：

```bash
export SING_BOX_VERSION="1.14.4"
```

allowlist 同时固定官方资产名称、字节数和 SHA-256。未指定时安装清单中唯一标为 `recommended` 的版本。

## 协议流量口径

「协议流量」来自 sing-box 本机 Clash API 的全局计数与活动连接快照，经本地采样后归类：

- Reality：类型为 `vless/reality` 的流
- Hysteria2：类型为 `hysteria2/hysteria2` 的流
- 未归属：未知类型，以及全局增量中无法可靠对应到上述活动流的部分
- 活动代理流：采样时仍在 sing-box 连接列表中的流数量，不是在线用户数

这里的上传/下载沿用 Clash API 方向，累计值是 sing-box 观察到的**代理有效载荷近似值**。它不含或不能精确反映 IP/TCP/UDP/QUIC/TLS 头、重传、握手、探测、面板、SSH、测速和同机其他进程流量；采样中断会标记缺口。因此它不是网卡字节账本，也不是运营商/RackNerd 计费口径，**不可用于结算、配额执法或审计**。

持久化状态只保留协议分类、字节基线、分钟桶和活动流 ID，不保存对端地址、域名、进程、路由规则或完整连接元数据。协议采集不可用时，主机网络指标和节点转发不因此停止。

## sing-box 版本选择与回滚

「系统设置 → sing-box 核心」只列出本次面板包内的兼容清单。切换时服务器会：

1. 从对应 GitHub Release 精确定位清单资产
2. 校验 Release 元数据、字节数和 SHA-256
3. 执行 `sing-box version` 并用当前生产配置运行 `sing-box check`
4. 原子切换版本并重启服务
5. 健康检查失败时尝试恢复上一版本

命令行等价操作：

```bash
hy2 sing-box list
hy2 sing-box set 1.14.4
hy2 sing-box rollback
```

仅保留当前版本和上一版本。回滚只有在确实存在上一版本时可用；它恢复核心二进制，不回滚面板配置。网络中断、GitHub 限流或磁盘不足会使下载/切换失败，失败结果需结合 `hy2 logs sing-box` 与 systemd 日志检查。

## `hy2` 管理命令

直接运行 `hy2` 可打开交互菜单，也可使用子命令：

```text
hy2 status
hy2 info
hy2 logs [panel|sing-box|watchdog|apply|upgrade]
hy2 restart [panel|proxy|all]
hy2 check
hy2 passwd
hy2 backup [文件]
hy2 restore <文件>
hy2 sing-box list
hy2 sing-box set <版本>
hy2 sing-box rollback
hy2 update
hy2 uninstall [--purge-data]
```

除只读查看外，多数维护操作需要 root。`hy2 update` 使用面板已检测到的目标版本；没有检测结果时会要求输入精确 `X.Y.Z`。

## 备份、恢复与卸载

### 备份

```bash
hy2 backup
# 或指定绝对路径
hy2 backup /root/hy2panel-backups/manual.tar.gz
```

备份默认写入 `/root/hy2panel-backups/`，权限为 0600，包含面板配置、Reality 私钥、Hysteria2 密码、证书副本、RackNerd 凭据和协议流量状态。它只是 tar.gz，**没有额外加密**；请离线加密并移出 VPS。Let’s Encrypt 账户、原始续期配置、sing-box 二进制和系统软件包不在备份内。

### 恢复

```bash
hy2 restore /root/hy2panel-backups/manual.tar.gz
```

恢复前会校验归档成员、内部 SHA-256、格式和面板配置，并自动创建一份恢复前快照。恢复只适用于当前工具能识别的备份格式和配置版本；仍应事先保留 VPS 快照，并在恢复后执行 `hy2 check`。

### 卸载

```bash
hy2 uninstall
# 同时删除 /etc/hy2panel、运行数据和本地凭据
hy2 uninstall --purge-data
```

卸载前会创建备份。默认保留配置与运行数据；`--purge-data` 才删除。卸载不会移除系统软件包、swap、`fstab` 项、Let’s Encrypt 账户及其原始证书数据。

## 面板升级与事务边界

「系统设置 → 面板更新」先从 GitHub 最新稳定 Release 取得**精确目标版本**，再提交含该版本的升级请求。服务器只下载 `v目标版本` 的固定 bundle 与 `.sha256`，并验证包内二进制报告的版本完全一致。

升级流程为：预检下载和校验 → 记录受管文件/单元快照 → 安装新文件 → 配置迁移与渲染 → 重启核心和面板 → 健康检查 → 提交。配置、二进制、脚本和 systemd 单元在同一维护锁内处理；失败时尝试恢复快照。`/etc/hy2panel/panel.json` 不会被新包直接覆盖，现有域名、UUID、密钥、密码与 RackNerd 凭据会保留。

升级使用原子文件替换、阶段日志和开机恢复单元来降低中断风险，但**不承诺任意掉电点都能无人值守恢复**。归档内 SHA-256 用于发现传输或存储损坏，不是可信签名；只恢复由自己创建并安全保管的备份。断电、文件系统损坏、磁盘耗尽或回滚本身失败后，应查看：

```bash
hy2 status
hy2 logs upgrade
systemctl status hy2panel-recover.service --no-pager
cat /etc/hy2panel/upgrade.result
```

重大升级前仍应运行 `hy2 backup` 并保留 VPS 快照。升级期间面板和节点可能短暂重启；不要并行执行恢复、核心切换或其他维护操作。

## 从 v0.4.x 迁移到 v0.5.x

v0.5 的 bundle 与维护脚本新增事务库、`hy2`、备份/恢复/卸载、sing-box allowlist 和恢复单元。v0.4.x 已安装的旧升级脚本不知道这些文件，**不要直接用旧面板的一键更新跨到 v0.5**。

在服务器上先备份 `/etc/hy2panel`、`/root/hy2panel-credentials` 和证书，最好再创建 VPS 快照。然后从目标 v0.5.x Release 下载 bundle 内的自包含迁移器并执行：

```bash
curl -fsSL "https://github.com/LinyuFiee/hy2panel/releases/download/v0.5.0/migrate-v04-to-v05.sh" -o /tmp/migrate-v04-to-v05.sh
bash /tmp/migrate-v04-to-v05.sh
```

迁移器会重新下载并校验目标 bundle，先安装事务库、升级/恢复与 sing-box 迁移组件，再提交精确目标升级。迁移属于需要维护窗口的操作；仅把新 `hy2panel-upgrade.sh` 单文件覆盖到旧系统并不够。

v0.5 成功迁移后会禁用并删除旧 `hy2panel-firewall.service`、`/etc/hy2panel/firewall.nft` 和 `inet hy2panel` nftables 表，面板 8443 改为不限制来源 IP；迁移前务必确认外层防火墙策略和强密码。旧配置中的 `admin_cidr` 仅为兼容读取，不再控制访问。

v0.3.x 应先按对应历史版本说明升级到最新 v0.4.x，再处理 v0.5 迁移，不建议跨两代直接覆盖。

## RackNerd 流量

在对应 VPS 的 RackNerd 控制页启用 API，取得该 VPS 独立的 API Key 和 API Hash，再到「系统设置 → RackNerd 官方流量」填写。面板只请求：

```text
action=info
bw=1
status=1
rdtype=json
```

不会调用 boot、reboot、shutdown 等电源操作。凭据仅保存在 0600 权限的 `/etc/hy2panel/panel.json`，状态接口不返回 Key/Hash。成功结果缓存 5 分钟，失败结果缓存 1 分钟。RackNerd 的套餐数据才接近供应商计费口径；协议流量卡片不能替代它。

## BBR、测速与自动恢复

BBR 按钮只应用项目内置 `/etc/hy2panel/99-hy2panel.conf`，不接受浏览器传入任意 sysctl。BBR 主要影响 Reality 的 TCP；Hysteria2 使用 QUIC 自有拥塞控制。

LibreSpeed 仅在点击后运行，默认 3 并发、下载和上传各 8 秒、HTTPS、禁用 ICMP。测速会占用带宽并计入供应商流量。测速时显示的实时网卡吞吐包含其他连接，最终结果以 LibreSpeed 输出为准。

systemd 会在进程退出后重启服务；看门狗每分钟检查配置和实际监听。连续 3 次失败后先重启 sing-box。只有显式开启「自动换端口」且仍未恢复，才从 `2053、2083、2087、2096、10443` 选择空闲端口，24 小时最多一次。换端口后必须重新导入客户端链接。

**服务监听正常不代表外部一定可达。** 本机无法可靠判断运营商或 GFW 封锁；需要客户端或外部 TCP/UDP 探测复核。

## 安全与真实限制

- 域名不会隐藏节点 IP；Reality/Hysteria2 客户端最终连接节点地址。
- 面板公网监听 8443，不再有项目内置 IP 白名单；HTTPS、强密码、登录限流和外层访问策略缺一不可。
- 登录限流为进程内状态：每个直接来源 IP 在 5 分钟窗口内最多 5 次失败，面板重启后状态重置；它不是完整 WAF 或抗 DDoS 方案。
- 客户端链接、二维码和备份都含完整连接凭据，不要公开分享。
- 初始密码仅写入 root 凭据文件；不要发到 issue、日志或截图。
- 安装器发现已有 `/etc/hy2panel/panel.json` 会停止，避免覆盖密钥。
- 面板服务仅附加 `systemd-journal` 组读取日志，不以 root 运行；维护单元仍具有固定的 root 权限。
- GitHub、Let’s Encrypt、ipwho.is、RackNerd 和 LibreSpeed 等外部服务不可用时，相应功能会失败或降级。
- 公网 IP 地理名称会把节点公网 IP 发送给 `https://ipwho.is`；结果成功缓存 24 小时、失败缓存 10 分钟，连接本身不依赖它。
- README 和测试中的 `203.0.113.10`、`198.51.100.20`、`192.0.2.10` 是 RFC 文档示例地址。

## 本地开发与测试

需要 Node.js 24 和 `go.mod` 指定的 Go 工具链。前端构建产物嵌入 Go 二进制：

```bash
cd web
npm ci
npm run build
cd ..
go test ./...
go test -race ./...
go vet ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -o dist/hy2panel-linux-amd64 ./cmd/hy2panel
```

UI 测试会启动仅绑定 `127.0.0.1` 的 mock Vite 服务，截图写入 Playwright 的测试产物目录，不会改写受版本控制的 `docs/images`：

```bash
cd web
npx playwright install --with-deps chromium
npm run test:ui
```

Shell 与 systemd 静态检查：

```bash
mapfile -d '' files < <(find scripts -type f \( -name '*.sh' -o -name 'hy2' \) -print0)
bash -n "${files[@]}"
shellcheck --shell=bash --severity=warning "${files[@]}"
systemd-analyze verify systemd/*.path systemd/*.service systemd/*.timer
```

`MOCK_UI=1` 只用于 Vite 开发服务器，不进入生产后端。完整 CI 还验证版本一致性、sing-box 清单中的每个资产、真实 `sing-box check`、Linux amd64 构建以及构建后工作树无 tracked 改动。

## 发布

v0.5 发布标签使用 `v0.5.x`。Release workflow 复用 CI，校验标签、`internal/version.Version`、`web/package.json` 和 lockfile 完全一致，以 ldflags 注入标签版本，生成：

```text
hy2panel-bundle-linux-amd64.tar.gz
hy2panel-bundle-linux-amd64.tar.gz.sha256
migrate-v04-to-v05.sh
```

bundle 包含 Linux 二进制、全部安装/维护脚本、事务库、sing-box 清单和当前 systemd 单元，不再包含旧 firewall 单元。GitHub 环境支持时还会为 bundle 生成 artifact attestation。

## 第三方项目

- [sing-box](https://github.com/SagerNet/sing-box)：GPL-3.0-or-later（另有上游名称/关联声明限制）；安装器按 allowlist 下载官方二进制，不重打包进 hy2panel bundle
- [Hairline](https://github.com/lucasmarkes/hairline)：MIT，指针互动线条图形
- [LibreSpeed CLI](https://github.com/librespeed/speedtest-cli)：LGPL-3.0，按需服务器测速
- [Lucide](https://github.com/lucide-icons/lucide)：ISC，界面图标

详细许可见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
