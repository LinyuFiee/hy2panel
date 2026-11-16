#!/usr/bin/env bash
set -Eeuo pipefail

if [[ ${EUID} -ne 0 ]]; then
  printf '请使用 root 运行。\n' >&2
  exit 1
fi

: "${PANEL_DOMAIN:?请设置 PANEL_DOMAIN，例如 panel.example.com}"
: "${HY2_DOMAIN:?请设置 HY2_DOMAIN，例如 hy2.example.com}"

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
PANEL_BINARY="${ROOT_DIR}/dist/hy2panel-linux-amd64"

for domain in "${PANEL_DOMAIN}" "${HY2_DOMAIN}"; do
  if [[ ! ${domain} =~ ^[A-Za-z0-9.-]+$ ]]; then
    printf '域名包含非法字符: %s\n' "${domain}" >&2
    exit 1
  fi
done
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
apt-get install -y ca-certificates curl jq tar openssl certbot util-linux iproute2

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

LIBRESPEED_VERSION=1.0.14
LIBRESPEED_ARCHIVE="librespeed-cli_${LIBRESPEED_VERSION}_linux_amd64.tar.gz"
LIBRESPEED_BASE_URL="https://github.com/librespeed/speedtest-cli/releases/download/v${LIBRESPEED_VERSION}"
curl -fL --proto '=https' --proto-redir '=https' "${LIBRESPEED_BASE_URL}/${LIBRESPEED_ARCHIVE}" -o "${WORK_DIR}/${LIBRESPEED_ARCHIVE}"
curl -fsSL --proto '=https' "${LIBRESPEED_BASE_URL}/checksums.txt" -o "${WORK_DIR}/librespeed-checksums.txt"
(
  cd "${WORK_DIR}"
  grep " ${LIBRESPEED_ARCHIVE}\$" librespeed-checksums.txt | sha256sum -c -
)
tar -xzf "${WORK_DIR}/${LIBRESPEED_ARCHIVE}" -C "${WORK_DIR}" --no-same-owner --no-same-permissions

if ! id hy2panel >/dev/null 2>&1; then
  useradd --system --home-dir /var/lib/hy2panel --shell /usr/sbin/nologin hy2panel
fi
install -d -o root -g hy2panel -m 1730 /etc/hy2panel
install -d -o root -g hy2panel -m 750 /etc/hy2panel/certs /var/lib/hy2panel-system /var/lib/hy2panel-system/sing-box
install -d -o hy2panel -g hy2panel -m 700 /var/lib/hy2panel
install -d -o root -g root -m 700 /var/lib/hy2panel-system/transactions
install -d -o root -g root -m 755 /usr/local/lib/hy2panel /usr/local/share/hy2panel

install -o root -g root -m 755 "${PANEL_BINARY}" /usr/local/bin/hy2panel
install -o root -g root -m 755 "${ROOT_DIR}/scripts/hy2" /usr/local/bin/hy2
install -o root -g root -m 755 "${WORK_DIR}/librespeed-cli" /usr/local/bin/librespeed-cli
for name in hy2panel-apply hy2panel-bbr hy2panel-watchdog hy2panel-upgrade hy2panel-recover hy2panel-sing-box hy2panel-backup hy2panel-restore hy2panel-uninstall; do
  install -o root -g root -m 750 "${ROOT_DIR}/scripts/${name}.sh" "/usr/local/sbin/${name}"
