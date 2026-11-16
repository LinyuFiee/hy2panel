#!/usr/bin/env bash
set -Eeuo pipefail

BASE=/etc/hy2panel
source /usr/local/lib/hy2panel/hy2panel-transaction.sh

PANEL_DOMAIN="$(jq -er '.panel.domain | select(type == "string" and test("^[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$"))' "${BASE}/panel.json")"
HY2_DOMAIN="$(jq -er '.hysteria2.domain | select(type == "string" and test("^[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$"))' "${BASE}/panel.json")"

if ! hy2panel_lock_runtime; then
  logger -t hy2panel-cert-deploy -- 'maintenance operation is active; certificate deployment deferred'
  exit 75
fi

install -o root -g hy2panel -m 640 "/etc/letsencrypt/live/${PANEL_DOMAIN}/fullchain.pem" "${BASE}/certs/panel.crt"
install -o root -g hy2panel -m 640 "/etc/letsencrypt/live/${PANEL_DOMAIN}/privkey.pem" "${BASE}/certs/panel.key"
install -o root -g hy2panel -m 640 "/etc/letsencrypt/live/${HY2_DOMAIN}/fullchain.pem" "${BASE}/certs/hy2.crt"
install -o root -g hy2panel -m 640 "/etc/letsencrypt/live/${HY2_DOMAIN}/privkey.pem" "${BASE}/certs/hy2.key"

if systemctl is-active --quiet hy2panel.service; then
  systemctl restart hy2panel.service
fi
if systemctl is-active --quiet sing-box.service; then
  systemctl restart sing-box.service
fi
