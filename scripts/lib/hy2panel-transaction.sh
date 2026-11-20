#!/usr/bin/env bash

HY2PANEL_BASE="${HY2PANEL_BASE:-/etc/hy2panel}"
HY2PANEL_MAINTENANCE_LOCK="${HY2PANEL_MAINTENANCE_LOCK:-/run/lock/hy2panel-maintenance.lock}"
HY2PANEL_RUNTIME_LOCK="${HY2PANEL_RUNTIME_LOCK:-/run/lock/hy2panel-runtime.lock}"

hy2panel_atomic_text() {
  local path=$1 owner=$2 group=$3 mode=$4 value=$5
  local directory temporary
  directory="$(dirname -- "${path}")"
  temporary="$(mktemp "${directory}/.$(basename -- "${path}").XXXXXX")"
  printf '%s\n' "${value}" > "${temporary}"
  chown "${owner}:${group}" "${temporary}"
  chmod "${mode}" "${temporary}"
  sync -f "${temporary}" 2>/dev/null || sync "${temporary}" 2>/dev/null || true
  mv -f "${temporary}" "${path}"
  sync -f "${directory}" 2>/dev/null || true
}

hy2panel_atomic_copy() {
  local source=$1 target=$2 owner=$3 group=$4 mode=$5
  local directory temporary
  directory="$(dirname -- "${target}")"
  temporary="$(mktemp "${directory}/.$(basename -- "${target}").XXXXXX")"
  cp -- "${source}" "${temporary}"
  chown "${owner}:${group}" "${temporary}"
  chmod "${mode}" "${temporary}"
  sync -f "${temporary}" 2>/dev/null || sync "${temporary}" 2>/dev/null || true
  mv -f "${temporary}" "${target}"
  sync -f "${directory}" 2>/dev/null || true
}

hy2panel_lock_maintenance() {
  if [[ ${HY2PANEL_MAINTENANCE_LOCK_HELD:-0} == 1 ]]; then
    return 0
  fi
  install -d -o root -g root -m 755 "$(dirname -- "${HY2PANEL_MAINTENANCE_LOCK}")"
  exec 8>"${HY2PANEL_MAINTENANCE_LOCK}"
  flock -n 8 || {
    exec 8>&-
    return 1
  }
  export HY2PANEL_MAINTENANCE_LOCK_HELD=1
}

hy2panel_lock_runtime() {
  if [[ ${HY2PANEL_RUNTIME_LOCK_HELD:-0} == 1 ]]; then
    return 0
  fi
  install -d -o root -g root -m 755 "$(dirname -- "${HY2PANEL_RUNTIME_LOCK}")"
  exec 9>"${HY2PANEL_RUNTIME_LOCK}"
  flock -n 9 || {
    exec 9>&-
    return 1
  }
  export HY2PANEL_RUNTIME_LOCK_HELD=1
}

hy2panel_sing_box_binary() {
  if [[ -x /usr/local/lib/hy2panel/sing-box/current/sing-box ]]; then
    printf '%s\n' /usr/local/lib/hy2panel/sing-box/current/sing-box
  else
    printf '%s\n' /usr/local/bin/sing-box
  fi
}

hy2panel_listeners_ok() {
  local config=$1 port
  if [[ $(jq -r '.reality.enabled // false' "${config}") == true ]]; then
    port="$(jq -r '.reality.port' "${config}")"
    ss -H -lnt "sport = :${port}" | grep -q . || return 1
  fi
  if [[ $(jq -r '.hysteria2.enabled // false' "${config}") == true ]]; then
    port="$(jq -r '.hysteria2.port' "${config}")"
    ss -H -lnu "sport = :${port}" | grep -q . || return 1
  fi
}

hy2panel_sing_box_healthy() {
  local binary=$1 config=$2 expected_version=${3:-} attempts=${4:-8}
  local before after actual index
  "${binary}" check -c "${config}" >/dev/null 2>&1 || return 1
  if [[ -n ${expected_version} ]]; then
    actual="$("${binary}" version 2>/dev/null | sed -nE 's/^sing-box version ([0-9]+\.[0-9]+\.[0-9]+).*/\1/p' | head -n 1)"
    [[ ${actual} == "${expected_version}" ]] || return 1
  fi
  before="$(systemctl show sing-box.service -p NRestarts --value 2>/dev/null || printf 0)"
  for ((index = 0; index < attempts; index++)); do
    if systemctl is-active --quiet sing-box.service && hy2panel_listeners_ok "${HY2PANEL_BASE}/panel.json"; then
      sleep 1
      after="$(systemctl show sing-box.service -p NRestarts --value 2>/dev/null || printf 0)"
      [[ ${after} == "${before}" ]] && return 0
      before="${after}"
    else
      sleep 1
    fi
  done
  return 1
}