done
install -o root -g root -m 750 "${ROOT_DIR}/scripts/cert-deploy.sh" /usr/local/sbin/hy2panel-cert-deploy
install -o root -g root -m 644 "${ROOT_DIR}/scripts/lib/hy2panel-transaction.sh" /usr/local/lib/hy2panel/hy2panel-transaction.sh
source /usr/local/lib/hy2panel/hy2panel-transaction.sh
install -o root -g root -m 644 "${ROOT_DIR}/packaging/sing-box-versions.json" /usr/local/share/hy2panel/sing-box-versions.json
install -o root -g root -m 644 "${ROOT_DIR}/scripts/99-hy2panel.conf" /etc/hy2panel/99-hy2panel.conf
for unit_file in "${ROOT_DIR}"/systemd/*; do
  install -o root -g root -m 644 "${unit_file}" "/etc/systemd/system/$(basename -- "${unit_file}")"
done
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

SWAPFILE_CREATED=false
FSTAB_SWAP_ADDED=false
if ! swapon --show=NAME --noheadings | grep -q .; then
  if [[ ! -f /swapfile ]]; then
    fallocate -l 1G /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    SWAPFILE_CREATED=true
  fi
  swapon /swapfile
  if ! grep -qE '^/swapfile[[:space:]]' /etc/fstab; then
    printf '/swapfile none swap sw 0 0\n' >> /etc/fstab
    FSTAB_SWAP_ADDED=true
  fi
fi

ACME_ACCOUNT_ARGS=(--register-unsafely-without-email)
if [[ -n ${ACME_EMAIL:-} ]]; then
  ACME_ACCOUNT_ARGS=(--email "${ACME_EMAIL}")
fi
certbot certonly --standalone --non-interactive --agree-tos "${ACME_ACCOUNT_ARGS[@]}" --preferred-challenges http --http-01-port 80 --keep-until-expiring --cert-name "${PANEL_DOMAIN}" -d "${PANEL_DOMAIN}"
certbot certonly --standalone --non-interactive --agree-tos "${ACME_ACCOUNT_ARGS[@]}" --preferred-challenges http --http-01-port 80 --keep-until-expiring --cert-name "${HY2_DOMAIN}" -d "${HY2_DOMAIN}"
install -o root -g hy2panel -m 640 "/etc/letsencrypt/live/${PANEL_DOMAIN}/fullchain.pem" /etc/hy2panel/certs/panel.crt
install -o root -g hy2panel -m 640 "/etc/letsencrypt/live/${PANEL_DOMAIN}/privkey.pem" /etc/hy2panel/certs/panel.key
install -o root -g hy2panel -m 640 "/etc/letsencrypt/live/${HY2_DOMAIN}/fullchain.pem" /etc/hy2panel/certs/hy2.crt
install -o root -g hy2panel -m 640 "/etc/letsencrypt/live/${HY2_DOMAIN}/privkey.pem" /etc/hy2panel/certs/hy2.key

if [[ -z ${PUBLIC_HOST:-} ]]; then
  PUBLIC_HOST="$(hostname -I)"
  PUBLIC_HOST="${PUBLIC_HOST%% *}"
fi
if [[ -z ${PUBLIC_HOST} ]]; then
  printf '无法确定公网地址，请通过 PUBLIC_HOST 显式指定。\n' >&2
  exit 1
fi

ADMIN_PASSWORD="$(openssl rand -base64 24 | tr -d '\n')"
printf '%s' "${ADMIN_PASSWORD}" | /usr/local/bin/hy2panel init --config /etc/hy2panel/panel.json --panel-domain "${PANEL_DOMAIN}" --hy2-domain "${HY2_DOMAIN}" --public-host "${PUBLIC_HOST}"
chown hy2panel:hy2panel /etc/hy2panel/panel.json
chmod 600 /etc/hy2panel/panel.json
install -o root -g root -m 600 /dev/null /root/hy2panel-credentials
printf '面板地址: https://%s:8443\n管理员密码: %s\n' "${PANEL_DOMAIN}" "${ADMIN_PASSWORD}" > /root/hy2panel-credentials

SING_BOX_VERSION="${SING_BOX_VERSION:-$(jq -r '.versions[] | select(.status == "recommended") | .version' /usr/local/share/hy2panel/sing-box-versions.json)}"
/usr/local/sbin/hy2panel-sing-box install "${SING_BOX_VERSION}"
systemctl daemon-reload
/usr/local/bin/hy2panel render --config /etc/hy2panel/panel.json --output /etc/hy2panel/sing-box.pending.json
chown hy2panel:hy2panel /etc/hy2panel/sing-box.pending.json

systemctl daemon-reload
systemctl enable certbot.timer hy2panel-recover.service hy2panel-apply.path hy2panel-bbr.path hy2panel-watchdog.timer hy2panel-upgrade.path hy2panel-sing-box.path hy2panel.service sing-box.service
hy2panel_atomic_text /etc/hy2panel/apply.request hy2panel hy2panel 600 apply
/usr/local/sbin/hy2panel-apply
systemctl start hy2panel-apply.path hy2panel-bbr.path hy2panel-watchdog.timer hy2panel-upgrade.path hy2panel-sing-box.path
systemctl start hy2panel.service
systemctl is-active --quiet hy2panel.service
systemctl is-active --quiet sing-box.service

jq -n --arg version "$(/usr/local/bin/hy2panel version --json | jq -r '.version')" --arg created "$(date -u +%FT%TZ)" --argjson created_swapfile "${SWAPFILE_CREATED}" --argjson added_fstab_swap "${FSTAB_SWAP_ADDED}" '{schema:1,panel_version:$version,installed_at:$created,created_swapfile:$created_swapfile,added_fstab_swap:$added_fstab_swap}' > /var/lib/hy2panel-system/installed-files.json
chmod 600 /var/lib/hy2panel-system/installed-files.json

printf '\n安装完成。\n面板: https://%s:8443\n面板端口不再限制来源 IP，请确保使用强密码。\n快捷命令: hy2\n凭据文件: /root/hy2panel-credentials\n' "${PANEL_DOMAIN}"
sysctl net.ipv4.tcp_congestion_control net.core.default_qdisc
