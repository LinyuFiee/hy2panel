#!/usr/bin/env bash
set -Eeuo pipefail

BASE=/etc/hy2panel
REQUEST="${BASE}/upgrade.request"
RESULT="${BASE}/upgrade.result"
BACKUP="${BASE}/upgrade-backup"
REPOSITORY=LinyuFiee/hy2panel
BUNDLE=hy2panel-bundle-linux-amd64.tar.gz
WORK_DIR=""

write_result() {
  local temporary
  temporary="$(mktemp "${BASE}/.upgrade-result.XXXXXX")"
  printf '%s\n' "$1" > "${temporary}"
  chown hy2panel:hy2panel "${temporary}"
  chmod 600 "${temporary}"
  mv "${temporary}" "${RESULT}"
}

rollback() {
  if [[ -d ${BACKUP}/bin ]]; then
    cp -a "${BACKUP}/bin/hy2panel" /usr/local/bin/hy2panel
    cp -a "${BACKUP}/sbin/." /usr/local/sbin/
    cp -a "${BACKUP}/systemd/." /etc/systemd/system/
    systemctl daemon-reload
    systemctl restart hy2panel.service || true
  fi
}

cleanup() {
  rm -f "${REQUEST}"
  if [[ -n ${WORK_DIR} && -d ${WORK_DIR} ]]; then
    rm -rf "${WORK_DIR}"
  fi
}
trap cleanup EXIT

rm -f "${RESULT}"
if [[ ! -f ${BASE}/panel.json ]]; then
  write_result '缺少现有面板配置'
  exit 1
fi

WORK_DIR="$(mktemp -d)"
BASE_URL="https://github.com/${REPOSITORY}/releases/latest/download"
curl -fL --connect-timeout 20 --max-time 300 "${BASE_URL}/${BUNDLE}" -o "${WORK_DIR}/${BUNDLE}"
curl -fsSL --connect-timeout 20 --max-time 60 "${BASE_URL}/${BUNDLE}.sha256" -o "${WORK_DIR}/${BUNDLE}.sha256"
(
  cd "${WORK_DIR}"
  sha256sum -c "${BUNDLE}.sha256"
)
mkdir "${WORK_DIR}/bundle"
tar -xzf "${WORK_DIR}/${BUNDLE}" -C "${WORK_DIR}/bundle"

rm -rf "${BACKUP}"
mkdir -p "${BACKUP}/bin" "${BACKUP}/sbin" "${BACKUP}/systemd"
cp -a /usr/local/bin/hy2panel "${BACKUP}/bin/"
cp -a /usr/local/sbin/hy2panel-* "${BACKUP}/sbin/"
cp -a /etc/systemd/system/hy2panel*.service /etc/systemd/system/hy2panel*.path /etc/systemd/system/hy2panel*.timer /etc/systemd/system/sing-box.service "${BACKUP}/systemd/"

install -o root -g root -m 755 "${WORK_DIR}/bundle/dist/hy2panel-linux-amd64" /usr/local/bin/hy2panel
install -o root -g root -m 750 "${WORK_DIR}/bundle/scripts/hy2panel-apply.sh" /usr/local/sbin/hy2panel-apply
install -o root -g root -m 750 "${WORK_DIR}/bundle/scripts/hy2panel-bbr.sh" /usr/local/sbin/hy2panel-bbr
install -o root -g root -m 750 "${WORK_DIR}/bundle/scripts/hy2panel-watchdog.sh" /usr/local/sbin/hy2panel-watchdog
install -o root -g root -m 750 "${WORK_DIR}/bundle/scripts/hy2panel-upgrade.sh" /usr/local/sbin/hy2panel-upgrade
install -o root -g root -m 750 "${WORK_DIR}/bundle/scripts/cert-deploy.sh" /usr/local/sbin/hy2panel-cert-deploy
install -o root -g root -m 644 "${WORK_DIR}/bundle/systemd/hy2panel.service" /etc/systemd/system/hy2panel.service
install -o root -g root -m 644 "${WORK_DIR}/bundle/systemd/sing-box.service" /etc/systemd/system/sing-box.service
install -o root -g root -m 644 "${WORK_DIR}/bundle/systemd/hy2panel-apply.service" /etc/systemd/system/hy2panel-apply.service
install -o root -g root -m 644 "${WORK_DIR}/bundle/systemd/hy2panel-apply.path" /etc/systemd/system/hy2panel-apply.path
install -o root -g root -m 644 "${WORK_DIR}/bundle/systemd/hy2panel-bbr.service" /etc/systemd/system/hy2panel-bbr.service
install -o root -g root -m 644 "${WORK_DIR}/bundle/systemd/hy2panel-bbr.path" /etc/systemd/system/hy2panel-bbr.path
install -o root -g root -m 644 "${WORK_DIR}/bundle/systemd/hy2panel-watchdog.service" /etc/systemd/system/hy2panel-watchdog.service
install -o root -g root -m 644 "${WORK_DIR}/bundle/systemd/hy2panel-watchdog.timer" /etc/systemd/system/hy2panel-watchdog.timer
install -o root -g root -m 644 "${WORK_DIR}/bundle/systemd/hy2panel-upgrade.service" /etc/systemd/system/hy2panel-upgrade.service
install -o root -g root -m 644 "${WORK_DIR}/bundle/systemd/hy2panel-upgrade.path" /etc/systemd/system/hy2panel-upgrade.path
install -o root -g root -m 644 "${WORK_DIR}/bundle/systemd/hy2panel-firewall.service" /etc/systemd/system/hy2panel-firewall.service

systemctl daemon-reload
if ! /usr/local/bin/hy2panel render --config "${BASE}/panel.json" --output "${BASE}/sing-box.pending.json" || ! /usr/local/sbin/hy2panel-apply; then
  rollback
  write_result '新配置应用失败，已回滚'
  exit 1
fi
systemctl enable hy2panel-apply.path hy2panel-bbr.path hy2panel-watchdog.timer hy2panel-upgrade.path
rm -f "${REQUEST}"
systemctl restart hy2panel-apply.path hy2panel-bbr.path hy2panel-watchdog.timer
systemctl start hy2panel-upgrade.path
if ! systemctl restart hy2panel.service; then
  rollback
  write_result '面板启动失败，已回滚'
  exit 1
fi
write_result ok