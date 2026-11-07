#!/usr/bin/env bash
set -Eeuo pipefail

source /etc/hy2panel/domains.env

install -o root -g hy2panel -m 640 "/etc/letsencrypt/live/${PANEL_DOMAIN}/fullchain.pem" /etc/hy2panel/certs/panel.crt
install -o root -g hy2panel -m 640 "/etc/letsencrypt/live/${PANEL_DOMAIN}/privkey.pem" /etc/hy2panel/certs/panel.key
install -o root -g hy2panel -m 640 "/etc/letsencrypt/live/${HY2_DOMAIN}/fullchain.pem" /etc/hy2panel/certs/hy2.crt
install -o root -g hy2panel -m 640 "/etc/letsencrypt/live/${HY2_DOMAIN}/privkey.pem" /etc/hy2panel/certs/hy2.key

if systemctl is-active --quiet hy2panel.service; then
  systemctl restart hy2panel.service
fi
if systemctl is-active --quiet sing-box.service; then
  systemctl restart sing-box.service
fi