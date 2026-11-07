#!/usr/bin/env bash
set -Eeuo pipefail

BASE=/etc/hy2panel
CONFIG="${BASE}/panel.json"
STATE="${BASE}/watchdog.failures"
BACKUP="${BASE}/panel.before-watchdog.json"
PANEL_STOPPED=0

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

exec 9>"${BASE}/watchdog.lock"
flock -n 9 || exit 0

if ! systemctl is-active --quiet sing-box.service; then
  log 'sing-box is not active; attempting restart'
  systemctl restart sing-box.service || true
  sleep 3
fi

if ! /usr/local/bin/sing-box check -c "${BASE}/sing-box.json" >/dev/null 2>&1; then
  log 'configuration check failed; automatic port change is suppressed'
  exit 1
fi

listeners_ok() {
  if [[ $(jq -r '.reality.enabled' "${CONFIG}") == true ]]; then
    local port
    port="$(jq -r '.reality.port' "${CONFIG}")"
    ss -H -lnt "sport = :${port}" | grep -q . || return 1
  fi
  if [[ $(jq -r '.hysteria2.enabled' "${CONFIG}") == true ]]; then
    local port
    port="$(jq -r '.hysteria2.port' "${CONFIG}")"
    ss -H -lnu "sport = :${port}" | grep -q . || return 1
  fi
  return 0
}

if listeners_ok; then
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
if listeners_ok; then
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
mv "${TEMP_CONFIG}" "${CONFIG}"

if ! /usr/local/bin/hy2panel render --config "${CONFIG}" --output "${BASE}/sing-box.pending.json" || ! /usr/local/sbin/hy2panel-apply; then
  install -o hy2panel -g hy2panel -m 600 "${BACKUP}" "${CONFIG}"
  /usr/local/bin/hy2panel render --config "${CONFIG}" --output "${BASE}/sing-box.pending.json" || true
  /usr/local/sbin/hy2panel-apply || true
  log 'automatic port change failed; previous configuration was restored'
  exit 1
fi

systemctl start hy2panel.service
PANEL_STOPPED=0
printf '0\n' > "${STATE}"
log "automatic port change succeeded: ${CURRENT_PORT} -> ${NEXT_PORT}; update clients from the panel"