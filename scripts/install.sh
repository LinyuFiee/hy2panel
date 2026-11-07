#!/usr/bin/env bash
set -Eeuo pipefail

if [[ ${EUID} -ne 0 ]]; then
  printf '请使用 root 运行。\n' >&2
  exit 1
fi

: "${PANEL_DOMAIN:?请设置 PANEL_DOMAIN，例如 panel.example.com}"
: "${HY2_DOMAIN:?请设置 HY2_DOMAIN，例如 hy2.example.com}"
: "${ADMIN_IP:?请设置 ADMIN_IP，例如 203.0.113.10 或 203.0.113.10/32}"

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
PANEL_BINARY="${ROOT_DIR}/dist/hy2panel-linux-amd64"

for domain in "${PANEL_DOMAIN}" "${HY2_DOMAIN}"; do
  if [[ ! ${domain} =~ ^[A-Za-z0-9.-]+$ ]]; then
    printf '域名包含非法字符: %s\n' "${domain}" >&2
    exit 1
  fi
done
if [[ ! ${ADMIN_IP} =~ ^[0-9A-Fa-f:./]+$ ]]; then
  printf '管理 IP 或 CIDR 包含非法字符。\n' >&2
  exit 1
fi

if [[ ! -f ${PANEL_BINARY} ]]; then
  printf '缺少 Linux 面板二进制: %s\n' "${PANEL_BINARY}" >&2
  exit 1
fi

source /etc/os-release
if [[ ${ID} != debian || ${VERSION_ID} != 12 ]]; then
  printf '此安装器仅支持 Debian 12。\n' >&2
  exit 1
fi

if [[ $(uname -m) != x86_64 ]]; then
  printf '当前安装包仅支持 linux/amd64。\n' >&2
  exit 1
fi

if [[ -f /etc/hy2panel/panel.json ]]; then
  printf '检测到现有配置，为避免覆盖密钥，安装已停止。\n' >&2
  exit 1
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y ca-certificates curl jq tar openssl certbot nftables

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