hy2panel_panel_healthy() {
  local expected_version=${1:-} domain address payload actual
  domain="$(jq -r '.panel.domain' "${HY2PANEL_BASE}/panel.json")"
  address="$(jq -r '.panel.listen' "${HY2PANEL_BASE}/panel.json")"
  address="${address##*:}"
  payload="$(curl -fsS --connect-timeout 3 --max-time 8 --resolve "${domain}:${address}:127.0.0.1" "https://${domain}:${address}/api/health")" || return 1
  [[ $(jq -r '.status // empty' <<<"${payload}") == ok ]] || return 1
  if [[ -n ${expected_version} ]]; then
    actual="$(jq -r '.version // empty' <<<"${payload}")"
    [[ ${actual} == "${expected_version}" ]] || return 1
  fi
}

hy2panel_verify_sha256_file() {
  local checksum_file=$1 expected_name=$2 directory=$3
  local digest marker filename actual file_size expected_size
  local -a checksum_lines=()
  [[ ${expected_name} =~ ^[A-Za-z0-9._-]+$ ]] || return 1
  [[ -f ${checksum_file} && ! -L ${checksum_file} ]] || return 1
  [[ -f ${directory}/${expected_name} && ! -L ${directory}/${expected_name} ]] || return 1
  file_size="$(wc -c < "${checksum_file}")" || return 1
  expected_size=$((64 + 2 + ${#expected_name} + 1))
  [[ ${file_size} -eq ${expected_size} || ${file_size} -eq $((expected_size + 1)) ]] || return 1
  mapfile -t checksum_lines < "${checksum_file}"
  [[ ${#checksum_lines[@]} -eq 1 ]] || return 1
  checksum_lines[0]="${checksum_lines[0]%$'\r'}"
  [[ ${checksum_lines[0]} =~ ^([0-9a-f]{64})(\ \ |\ \*)([^/\\]+)$ ]] || return 1
  digest=${BASH_REMATCH[1]}
  marker=${BASH_REMATCH[2]}
  filename=${BASH_REMATCH[3]}
  [[ ${marker} == "  " || ${marker} == " *" ]] || return 1
  [[ ${filename} == "${expected_name}" ]] || return 1
  actual="$(sha256sum "${directory}/${expected_name}" | cut -d' ' -f1)" || return 1
  [[ ${actual} == "${digest}" ]]
}

hy2panel_min_panel_version() {
  printf '%s\n' 0.6.1
}

hy2panel_version_at_least() {
  local candidate=${1:-} floor=${2:-$(hy2panel_min_panel_version)}
  local -a candidate_parts floor_parts
  local index
  [[ ${candidate} =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || return 1
  IFS=. read -r -a candidate_parts <<< "${candidate}"
  IFS=. read -r -a floor_parts <<< "${floor}"
  for index in 0 1 2; do
    if ((10#${candidate_parts[index]} > 10#${floor_parts[index]})); then
      return 0
    fi
    if ((10#${candidate_parts[index]} < 10#${floor_parts[index]})); then
      return 1
    fi
  done
  return 0
}

hy2panel_archive_members_safe() {
  local archive=$1 listing verbose name
  listing="$(mktemp)"
  tar -tzf "${archive}" > "${listing}"
  while IFS= read -r name; do
    name="${name#./}"
    [[ -z ${name} ]] && continue
    if [[ ${name} == /* || ${name} == ".." || ${name} == ../* || ${name} == */../* || ${name} == */.. ]]; then
      rm -f "${listing}"
      return 1
    fi
  done < "${listing}"
  verbose="$(mktemp)"
  tar -tzvf "${archive}" > "${verbose}"
  if grep -Eq '^[lhbcp]' "${verbose}"; then
    rm -f "${listing}" "${verbose}"
    return 1
  fi
  rm -f "${listing}" "${verbose}"
}
