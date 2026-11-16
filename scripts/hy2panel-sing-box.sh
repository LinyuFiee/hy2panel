#!/usr/bin/env bash
set -Eeuo pipefail

BASE=/etc/hy2panel
MANIFEST=/usr/local/share/hy2panel/sing-box-versions.json
ROOT=/usr/local/lib/hy2panel/sing-box
VERSIONS="${ROOT}/versions"
STATE_ROOT=/var/lib/hy2panel-system/sing-box
STATE="${STATE_ROOT}/state.json"
TRANSACTIONS=/var/lib/hy2panel-system/transactions
REQUEST="${BASE}/sing-box-update.request"
RESULT="${BASE}/sing-box-update.result"
REPOSITORY=SagerNet/sing-box
WORK_DIR=""
OPERATION_ID=""
ACTION=""
TARGET_VERSION=""
PHASE=requested
OLD_CURRENT=""
OLD_PREVIOUS=""

source /usr/local/lib/hy2panel/hy2panel-transaction.sh

cleanup() {
  rm -f "${REQUEST}"
  if [[ -n ${WORK_DIR} && -d ${WORK_DIR} ]]; then
    rm -rf "${WORK_DIR}"
  fi
}
trap cleanup EXIT

current_version() {
  if [[ -L ${ROOT}/current ]]; then
    basename -- "$(readlink -f "${ROOT}/current")"
  fi
}

previous_version() {
  if [[ -L ${ROOT}/previous ]]; then
    basename -- "$(readlink -f "${ROOT}/previous")"
  fi
}

recommended_version() {
  jq -r '.versions[] | select(.status == "recommended") | .version' "${MANIFEST}"
}

write_operation() {
  local status=$1 message=${2:-} rollback_attempted=${3:-false} rollback_succeeded=${4:-false}
  local temporary current previous recommended
  current="$(current_version)"
  previous="$(previous_version)"
  recommended="$(recommended_version)"
  temporary="$(mktemp "${STATE_ROOT}/.state.XXXXXX")"
  jq -n \
    --arg id "${OPERATION_ID}" \
    --arg action "${ACTION}" \
    --arg status "${status}" \
    --arg phase "${PHASE}" \
    --arg target "${TARGET_VERSION}" \
    --arg current "${current}" \
    --arg previous "${previous}" \
    --arg recommended "${recommended}" \
    --arg message "${message}" \
    --argjson attempted "${rollback_attempted}" \
    --argjson restored "${rollback_succeeded}" \
    '{schema:1,current_version:$current,previous_version:$previous,recommended_version:$recommended,operation:{id:$id,action:$action,status:$status,phase:$phase,target_version:$target,message:$message,rollback:{attempted:$attempted,succeeded:$restored}}}' > "${temporary}"
  chown root:hy2panel "${temporary}"
  chmod 640 "${temporary}"
  sync -f "${temporary}" 2>/dev/null || true
  mv -f "${temporary}" "${STATE}"
  if [[ ${status} != running ]]; then
    cp "${STATE}" "${RESULT}.tmp"
    chown hy2panel:hy2panel "${RESULT}.tmp"
    chmod 600 "${RESULT}.tmp"
    mv -f "${RESULT}.tmp" "${RESULT}"
  fi
}

set_phase() {
  PHASE=$1
  write_operation running "${2:-}"
}

manifest_entry() {
  jq -ce --arg version "$1" '.versions[] | select(.version == $version)' "${MANIFEST}"
}

verify_manifest() {
  jq -e '
    .schema == 1 and .platform == "linux-amd64" and
    ([.versions[] | select(.status == "recommended")] | length) == 1 and
    (.versions | length) > 0 and
    all(.versions[];
      (.version | test("^[0-9]+\\.[0-9]+\\.[0-9]+$")) and
      .tag == ("v" + .version) and
      (.status == "recommended" or .status == "supported") and
      (.asset_name == ("sing-box-" + .version + "-linux-amd64.tar.gz")) and
      (.size | type == "number" and . > 0) and
      (.sha256 | test("^[0-9a-f]{64}$"))
    ) and
    ([.versions[].version] | unique | length) == (.versions | length) and
    ([.versions[].asset_name] | unique | length) == (.versions | length)
  ' "${MANIFEST}" >/dev/null
}