SING_BOX_TAG="$(curl -fsSL https://api.github.com/repos/SagerNet/sing-box/releases/latest | jq -r .tag_name)"
if [[ ! ${SING_BOX_TAG} =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  printf '无法确定 sing-box 稳定版本。\n' >&2
  exit 1
fi
SING_BOX_VERSION="${SING_BOX_TAG#v}"
SING_BOX_ARCHIVE="sing-box-${SING_BOX_VERSION}-linux-amd64.tar.gz"
curl -fL "https://github.com/SagerNet/sing-box/releases/download/${SING_BOX_TAG}/${SING_BOX_ARCHIVE}" -o "${WORK_DIR}/${SING_BOX_ARCHIVE}"
tar -xzf "${WORK_DIR}/${SING_BOX_ARCHIVE}" -C "${WORK_DIR}"

LIBRESPEED_VERSION=1.0.14
LIBRESPEED_ARCHIVE="librespeed-cli_${LIBRESPEED_VERSION}_linux_amd64.tar.gz"
LIBRESPEED_BASE_URL="https://github.com/librespeed/speedtest-cli/releases/download/v${LIBRESPEED_VERSION}"
curl -fL "${LIBRESPEED_BASE_URL}/${LIBRESPEED_ARCHIVE}" -o "${WORK_DIR}/${LIBRESPEED_ARCHIVE}"
curl -fsSL "${LIBRESPEED_BASE_URL}/checksums.txt" -o "${WORK_DIR}/librespeed-checksums.txt"
(
  cd "${WORK_DIR}"
  grep " ${LIBRESPEED_ARCHIVE}\$" librespeed-checksums.txt | sha256sum -c -
)
tar -xzf "${WORK_DIR}/${LIBRESPEED_ARCHIVE}" -C "${WORK_DIR}"

if ! id hy2panel >/dev/null 2>&1; then
  useradd --system --home-dir /var/lib/hy2panel --shell /usr/sbin/nologin hy2panel
fi

install -d -o hy2panel -g hy2panel -m 700 /etc/hy2panel
install -d -o root -g hy2panel -m 750 /etc/hy2panel/certs
install -d -o hy2panel -g hy2panel -m 750 /var/lib/hy2panel
install -o root -g root -m 755 "${PANEL_BINARY}" /usr/local/bin/hy2panel
install -o root -g root -m 755 "${WORK_DIR}/sing-box-${SING_BOX_VERSION}-linux-amd64/sing-box" /usr/local/bin/sing-box
install -o root -g root -m 755 "${WORK_DIR}/librespeed-cli" /usr/local/bin/librespeed-cli
install -o root -g root -m 750 "${ROOT_DIR}/scripts/hy2panel-apply.sh" /usr/local/sbin/hy2panel-apply
install -o root -g root -m 750 "${ROOT_DIR}/scripts/hy2panel-bbr.sh" /usr/local/sbin/hy2panel-bbr
install -o root -g root -m 750 "${ROOT_DIR}/scripts/cert-deploy.sh" /usr/local/sbin/hy2panel-cert-deploy
install -o root -g root -m 644 "${ROOT_DIR}/scripts/99-hy2panel.conf" /etc/hy2panel/99-hy2panel.conf
install -o root -g root -m 644 "${ROOT_DIR}/systemd/hy2panel.service" /etc/systemd/system/hy2panel.service
install -o root -g root -m 644 "${ROOT_DIR}/systemd/sing-box.service" /etc/systemd/system/sing-box.service
install -o root -g root -m 644 "${ROOT_DIR}/systemd/hy2panel-apply.service" /etc/systemd/system/hy2panel-apply.service
install -o root -g root -m 644 "${ROOT_DIR}/systemd/hy2panel-apply.path" /etc/systemd/system/hy2panel-apply.path
install -o root -g root -m 644 "${ROOT_DIR}/systemd/hy2panel-bbr.service" /etc/systemd/system/hy2panel-bbr.service
install -o root -g root -m 644 "${ROOT_DIR}/systemd/hy2panel-bbr.path" /etc/systemd/system/hy2panel-bbr.path
install -o root -g root -m 644 "${ROOT_DIR}/systemd/hy2panel-firewall.service" /etc/systemd/system/hy2panel-firewall.service
install -D -o root -g root -m 750 "${ROOT_DIR}/scripts/cert-deploy.sh" /etc/letsencrypt/renewal-hooks/deploy/hy2panel

printf 'PANEL_DOMAIN=%s\nHY2_DOMAIN=%s\n' "${PANEL_DOMAIN}" "${HY2_DOMAIN}" > /etc/hy2panel/domains.env
chmod 600 /etc/hy2panel/domains.env

if [[ -f /etc/sysctl.d/99-hy2panel.conf ]]; then
  cp /etc/sysctl.d/99-hy2panel.conf /etc/hy2panel/sysctl.backup
else
  : > /etc/hy2panel/sysctl.backup
fi
install -o root -g root -m 644 "${ROOT_DIR}/scripts/99-hy2panel.conf" /etc/sysctl.d/99-hy2panel.conf
if ! sysctl -p /etc/sysctl.d/99-hy2panel.conf; then
  if [[ -s /etc/hy2panel/sysctl.backup ]]; then
    cp /etc/hy2panel/sysctl.backup /etc/sysctl.d/99-hy2panel.conf
  else
    rm -f /etc/sysctl.d/99-hy2panel.conf
  fi
  printf '内核参数应用失败，已恢复原配置。\n' >&2
  exit 1
fi

if ! swapon --show=NAME --noheadings | grep -q .; then
  if [[ ! -f /swapfile ]]; then
    fallocate -l 1G /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
  fi
  swapon /swapfile
  if ! grep -qE '^/swapfile[[:space:]]' /etc/fstab; then
    printf '/swapfile none swap sw 0 0\n' >> /etc/fstab
  fi
fi

ACME_ACCOUNT_ARGS=(--register-unsafely-without-email)
if [[ -n ${ACME_EMAIL:-} ]]; then
  ACME_ACCOUNT_ARGS=(--email "${ACME_EMAIL}")
fi
certbot certonly --standalone --non-interactive --agree-tos "${ACME_ACCOUNT_ARGS[@]}" --preferred-challenges http --http-01-port 80 --keep-until-expiring --cert-name "${PANEL_DOMAIN}" -d "${PANEL_DOMAIN}"
certbot certonly --standalone --non-interactive --agree-tos "${ACME_ACCOUNT_ARGS[@]}" --preferred-challenges http --http-01-port 80 --keep-until-expiring --cert-name "${HY2_DOMAIN}" -d "${HY2_DOMAIN}"
/usr/local/sbin/hy2panel-cert-deploy

if [[ -z ${PUBLIC_HOST:-} ]]; then
  PUBLIC_HOST="$(hostname -I)"
  PUBLIC_HOST="${PUBLIC_HOST%% *}"
fi
if [[ -z ${PUBLIC_HOST} ]]; then
  printf '无法确定公网地址，请通过 PUBLIC_HOST 显式指定。\n' >&2
  exit 1
fi

ADMIN_PASSWORD="$(openssl rand -base64 24 | tr -d '\n')"
printf '%s' "${ADMIN_PASSWORD}" | /usr/local/bin/hy2panel init --config /etc/hy2panel/panel.json --panel-domain "${PANEL_DOMAIN}" --hy2-domain "${HY2_DOMAIN}" --public-host "${PUBLIC_HOST}" --admin-ip "${ADMIN_IP}"
chown hy2panel:hy2panel /etc/hy2panel/panel.json
chmod 600 /etc/hy2panel/panel.json
install -o root -g root -m 600 /dev/null /root/hy2panel-credentials
printf '面板地址: https://%s:8443\n管理白名单: %s\n管理员密码: %s\n' "${PANEL_DOMAIN}" "${ADMIN_IP}" "${ADMIN_PASSWORD}" > /root/hy2panel-credentials
chmod 600 /root/hy2panel-credentials
/usr/local/bin/hy2panel render --config /etc/hy2panel/panel.json --output /etc/hy2panel/sing-box.pending.json
chown hy2panel:hy2panel /etc/hy2panel/sing-box.pending.json

NFT_FAMILY=ip
if [[ ${ADMIN_IP} == *:* ]]; then
  NFT_FAMILY=ip6
fi
{
  printf 'table inet hy2panel {\n'
  printf '  chain input {\n'
  printf '    type filter hook input priority 10; policy accept;\n'
  printf '    tcp dport 8443 %s saddr %s accept\n' "${NFT_FAMILY}" "${ADMIN_IP}"
  printf '    tcp dport 8443 drop\n'
  printf '  }\n'
  printf '}\n'
} > /etc/hy2panel/firewall.nft
chmod 600 /etc/hy2panel/firewall.nft

systemctl daemon-reload
systemctl enable certbot.timer hy2panel-apply.path hy2panel-bbr.path hy2panel-firewall.service hy2panel.service sing-box.service
systemctl start hy2panel-firewall.service
systemctl start hy2panel-apply.path
systemctl start hy2panel-bbr.path
if ! /usr/local/sbin/hy2panel-apply; then
  printf 'sing-box 首次启动失败: %s\n' "$(< /etc/hy2panel/apply.result)" >&2
  exit 1
fi
systemctl start hy2panel.service
systemctl is-active --quiet hy2panel.service
systemctl is-active --quiet sing-box.service


printf '\n安装完成。\n面板: https://%s:8443\n凭据文件: /root/hy2panel-credentials\n' "${PANEL_DOMAIN}"
sysctl net.ipv4.tcp_congestion_control net.core.default_qdisc