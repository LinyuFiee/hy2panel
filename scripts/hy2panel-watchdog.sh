#!/usr/bin/env bash
set -Eeuo pipefail

BASE=/etc/hy2panel
SYSTEM_STATE=/var/lib/hy2panel-system
CONFIG="${BASE}/panel.json"
STATE="${SYSTEM_STATE}/watchdog.failures"
BACKUP="${SYSTEM_STATE}/panel.before-watchdog.json"
PANEL_STOPPED=0

source /usr/local/lib/hy2panel/hy2panel-transaction.sh
install -d -o root -g root -m 700 "${SYSTEM_STATE}"

log() {
  logger -t hy2panel-watchdog -- "$*"
}

restore_panel() {
  if [[ ${PANEL_STOPPED} -eq 1 ]]; then
    systemctl start hy2panel.service || true
    PANEL_STOPPED=0
  fi
}
trap restore_panel EXIT

if ! hy2panel_lock_runtime; then
  log 'maintenance operation is active; skipping this watchdog cycle'
  exit 0
fi

BINARY="$(hy2panel_sing_box_binary)"
if ! systemctl is-active --quiet sing-box.service; then
  log 'sing-box is not active; attempting restart'
  systemctl restart sing-box.service || true
  sleep 3
fi

if ! "${BINARY}" check -c "${BASE}/sing-box.json" >/dev/null 2>&1; then
  log 'configuration check failed; automatic port change is suppressed'
  exit 1
fi

if hy2panel_listeners_ok "${CONFIG}"; then
  printf '0\n' > "${STATE}"
  exit 0
fi

FAILURES=0
if [[ -f ${STATE} ]]; then
  FAILURES="$(< "${STATE}")"
fi
if [[ ! ${FAILURES} =~ ^[0-9]+$ ]]; then
  FAILURES=0
fi
FAILURES=$((FAILURES + 1))
printf '%s\n' "${FAILURES}" > "${STATE}"
log "listener check failed ${FAILURES} consecutive times"

if ((FAILURES % 3 != 0)); then
  exit 0
fi

log 'attempting service restart after repeated listener failures'
systemctl restart sing-box.service || true
sleep 3
if hy2panel_listeners_ok "${CONFIG}"; then
  printf '0\n' > "${STATE}"
  log 'listeners recovered after restart'
  exit 0
fi

if [[ $(jq -r '.recovery.auto_port // false' "${CONFIG}") != true ]]; then
  log 'automatic port change is disabled; manual investigation is required'
  exit 0
fi

NOW="$(date +%s)"
LAST_CHANGE="$(jq -r '.recovery.last_port_change // 0' "${CONFIG}")"
if [[ ${LAST_CHANGE} =~ ^[0-9]+$ ]] && ((NOW - LAST_CHANGE < 86400)); then
  log 'automatic port change is cooling down for 24 hours'
  exit 0
fi

CURRENT_PORT="$(jq -r 'if .reality.enabled then .reality.port elif .hysteria2.enabled then .hysteria2.port else 443 end' "${CONFIG}")"
CANDIDATES=(2053 2083 2087 2096 10443)
START=0
for index in "${!CANDIDATES[@]}"; do
  if [[ ${CANDIDATES[$index]} == "${CURRENT_PORT}" ]]; then
    START=$((index + 1))
  fi
done
NEXT_PORT=""
for offset in 0 1 2 3 4; do
  index=$(((START + offset) % ${#CANDIDATES[@]}))
  candidate="${CANDIDATES[$index]}"
  if [[ ${candidate} == "${CURRENT_PORT}" || ${candidate} == 8443 ]]; then
    continue
  fi
  if ! ss -H -lnt "sport = :${candidate}" | grep -q . && ! ss -H -lnu "sport = :${candidate}" | grep -q .; then
    NEXT_PORT="${candidate}"
    break
  fi
done
if [[ -z ${NEXT_PORT} ]]; then
  log 'no unused candidate port is available'
  exit 1
fi

cp "${CONFIG}" "${BACKUP}"
systemctl stop hy2panel.service
PANEL_STOPPED=1
TEMP_CONFIG="$(mktemp "${BASE}/.watchdog-panel.XXXXXX")"
if ! jq --argjson port "${NEXT_PORT}" --argjson now "${NOW}" '.reality.port = $port | .hysteria2.port = $port | .recovery.last_port_change = $now' "${BACKUP}" > "${TEMP_CONFIG}"; then
  rm -f "${TEMP_CONFIG}"
  log 'failed to generate recovery configuration'
  exit 1
fi
chown hy2panel:hy2panel "${TEMP_CONFIG}"
chmod 600 "${TEMP_CONFIG}"
mv "${TEMP_CONFIG}" "${BASE}/panel.pending.json"

if ! /usr/local/bin/hy2panel render --config "${BASE}/panel.pending.json" --output "${BASE}/sing-box.pending.json"; then
  rm -f "${BASE}/panel.pending.json" "${BASE}/sing-box.pending.json"
  log 'automatic port change failed while rendering candidate configuration'
  exit 1
fi
hy2panel_atomic_text "${BASE}/apply.request" hy2panel hy2panel 600 apply
systemctl stop hy2panel-apply.path >/dev/null 2>&1 || true
export HY2PANEL_RUNTIME_LOCK_HELD=1
if ! /usr/local/sbin/hy2panel-apply; then
  systemctl start hy2panel-apply.path >/dev/null 2>&1 || true
  log 'automatic port change failed; previous configuration was restored'
  exit 1
fi
systemctl start hy2panel-apply.path

systemctl start hy2panel.service
PANEL_STOPPED=0
printf '0\n' > "${STATE}"
log "automatic port change succeeded: ${CURRENT_PORT} -> ${NEXT_PORT}; update clients from the panel"