safe_download() {
  local entry=$1 version tag asset expected_size expected_sha api release asset_json url digest archive actual_size actual_sha
  version="$(jq -r '.version' <<<"${entry}")"
  tag="$(jq -r '.tag' <<<"${entry}")"
  asset="$(jq -r '.asset_name' <<<"${entry}")"
  expected_size="$(jq -r '.size' <<<"${entry}")"
  expected_sha="$(jq -r '.sha256' <<<"${entry}")"
  api="https://api.github.com/repos/${REPOSITORY}/releases/tags/${tag}"
  release="${WORK_DIR}/release.json"
  curl -fsSL --proto '=https' --connect-timeout 20 --max-time 60 -H 'Accept: application/vnd.github+json' -H 'User-Agent: hy2panel' "${api}" -o "${release}"
  [[ $(stat -c %s "${release}") -le 2097152 ]]
  jq -e --arg tag "${tag}" '.tag_name == $tag and .draft == false and .prerelease == false' "${release}" >/dev/null
  [[ $(jq --arg name "${asset}" '[.assets[] | select(.name == $name)] | length' "${release}") -eq 1 ]]
  asset_json="$(jq -ce --arg name "${asset}" '.assets[] | select(.name == $name)' "${release}")"
  [[ $(jq -r '.size' <<<"${asset_json}") == "${expected_size}" ]]
  digest="$(jq -r '.digest // ""' <<<"${asset_json}")"
  if [[ -n ${digest} ]]; then
    [[ ${digest} == "sha256:${expected_sha}" ]]
  fi
  url="$(jq -r '.browser_download_url' <<<"${asset_json}")"
  [[ ${url} =~ ^https://github\.com/SagerNet/sing-box/releases/download/ ]]
  archive="${WORK_DIR}/${asset}"
  curl -fL --proto '=https' --proto-redir '=https' --connect-timeout 20 --max-time 300 "${url}" -o "${archive}"
  actual_size="$(stat -c %s "${archive}")"
  [[ ${actual_size} == "${expected_size}" ]]
  actual_sha="$(sha256sum "${archive}" | cut -d' ' -f1)"
  [[ ${actual_sha} == "${expected_sha}" ]]
  hy2panel_archive_members_safe "${archive}"
  printf '%s\n' "${archive}"
}

install_version() {
  local version=$1 entry archive staging member binary reported target
  target="${VERSIONS}/${version}"
  if [[ -x ${target}/sing-box ]]; then
    [[ ! -L ${target}/sing-box ]]
    [[ $(stat -c '%U:%G:%a' "${target}/sing-box") == root:root:755 ]]
    reported="$("${target}/sing-box" version 2>/dev/null | sed -nE 's/^sing-box version ([0-9]+\.[0-9]+\.[0-9]+).*/\1/p' | head -n 1)"
    [[ ${reported} == "${version}" ]]
    printf '%s\n' "${target}/sing-box"
    return
  fi
  entry="$(manifest_entry "${version}")"
  archive="$(safe_download "${entry}")"
  staging="$(mktemp -d "${VERSIONS}/.${version}.XXXXXX")"
  chmod 755 "${staging}"
  member="sing-box-${version}-linux-amd64/sing-box"
  tar -xzf "${archive}" -C "${staging}" --no-same-owner --no-same-permissions --strip-components=1 -- "${member}"
  binary="${staging}/sing-box"
  [[ -f ${binary} && ! -L ${binary} ]]
  chown root:root "${binary}"
  chmod 755 "${binary}"
  reported="$("${binary}" version 2>/dev/null | sed -nE 's/^sing-box version ([0-9]+\.[0-9]+\.[0-9]+).*/\1/p' | head -n 1)"
  [[ ${reported} == "${version}" ]]
  if [[ -f ${BASE}/sing-box.json ]]; then
    "${binary}" check -c "${BASE}/sing-box.json" >/dev/null
  fi
  if ! mv "${staging}" "${target}" 2>/dev/null; then
    rm -rf "${staging}"
    [[ -x ${target}/sing-box ]]
  fi
  printf '%s\n' "${target}/sing-box"
}

atomic_link() {
  local target=$1 link=$2 temporary
  temporary="${link}.new.${OPERATION_ID}"
  rm -f "${temporary}"
  ln -s "${target}" "${temporary}"
  mv -Tf "${temporary}" "${link}"
}

migrate_legacy() {
  local legacy=/usr/local/bin/sing-box reported digest version destination
  if [[ -L ${ROOT}/current || ! -x ${legacy} ]]; then
    return
  fi
  reported="$("${legacy}" version 2>/dev/null | sed -nE 's/^sing-box version ([0-9]+\.[0-9]+\.[0-9]+).*/\1/p' | head -n 1)"
  digest="$(sha256sum "${legacy}" | cut -d' ' -f1)"
  version="${reported:-legacy-${digest:0:12}}"
  destination="${VERSIONS}/${version}"
  install -d -o root -g root -m 755 "${destination}"
  chmod 755 "${VERSIONS}" "${destination}"
  install -o root -g root -m 755 "${legacy}" "${destination}/sing-box"
  atomic_link "versions/${version}" "${ROOT}/current"
  rm -f "${legacy}"
  ln -s "${ROOT}/current/sing-box" "${legacy}"
}

rollback_current() {
  local old=$1 binary
  if [[ -n ${old} && -x ${VERSIONS}/${old}/sing-box ]]; then
    atomic_link "versions/${old}" "${ROOT}/current"
    binary="${VERSIONS}/${old}/sing-box"
    systemctl restart sing-box.service
    hy2panel_sing_box_healthy "${binary}" "${BASE}/sing-box.json" "${old}" 8
  else
    return 1
  fi
}

prune_versions() {
  local keep_current keep_previous directory name
  keep_current="$(current_version)"
  keep_previous="$(previous_version)"
  for directory in "${VERSIONS}"/*; do
    [[ -d ${directory} ]] || continue
    name="$(basename -- "${directory}")"
    if [[ ${name} != "${keep_current}" && ${name} != "${keep_previous}" ]]; then
      rm -rf "${directory}"
    fi
  done
}

initialize() {
  install -d -o root -g root -m 755 "${ROOT}" "${VERSIONS}"
  chmod 755 "${ROOT}" "${VERSIONS}"
  install -d -o root -g hy2panel -m 750 "$(dirname -- "${STATE_ROOT}")" "${STATE_ROOT}"
  install -d -o root -g root -m 700 "${TRANSACTIONS}"
  verify_manifest
}

run_initial_install() {
  ACTION=install
  TARGET_VERSION=${1:-$(recommended_version)}
  OPERATION_ID="initial-$(date +%s)-$$"
  if ! hy2panel_lock_maintenance; then
    PHASE=preflight
    write_operation failed_no_change '另一个维护任务正在运行'
    exit 1
  fi
  WORK_DIR="$(mktemp -d "${TRANSACTIONS}/sing-box.XXXXXX")"
  migrate_legacy
  local old_current
  old_current="$(current_version)"
  install_version "${TARGET_VERSION}" >/dev/null
  atomic_link "versions/${TARGET_VERSION}" "${ROOT}/current"
  if [[ -n ${old_current} && ${old_current} != "${TARGET_VERSION}" && -x ${VERSIONS}/${old_current}/sing-box ]]; then
    atomic_link "versions/${old_current}" "${ROOT}/previous"
  fi
  rm -f /usr/local/bin/sing-box
  ln -s "${ROOT}/current/sing-box" /usr/local/bin/sing-box
  PHASE=committed
  write_operation succeeded
}

run_request() {
  [[ -f ${REQUEST} ]]
  jq -e 'type == "object" and .schema == 1 and (.id | type == "string" and length >= 16 and length <= 128) and (.action == "switch" or .action == "rollback")' "${REQUEST}" >/dev/null
  OPERATION_ID="$(jq -r '.id' "${REQUEST}")"
  ACTION="$(jq -r '.action' "${REQUEST}")"
  if [[ ${ACTION} == switch ]]; then
    TARGET_VERSION="$(jq -r '.target_version // ""' "${REQUEST}")"
    manifest_entry "${TARGET_VERSION}" >/dev/null
  else
    TARGET_VERSION="$(previous_version)"
    [[ -n ${TARGET_VERSION} ]]
  fi
  rm -f "${RESULT}"
  WORK_DIR="$(mktemp -d "${TRANSACTIONS}/sing-box.XXXXXX")"
  chmod 700 "${WORK_DIR}"
  if ! hy2panel_lock_maintenance; then
    PHASE=preflight
    write_operation failed_no_change '另一个维护任务正在运行'
    exit 1
  fi
  migrate_legacy
  OLD_CURRENT="$(current_version)"
  OLD_PREVIOUS="$(previous_version)"
  if [[ ${TARGET_VERSION} == "${OLD_CURRENT}" ]]; then
    PHASE=committed
    write_operation succeeded '当前已经是所选版本'
    return
  fi
  set_phase downloading '正在下载并校验 sing-box'
  install_version "${TARGET_VERSION}" >/dev/null
  set_phase preflight '正在检查当前配置兼容性'
  "${VERSIONS}/${TARGET_VERSION}/sing-box" check -c "${BASE}/sing-box.json" >/dev/null
  local config_sha
  config_sha="$(sha256sum "${BASE}/sing-box.json" | cut -d' ' -f1)"
  if ! hy2panel_lock_runtime; then
    PHASE=preflight
    write_operation failed_no_change '节点运行状态正在变更'
    exit 1
  fi
  export HY2PANEL_RUNTIME_LOCK_HELD=1
  [[ $(sha256sum "${BASE}/sing-box.json" | cut -d' ' -f1) == "${config_sha}" ]]
  set_phase switching '正在切换 sing-box'
  atomic_link "versions/${TARGET_VERSION}" "${ROOT}/current"
  if systemctl restart sing-box.service && hy2panel_sing_box_healthy "${VERSIONS}/${TARGET_VERSION}/sing-box" "${BASE}/sing-box.json" "${TARGET_VERSION}" 8; then
    if [[ -n ${OLD_CURRENT} ]]; then
      atomic_link "versions/${OLD_CURRENT}" "${ROOT}/previous"
    fi
    PHASE=committed
    write_operation succeeded 'sing-box 切换完成'
    prune_versions
    return
  fi
  PHASE=rolling_back
  write_operation running '新版本未通过健康检查，正在恢复上一版本' true false
  if rollback_current "${OLD_CURRENT}"; then
    if [[ -n ${OLD_PREVIOUS} && -x ${VERSIONS}/${OLD_PREVIOUS}/sing-box ]]; then
      atomic_link "versions/${OLD_PREVIOUS}" "${ROOT}/previous"
    else
      rm -f "${ROOT}/previous"
    fi
    PHASE=rolled_back
    write_operation failed_rolled_back '新版本未通过健康检查，已恢复上一版本' true true
  else
    PHASE=rollback_failed
    write_operation rollback_failed '新版本失败，上一版本也未能恢复，请人工检查' true false
  fi
  exit 1
}

initialize
if [[ ${1:-} == install ]]; then
  run_initial_install "${2:-}"
else
  run_request
fi
