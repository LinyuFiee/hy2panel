#!/usr/bin/env bash
set -Eeuo pipefail

BASE=/etc/hy2panel
SYSTEM_STATE=/var/lib/hy2panel-system
PENDING="${BASE}/sing-box.pending.json"
PANEL_PENDING="${BASE}/panel.pending.json"
CURRENT="${BASE}/sing-box.json"
PANEL_CURRENT="${BASE}/panel.json"
REQUEST="${BASE}/apply.request"
RESULT="${BASE}/apply.result"
LOG_FILE="${SYSTEM_STATE}/apply.log"
WORK_DIR=""
REQUEST_JSON=0
OPERATION_ID=legacy

source /usr/local/lib/hy2panel/hy2panel-transaction.sh

write_result() {
  local status=$1 message=${2:-} rollback_attempted=${3:-false} rollback_succeeded=${4:-false}
  if [[ ${REQUEST_JSON} -eq 0 ]]; then
    if [[ ${status} == succeeded ]]; then
      hy2panel_atomic_text "${RESULT}" hy2panel hy2panel 600 ok
    else
      hy2panel_atomic_text "${RESULT}" hy2panel hy2panel 600 "${message:-配置应用失败}"
    fi
    return
  fi
  local temporary
  temporary="$(mktemp "${BASE}/.apply-result.XXXXXX")"
  jq -n \
    --arg id "${OPERATION_ID}" \
    --arg status "${status}" \
    --arg message "${message}" \
    --argjson attempted "${rollback_attempted}" \
    --argjson restored "${rollback_succeeded}" \
    '{schema:1,id:$id,status:$status,message:$message,rollback:{attempted:$attempted,succeeded:$restored}}' > "${temporary}"
  chown hy2panel:hy2panel "${temporary}"
  chmod 600 "${temporary}"
  sync -f "${temporary}" 2>/dev/null || true
  mv -f "${temporary}" "${RESULT}"
}

parse_request() {
  [[ -f ${REQUEST} ]] || return 0
  if jq -e 'type == "object" and .schema == 1 and .action == "apply" and (.id | type == "string" and length >= 16 and length <= 128)' "${REQUEST}" >/dev/null 2>&1; then
    REQUEST_JSON=1
    OPERATION_ID="$(jq -r '.id' "${REQUEST}")"
  fi
}

restore_snapshot() {
  local restored=0
  if [[ -f ${WORK_DIR}/panel.json ]]; then
    hy2panel_atomic_copy "${WORK_DIR}/panel.json" "${PANEL_CURRENT}" hy2panel hy2panel 600
  elif [[ -f ${WORK_DIR}/panel.missing ]]; then
    rm -f "${PANEL_CURRENT}"
  fi
  if [[ -f ${WORK_DIR}/sing-box.json ]]; then
    hy2panel_atomic_copy "${WORK_DIR}/sing-box.json" "${CURRENT}" hy2panel hy2panel 600
    if systemctl restart sing-box.service >>"${LOG_FILE}" 2>&1; then
      local binary
      binary="$(hy2panel_sing_box_binary)"
      if hy2panel_sing_box_healthy "${binary}" "${CURRENT}" "" 8; then
        restored=1
      fi
    fi
  elif [[ -f ${WORK_DIR}/sing-box.missing ]]; then
    rm -f "${CURRENT}"
    systemctl stop sing-box.service >>"${LOG_FILE}" 2>&1 || true
    restored=1
  fi
  [[ ${restored} -eq 1 ]]
}

cleanup() {
  rm -f "${REQUEST}"
  if [[ -n ${WORK_DIR} && -d ${WORK_DIR} ]]; then
    rm -rf "${WORK_DIR}"
  fi
}
trap cleanup EXIT

install -d -o root -g root -m 700 "${SYSTEM_STATE}"
parse_request
rm -f "${REQUEST}" "${RESULT}"
if [[ -L ${LOG_FILE} ]]; then
  rm -f "${LOG_FILE}"
fi
: > "${LOG_FILE}"
chown root:hy2panel "${LOG_FILE}"
chmod 640 "${LOG_FILE}"

if ! hy2panel_lock_runtime; then
  write_result failed_no_change '另一个节点维护操作正在运行'
  exit 1
fi

if [[ ! -f ${PENDING} || -L ${PENDING} || $(stat -c '%U:%G:%a' "${PENDING}") != hy2panel:hy2panel:600 ]]; then
  write_result failed_no_change '候选 sing-box 配置缺失或权限无效'
  exit 1
fi
if [[ -e ${PANEL_PENDING} ]]; then
  if [[ ! -f ${PANEL_PENDING} || -L ${PANEL_PENDING} || $(stat -c '%U:%G:%a' "${PANEL_PENDING}") != hy2panel:hy2panel:600 ]]; then
    write_result failed_no_change '候选面板配置权限无效'
    exit 1
  fi
fi

BINARY="$(hy2panel_sing_box_binary)"
if ! "${BINARY}" check -c "${PENDING}" >"${LOG_FILE}" 2>&1; then
  write_result failed_no_change "$(tr '\n' ' ' < "${LOG_FILE}")"
  exit 1
fi
if [[ -f ${PANEL_PENDING} ]] && ! /usr/local/bin/hy2panel check --config "${PANEL_PENDING}" >>"${LOG_FILE}" 2>&1; then
  write_result failed_no_change "$(tr '\n' ' ' < "${LOG_FILE}")"
  exit 1
fi

WORK_DIR="$(mktemp -d "${SYSTEM_STATE}/apply.XXXXXX")"
chmod 700 "${WORK_DIR}"
if [[ -f ${CURRENT} ]]; then
  cp -a "${CURRENT}" "${WORK_DIR}/sing-box.json"
else
  : > "${WORK_DIR}/sing-box.missing"
fi
if [[ -f ${PANEL_CURRENT} ]]; then
  cp -a "${PANEL_CURRENT}" "${WORK_DIR}/panel.json"
else
  : > "${WORK_DIR}/panel.missing"
fi

if [[ -f ${PANEL_PENDING} ]]; then
  hy2panel_atomic_copy "${PANEL_PENDING}" "${PANEL_CURRENT}" hy2panel hy2panel 600
fi
hy2panel_atomic_copy "${PENDING}" "${CURRENT}" hy2panel hy2panel 600

if ! systemctl restart sing-box.service >>"${LOG_FILE}" 2>&1 || ! hy2panel_sing_box_healthy "${BINARY}" "${CURRENT}" "" 8; then
  systemctl status sing-box.service --no-pager >>"${LOG_FILE}" 2>&1 || true
  if restore_snapshot; then
    write_result failed_rolled_back "$(tr '\n' ' ' < "${LOG_FILE}")" true true
  else
    write_result rollback_failed "$(tr '\n' ' ' < "${LOG_FILE}")" true false
  fi
  exit 1
fi

rm -f "${PENDING}" "${PANEL_PENDING}"
write_result succeeded
