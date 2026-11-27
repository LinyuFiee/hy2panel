#!/usr/bin/env bash
set -Eeuo pipefail

BASE=/etc/hy2panel
STATE_ROOT=/var/lib/hy2panel-system
TRANSACTIONS="${STATE_ROOT}/transactions"
ACTIVE="${TRANSACTIONS}/active"
RESULT="${BASE}/upgrade.result"

source /usr/local/lib/hy2panel/hy2panel-transaction.sh

write_result() {
  local status=$1 phase=$2 message=$3 rollback_succeeded=${4:-false}
  local operation_id target temporary
  operation_id="$(jq -r '.id // "unknown"' "${ACTIVE}/journal.json" 2>/dev/null || printf unknown)"
  target="$(jq -r '.target_version // ""' "${ACTIVE}/journal.json" 2>/dev/null || true)"
  temporary="$(mktemp "${BASE}/.upgrade-result.XXXXXX")"
  jq -n --arg id "${operation_id}" --arg status "${status}" --arg phase "${phase}" --arg target "${target}" --arg message "${message}" --argjson restored "${rollback_succeeded}" \
    '{schema:1,id:$id,status:$status,phase:$phase,target_version:$target,message:$message,rollback:{attempted:true,succeeded:$restored}}' > "${temporary}"
  chmod 600 "${temporary}"
  chown hy2panel:hy2panel "${temporary}"
  sync -f "${BASE}" 2>/dev/null || true
  mv -fT -- "${temporary}" "${RESULT}"
}

restore_snapshot() {
  [[ -d ${ACTIVE}/snapshot ]] || return 1
  local snapshot="${ACTIVE}/snapshot" path record exists backup failed=0
  if [[ -f ${snapshot}/managed.json ]]; then
    while IFS= read -r record; do
      path="$(jq -r '.path' <<<"${record}")"
      exists="$(jq -r '.exists' <<<"${record}")"
      backup="$(jq -r '.backup // ""' <<<"${record}")"
      if [[ ${exists} == true ]]; then
        hy2panel_restore_snapshot_entry "${snapshot}/files/${backup}" "${path}" || failed=1
      else
        rm -rf "${path}" || failed=1
      fi
    done < <(jq -c '.[]' "${snapshot}/managed.json")
  fi
  systemctl daemon-reload || failed=1
  if [[ -f ${snapshot}/units.json ]]; then
    while IFS= read -r record; do
      local unit enabled active
      unit="$(jq -r '.unit' <<<"${record}")"
      enabled="$(jq -r '.enabled' <<<"${record}")"
      active="$(jq -r '.active' <<<"${record}")"
      if [[ ${enabled} == true ]]; then
        systemctl enable "${unit}" >/dev/null 2>&1 || failed=1
      else
        systemctl disable "${unit}" >/dev/null 2>&1 || true
      fi
      if [[ ${active} == true && ${unit} != hy2panel-recover.service ]]; then
        systemctl --no-block start "${unit}" >/dev/null 2>&1 || true
      elif [[ ${active} != true && ${unit} != hy2panel-recover.service ]]; then
        systemctl --no-block stop "${unit}" >/dev/null 2>&1 || true
      fi
    done < <(jq -c '.[]' "${snapshot}/units.json")
  fi
  if [[ -f ${snapshot}/firewall-active ]] && [[ $(< "${snapshot}/firewall-active") == true ]]; then
    nft -f "${snapshot}/firewall.nft" >/dev/null 2>&1 || failed=1
  else
    nft delete table inet hy2panel >/dev/null 2>&1 || true
  fi
  return "${failed}"
}

if [[ ! -f ${ACTIVE}/journal.json ]]; then
  exit 0
fi
if ! hy2panel_lock_maintenance; then
  exit 0
fi
if ! hy2panel_lock_runtime; then
  exit 0
fi

[[ -f ${ACTIVE}/journal.json ]] || exit 0
case $(jq -r '.status // ""' "${ACTIVE}/journal.json") in
  running | rolling_back) ;;
  *) exit 0 ;;
esac
hy2panel_prepare_config_dir
if restore_snapshot; then
  jq '.status="rolled_back" | .phase="recovery" | .finished_at=(now|todateiso8601)' "${ACTIVE}/journal.json" > "${ACTIVE}/journal.tmp"
  mv "${ACTIVE}/journal.tmp" "${ACTIVE}/journal.json"
  write_result failed_rolled_back recovery '检测到未完成升级，已恢复上一稳定版本' true
else
  jq '.status="rollback_failed" | .phase="recovery" | .finished_at=(now|todateiso8601)' "${ACTIVE}/journal.json" > "${ACTIVE}/journal.tmp"
  mv "${ACTIVE}/journal.tmp" "${ACTIVE}/journal.json"
  write_result rollback_failed recovery '检测到未完成升级，但自动恢复失败，请人工检查' false
  exit 1
fi
