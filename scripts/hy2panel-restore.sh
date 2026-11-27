#!/usr/bin/env bash
set -Eeuo pipefail

ARCHIVE=${1:-}
EMERGENCY_MODE=0
if [[ ${2:-} == --emergency ]]; then
  EMERGENCY_MODE=1
fi
if [[ ${EUID} -ne 0 || -z ${ARCHIVE} ]]; then
  printf '用法: hy2 restore <备份文件>（需要 root）\n' >&2
  exit 2
fi
[[ -f ${ARCHIVE} && ! -L ${ARCHIVE} ]] || { printf '备份文件不存在或类型不安全。\n' >&2; exit 1; }
ARCHIVE="$(readlink -f -- "${ARCHIVE}")"
BASE=/etc/hy2panel
WORK_DIR=""
EMERGENCY=""
PANEL_WAS_ACTIVE=0
WATCHDOG_WAS_ACTIVE=0

source /usr/local/lib/hy2panel/hy2panel-transaction.sh

if ! hy2panel_lock_maintenance || ! hy2panel_lock_runtime; then
  printf '另一个维护操作正在运行。\n' >&2
  exit 1
fi

WORK_DIR="$(mktemp -d)"
CERT_STAGE=""
trap 'rm -rf -- "${WORK_DIR}"; if [[ -n ${CERT_STAGE} ]]; then rm -rf -- "${CERT_STAGE}"; fi' EXIT
hy2panel_archive_members_safe "${ARCHIVE}" || { printf '备份归档包含不安全成员。\n' >&2; exit 1; }
tar -xzf "${ARCHIVE}" -C "${WORK_DIR}" --no-same-owner --no-same-permissions
[[ -f ${WORK_DIR}/manifest.json && -f ${WORK_DIR}/checksums.txt && -f ${WORK_DIR}/etc/hy2panel/panel.json ]] || { printf '备份结构不完整。\n' >&2; exit 1; }
jq -e '.backup_format_version == 1 and (.config_version | type == "number")' "${WORK_DIR}/manifest.json" >/dev/null
(
  cd "${WORK_DIR}"
  while IFS= read -r line; do
    digest=${line%% *}
    relative=${line#*  }
    [[ ${digest} =~ ^[0-9a-f]{64}$ ]]
    [[ ${relative} == ./* && ${relative} != *'/../'* && ${relative} != ../* ]]
  done < checksums.txt
  sha256sum -c checksums.txt
)
/usr/local/bin/hy2panel check --config "${WORK_DIR}/etc/hy2panel/panel.json"

hy2panel_prepare_config_dir
if [[ ${EMERGENCY_MODE} -eq 0 ]]; then
  EMERGENCY="/root/hy2panel-backups/pre-restore-$(date -u +%Y%m%d-%H%M%S).tar.gz"
  export HY2PANEL_MAINTENANCE_LOCK_HELD=1
  /usr/local/sbin/hy2panel-backup "${EMERGENCY}"
fi
systemctl is-active --quiet hy2panel.service && PANEL_WAS_ACTIVE=1
systemctl is-active --quiet hy2panel-watchdog.timer && WATCHDOG_WAS_ACTIVE=1
systemctl stop hy2panel-watchdog.timer hy2panel.service

restore_failed() {
  local code=$?
  trap - ERR
  printf '恢复失败，正在恢复操作前备份。\n' >&2
  if [[ ${EMERGENCY_MODE} -eq 0 && -f ${EMERGENCY} ]]; then
    HY2PANEL_MAINTENANCE_LOCK_HELD=1 HY2PANEL_RUNTIME_LOCK_HELD=1 /usr/local/sbin/hy2panel-restore "${EMERGENCY}" --emergency || true
  fi
  exit "${code}"
}
if [[ ${EMERGENCY_MODE} -eq 0 ]]; then
  trap restore_failed ERR
fi

# A subscription issued before restore must not expose the restored credentials.
rm -f /var/lib/hy2panel/subscription.json
# certs is a root-controlled child in a panel-writable sticky directory.
if [[ ! -e ${BASE}/certs && ! -L ${BASE}/certs ]]; then
  mkdir -m 700 -- "${BASE}/certs"
fi
[[ $(LC_ALL=C stat -c '%u:%F' -- "${BASE}/certs") == '0:directory' ]]
install -d -o root -g hy2panel -m 750 "${BASE}/certs"
hy2panel_atomic_copy "${WORK_DIR}/etc/hy2panel/panel.json" "${BASE}/panel.json" hy2panel hy2panel 600
for name in domains.env 99-hy2panel.conf sysctl.backup; do
  [[ ! -f ${WORK_DIR}/etc/hy2panel/${name} ]] || hy2panel_atomic_copy "${WORK_DIR}/etc/hy2panel/${name}" "${BASE}/${name}" root root 600
done
if [[ -d ${WORK_DIR}/etc/hy2panel/certs ]]; then
  CERT_STAGE="$(mktemp -d "${BASE}/.restore.XXXXXX")"
  cp -aT -- "${WORK_DIR}/etc/hy2panel/certs" "${CERT_STAGE}/certs"
  chown -R root:hy2panel "${CERT_STAGE}/certs"
  find "${CERT_STAGE}/certs" -type d -exec chmod 750 {} +
  find "${CERT_STAGE}/certs" -type f -exec chmod 640 {} +
  rm -rf -- "${BASE}/certs"
  mv -T -- "${CERT_STAGE}/certs" "${BASE}/certs"
  rmdir -- "${CERT_STAGE}"
fi
[[ ! -f ${WORK_DIR}/root/hy2panel-credentials ]] || install -o root -g root -m 600 "${WORK_DIR}/root/hy2panel-credentials" /root/hy2panel-credentials
[[ ! -f ${WORK_DIR}/var/lib/hy2panel/traffic.json ]] || install -D -o hy2panel -g hy2panel -m 600 "${WORK_DIR}/var/lib/hy2panel/traffic.json" /var/lib/hy2panel/traffic.json

rm -f "${BASE}"/*.request "${BASE}"/*.result "${BASE}"/*.pending.json
/usr/local/bin/hy2panel migrate --config "${BASE}/panel.json"
/usr/local/bin/hy2panel render --config "${BASE}/panel.json" --output "${BASE}/sing-box.pending.json"
systemctl stop hy2panel-apply.path >/dev/null 2>&1 || true
hy2panel_atomic_text "${BASE}/apply.request" hy2panel hy2panel 600 apply
export HY2PANEL_RUNTIME_LOCK_HELD=1
/usr/local/sbin/hy2panel-apply
systemctl start hy2panel-apply.path
if [[ ${PANEL_WAS_ACTIVE} -eq 1 || ${EMERGENCY_MODE} -eq 0 ]]; then
  systemctl start hy2panel.service
  hy2panel_panel_healthy
fi
[[ ${WATCHDOG_WAS_ACTIVE} -eq 0 ]] || systemctl start hy2panel-watchdog.timer
trap - ERR
if [[ ${EMERGENCY_MODE} -eq 0 ]]; then
  printf '备份已恢复。恢复前快照：%s\n' "${EMERGENCY}"
else
  printf '紧急恢复已完成。\n'
fi
