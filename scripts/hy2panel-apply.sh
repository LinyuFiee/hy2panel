#!/usr/bin/env bash
set -Eeuo pipefail

BASE=/etc/hy2panel
PENDING="${BASE}/sing-box.pending.json"
CURRENT="${BASE}/sing-box.json"
BACKUP="${BASE}/sing-box.previous.json"
REQUEST="${BASE}/apply.request"
RESULT="${BASE}/apply.result"
LOG_FILE="${BASE}/apply.log"

write_result() {
  printf '%s\n' "$1" > "${RESULT}"
  chown hy2panel:hy2panel "${RESULT}"
  chmod 600 "${RESULT}"
}

restore_previous() {
  if [[ -f ${BACKUP} ]]; then
    install -o hy2panel -g hy2panel -m 600 "${BACKUP}" "${CURRENT}"
    systemctl restart sing-box.service || true
  else
    rm -f "${CURRENT}"
    systemctl stop sing-box.service || true
  fi
}

rm -f "${REQUEST}" "${RESULT}"
: > "${LOG_FILE}"
chmod 600 "${LOG_FILE}"

if [[ ! -f ${PENDING} ]]; then
  write_result '缺少候选配置'
  exit 1
fi

if ! /usr/local/bin/sing-box check -c "${PENDING}" >"${LOG_FILE}" 2>&1; then
  write_result "$(tr '\n' ' ' < "${LOG_FILE}")"
  exit 1
fi

if [[ -f ${CURRENT} ]]; then
  install -o hy2panel -g hy2panel -m 600 "${CURRENT}" "${BACKUP}"
fi
install -o hy2panel -g hy2panel -m 600 "${PENDING}" "${CURRENT}"

if ! systemctl restart sing-box.service >>"${LOG_FILE}" 2>&1; then
  restore_previous
  write_result "$(tr '\n' ' ' < "${LOG_FILE}")"
  exit 1
fi

sleep 2
if ! systemctl is-active --quiet sing-box.service; then
  systemctl status sing-box.service --no-pager >>"${LOG_FILE}" 2>&1 || true
  restore_previous
  write_result "$(tr '\n' ' ' < "${LOG_FILE}")"
  exit 1
fi

rm -f "${PENDING}"
write_result ok