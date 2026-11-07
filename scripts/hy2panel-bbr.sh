#!/usr/bin/env bash
set -Eeuo pipefail

BASE=/etc/hy2panel
TEMPLATE="${BASE}/99-hy2panel.conf"
TARGET=/etc/sysctl.d/99-hy2panel.conf
BACKUP="${BASE}/bbr.previous.conf"
REQUEST="${BASE}/bbr.request"
RESULT="${BASE}/bbr.result"
LOG_FILE="${BASE}/bbr.log"

write_result() {
  printf '%s\n' "$1" > "${RESULT}"
  chown hy2panel:hy2panel "${RESULT}"
  chmod 600 "${RESULT}"
}

rm -f "${REQUEST}" "${RESULT}"
: > "${LOG_FILE}"
chmod 600 "${LOG_FILE}"

if [[ ! -f ${TEMPLATE} ]]; then
  write_result '缺少内置 BBR 配置模板'
  exit 1
fi

if [[ -f ${TARGET} ]]; then
  cp "${TARGET}" "${BACKUP}"
else
  : > "${BACKUP}"
fi

if sysctl -p "${TEMPLATE}" >"${LOG_FILE}" 2>&1 && install -o root -g root -m 644 "${TEMPLATE}" "${TARGET}"; then
  write_result ok
  exit 0
fi

if [[ -s ${BACKUP} ]]; then
  cp "${BACKUP}" "${TARGET}"
  sysctl -p "${TARGET}" >>"${LOG_FILE}" 2>&1 || true
else
  rm -f "${TARGET}"
fi
write_result "$(tr '\n' ' ' < "${LOG_FILE}")"
exit 1