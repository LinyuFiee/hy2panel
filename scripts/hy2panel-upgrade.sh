#!/usr/bin/env bash
set -Eeuo pipefail

BASE=/etc/hy2panel
REQUEST="${BASE}/upgrade.request"
RESULT="${BASE}/upgrade.result"
STATE_ROOT=/var/lib/hy2panel-system
TRANSACTIONS="${STATE_ROOT}/transactions"
ACTIVE="${TRANSACTIONS}/active"
REPOSITORY=LinyuFiee/hy2panel
BUNDLE=hy2panel-bundle-linux-amd64.tar.gz
WORK_DIR=""
OPERATION_ID=""
TARGET_VERSION=""
PHASE=requested
SNAPSHOT_READY=0
ROLLING_BACK=0

source /usr/local/lib/hy2panel/hy2panel-transaction.sh

managed_paths=(
  /usr/local/bin/hy2panel
  /usr/local/bin/hy2
  /usr/local/sbin/hy2panel-apply
  /usr/local/sbin/hy2panel-bbr
  /usr/local/sbin/hy2panel-watchdog
  /usr/local/sbin/hy2panel-upgrade
  /usr/local/sbin/hy2panel-recover
  /usr/local/sbin/hy2panel-sing-box
  /usr/local/sbin/hy2panel-backup
  /usr/local/sbin/hy2panel-restore
  /usr/local/sbin/hy2panel-uninstall
  /usr/local/sbin/hy2panel-cert-deploy
  /usr/local/lib/hy2panel/hy2panel-transaction.sh
  /usr/local/share/hy2panel/sing-box-versions.json
  /etc/letsencrypt/renewal-hooks/deploy/hy2panel
  /etc/hy2panel/99-hy2panel.conf
  /etc/hy2panel/panel.json
  /etc/hy2panel/sing-box.json
  /etc/hy2panel/sing-box.previous.json
  /etc/hy2panel/firewall.nft
  /usr/local/lib/hy2panel/sing-box
  /usr/local/bin/sing-box
  /var/lib/hy2panel-system/sing-box
  /etc/systemd/system/hy2panel.service
  /etc/systemd/system/sing-box.service
  /etc/systemd/system/hy2panel-apply.service
  /etc/systemd/system/hy2panel-apply.path
  /etc/systemd/system/hy2panel-bbr.service
  /etc/systemd/system/hy2panel-bbr.path
  /etc/systemd/system/hy2panel-watchdog.service
  /etc/systemd/system/hy2panel-watchdog.timer
  /etc/systemd/system/hy2panel-upgrade.service
  /etc/systemd/system/hy2panel-upgrade.path
  /etc/systemd/system/hy2panel-sing-box.service
  /etc/systemd/system/hy2panel-sing-box.path
  /etc/systemd/system/hy2panel-recover.service
  /etc/systemd/system/hy2panel-firewall.service
)
managed_units=(
  hy2panel.service
  sing-box.service
  hy2panel-apply.path
  hy2panel-bbr.path
  hy2panel-watchdog.timer
  hy2panel-upgrade.path
  hy2panel-sing-box.path
  hy2panel-recover.service
  hy2panel-firewall.service
)

atomic_json_result() {
  local status=$1 message=$2 rollback_attempted=${3:-false} rollback_succeeded=${4:-false}
  local temporary
  temporary="$(mktemp "${BASE}/.upgrade-result.XXXXXX")"
  jq -n --arg id "${OPERATION_ID}" --arg status "${status}" --arg phase "${PHASE}" --arg target "${TARGET_VERSION}" --arg message "${message}" --argjson attempted "${rollback_attempted}" --argjson restored "${rollback_succeeded}" \
    '{schema:1,id:$id,status:$status,phase:$phase,target_version:$target,message:$message,rollback:{attempted:$attempted,succeeded:$restored}}' > "${temporary}"
  chown hy2panel:hy2panel "${temporary}"
  chmod 600 "${temporary}"
  sync -f "${temporary}" 2>/dev/null || true
  mv -f "${temporary}" "${RESULT}"
}

write_journal() {
  local status=$1 message=${2:-}
  local temporary
  temporary="$(mktemp "${ACTIVE}/.journal.XXXXXX")"
  jq -n --arg id "${OPERATION_ID}" --arg target "${TARGET_VERSION}" --arg status "${status}" --arg phase "${PHASE}" --arg message "${message}" \
    '{schema:1,id:$id,target_version:$target,status:$status,phase:$phase,message:$message,updated_at:(now|todateiso8601)}' > "${temporary}"
  chmod 600 "${temporary}"
  sync -f "${temporary}" 2>/dev/null || true
  mv -f "${temporary}" "${ACTIVE}/journal.json"
}

set_phase() {
  PHASE=$1
  write_journal running "${2:-}"
  atomic_json_result running "${2:-}"
}

record_snapshot() {
  local snapshot="${ACTIVE}/snapshot" path encoded exists=false mode="" owner="" group=""
  rm -rf "${snapshot}"
  install -d -o root -g root -m 700 "${snapshot}/files"
  printf '[]\n' > "${snapshot}/managed.json"
  for path in "${managed_paths[@]}"; do
    encoded="$(printf '%s' "${path}" | sha256sum | cut -d' ' -f1)"
    exists=false mode="" owner="" group=""
    if [[ -e ${path} || -L ${path} ]]; then
      exists=true
      cp -a -- "${path}" "${snapshot}/files/${encoded}"
      if [[ -L ${path} ]]; then
        mode=""
      else
        mode="$(stat -c '%a' "${path}")"
      fi
      owner="$(stat -c '%U' "${path}")"
      group="$(stat -c '%G' "${path}")"
    fi
    jq --arg path "${path}" --arg backup "${encoded}" --arg mode "${mode}" --arg owner "${owner}" --arg group "${group}" --argjson exists "${exists}" \
      '. + [{path:$path,exists:$exists,backup:$backup,mode:$mode,owner:$owner,group:$group}]' "${snapshot}/managed.json" > "${snapshot}/managed.tmp"
    mv "${snapshot}/managed.tmp" "${snapshot}/managed.json"
  done
  printf '[]\n' > "${snapshot}/units.json"
  local unit enabled active
  for unit in "${managed_units[@]}"; do
    enabled=false active=false
    systemctl is-enabled --quiet "${unit}" 2>/dev/null && enabled=true
    systemctl is-active --quiet "${unit}" 2>/dev/null && active=true
    jq --arg unit "${unit}" --argjson enabled "${enabled}" --argjson active "${active}" '. + [{unit:$unit,enabled:$enabled,active:$active}]' "${snapshot}/units.json" > "${snapshot}/units.tmp"
    mv "${snapshot}/units.tmp" "${snapshot}/units.json"
  done
  if nft list table inet hy2panel > "${snapshot}/firewall.nft" 2>/dev/null; then
    printf 'true\n' > "${snapshot}/firewall-active"
  else
    printf 'false\n' > "${snapshot}/firewall-active"
  fi
  sync -f "${snapshot}/managed.json" 2>/dev/null || true
  SNAPSHOT_READY=1
}

restore_snapshot() {
  local snapshot="${ACTIVE}/snapshot" record path exists backup mode owner group failed=0 unit enabled active
  ROLLING_BACK=1
  export HY2PANEL_MAINTENANCE_LOCK_HELD=1 HY2PANEL_RUNTIME_LOCK_HELD=1
  systemctl stop hy2panel.service >/dev/null 2>&1 || true
  while IFS= read -r record; do
    path="$(jq -r '.path' <<<"${record}")"
    exists="$(jq -r '.exists' <<<"${record}")"
    backup="$(jq -r '.backup' <<<"${record}")"
    mode="$(jq -r '.mode' <<<"${record}")"
    owner="$(jq -r '.owner' <<<"${record}")"
    group="$(jq -r '.group' <<<"${record}")"
    if [[ ${exists} == true ]]; then
      install -d -o root -g root -m 755 "$(dirname -- "${path}")"
      rm -rf -- "${path}"
      cp -a -- "${snapshot}/files/${backup}" "${path}" || failed=1
      if [[ -n ${mode} && ! -L ${path} ]]; then
        chmod "${mode}" "${path}" || failed=1
      fi
      [[ -z ${owner} || -z ${group} ]] || chown -h "${owner}:${group}" "${path}" || failed=1
    else
      rm -rf -- "${path}" || failed=1
    fi
  done < <(jq -c '.[]' "${snapshot}/managed.json")
  systemctl daemon-reload || failed=1
  while IFS= read -r record; do
    unit="$(jq -r '.unit' <<<"${record}")"
    enabled="$(jq -r '.enabled' <<<"${record}")"
    active="$(jq -r '.active' <<<"${record}")"
    if [[ ${enabled} == true ]]; then
      systemctl enable "${unit}" >/dev/null 2>&1 || failed=1
    else
      systemctl disable "${unit}" >/dev/null 2>&1 || true
    fi
    if [[ ${active} == true ]]; then
      systemctl restart "${unit}" >/dev/null 2>&1 || failed=1
    else
      systemctl stop "${unit}" >/dev/null 2>&1 || true
    fi
  done < <(jq -c '.[]' "${snapshot}/units.json")
  if [[ $(< "${snapshot}/firewall-active") == true ]]; then
    nft -f "${snapshot}/firewall.nft" >/dev/null 2>&1 || failed=1
  else
    nft delete table inet hy2panel >/dev/null 2>&1 || true
  fi
  ROLLING_BACK=0
  return "${failed}"
}

rollback_on_error() {
  local exit_code=$?
  trap - ERR INT TERM HUP
  if [[ ${ROLLING_BACK} -eq 1 ]]; then
    exit "${exit_code}"
  fi
  if [[ ${SNAPSHOT_READY} -eq 1 ]]; then
    PHASE=rolling_back
    write_journal rolling_back '升级失败，正在恢复上一版本'
    if restore_snapshot; then
      PHASE=rolled_back
      write_journal rolled_back '升级失败，已恢复上一版本'
      atomic_json_result failed_rolled_back '升级失败，已恢复上一版本' true true
    else
      PHASE=rollback_failed
      write_journal rollback_failed '升级失败且自动恢复未完成，请人工检查'
      atomic_json_result rollback_failed '升级失败且自动恢复未完成，请人工检查' true false
    fi
  else
    PHASE=failed_no_change
    [[ -d ${ACTIVE} ]] && write_journal failed_no_change '升级预检失败，现有系统未修改'
    atomic_json_result failed_no_change '升级预检失败，现有系统未修改' false false
  fi
  exit "${exit_code}"
}
trap rollback_on_error ERR INT TERM HUP

cleanup() {
  rm -f "${REQUEST}"
  if [[ -n ${WORK_DIR} && -d ${WORK_DIR} ]]; then
    rm -rf "${WORK_DIR}"
  fi
}
trap cleanup EXIT

parse_request() {
  jq -e 'type == "object" and .schema == 1 and .action == "upgrade" and (.id | type == "string" and length >= 16 and length <= 128) and (.target_version | test("^[0-9]+\\.[0-9]+\\.[0-9]+$"))' "${REQUEST}" >/dev/null
  OPERATION_ID="$(jq -r '.id' "${REQUEST}")"
  TARGET_VERSION="$(jq -r '.target_version' "${REQUEST}")"
}

verify_bundle() {
  local base_url="https://github.com/${REPOSITORY}/releases/download/v${TARGET_VERSION}"
  curl -fL --proto '=https' --proto-redir '=https' --connect-timeout 20 --max-time 300 "${base_url}/${BUNDLE}" -o "${WORK_DIR}/${BUNDLE}"
  curl -fL --proto '=https' --proto-redir '=https' --connect-timeout 20 --max-time 60 "${base_url}/${BUNDLE}.sha256" -o "${WORK_DIR}/${BUNDLE}.sha256"
  hy2panel_verify_sha256_file "${WORK_DIR}/${BUNDLE}.sha256" "${BUNDLE}" "${WORK_DIR}"
  hy2panel_archive_members_safe "${WORK_DIR}/${BUNDLE}"
  mkdir "${WORK_DIR}/bundle"
  tar -xzf "${WORK_DIR}/${BUNDLE}" -C "${WORK_DIR}/bundle" --no-same-owner --no-same-permissions
  "${WORK_DIR}/bundle/dist/hy2panel-linux-amd64" version --json | jq -e --arg version "${TARGET_VERSION}" '.version == $version' >/dev/null
  "${WORK_DIR}/bundle/dist/hy2panel-linux-amd64" check --config "${BASE}/panel.json" >/dev/null
}

install_file() {
  local source=$1 target=$2 mode=$3
  install -d -o root -g root -m 755 "$(dirname -- "${target}")"
  local temporary="${target}.new.${OPERATION_ID}"
  install -o root -g root -m "${mode}" "${source}" "${temporary}"
  sync -f "${temporary}" 2>/dev/null || true
  mv -f "${temporary}" "${target}"
}

install_bundle() {
  local bundle="${WORK_DIR}/bundle"
  install_file "${bundle}/dist/hy2panel-linux-amd64" /usr/local/bin/hy2panel 755
  install_file "${bundle}/scripts/hy2" /usr/local/bin/hy2 755
  for name in hy2panel-apply hy2panel-bbr hy2panel-watchdog hy2panel-upgrade hy2panel-recover hy2panel-sing-box hy2panel-backup hy2panel-restore hy2panel-uninstall; do
    install_file "${bundle}/scripts/${name}.sh" "/usr/local/sbin/${name}" 750
  done
  install_file "${bundle}/scripts/cert-deploy.sh" /usr/local/sbin/hy2panel-cert-deploy 750
  install_file "${bundle}/scripts/cert-deploy.sh" /etc/letsencrypt/renewal-hooks/deploy/hy2panel 750
  install_file "${bundle}/scripts/99-hy2panel.conf" /etc/hy2panel/99-hy2panel.conf 644
  install_file "${bundle}/scripts/lib/hy2panel-transaction.sh" /usr/local/lib/hy2panel/hy2panel-transaction.sh 644
  install_file "${bundle}/packaging/sing-box-versions.json" /usr/local/share/hy2panel/sing-box-versions.json 644
  for unit_file in "${bundle}"/systemd/*; do
    install_file "${unit_file}" "/etc/systemd/system/$(basename -- "${unit_file}")" 644
  done
  rm -f /etc/systemd/system/hy2panel-firewall.service
  systemctl daemon-reload
}

remove_legacy_firewall() {
  systemctl disable --now hy2panel-firewall.service >/dev/null 2>&1 || true
  nft delete table inet hy2panel >/dev/null 2>&1 || true
  rm -f /etc/hy2panel/firewall.nft /etc/systemd/system/hy2panel-firewall.service
  systemctl daemon-reload
}

[[ -f ${REQUEST} && -f ${BASE}/panel.json ]]
parse_request
if ! hy2panel_version_at_least "${TARGET_VERSION}"; then
  atomic_json_result failed_no_change "不再支持 ${TARGET_VERSION}：面板最低可升级版本为 $(hy2panel_min_panel_version)"
  exit 1
fi
install -d -o root -g root -m 700 "${STATE_ROOT}" "${TRANSACTIONS}"
if ! hy2panel_lock_maintenance; then
  atomic_json_result failed_no_change '另一个维护任务正在运行' false false
  exit 1
fi
rm -f "${RESULT}"
WORK_DIR="$(mktemp -d "${TRANSACTIONS}/upgrade.XXXXXX")"
chmod 700 "${WORK_DIR}"
rm -rf "${ACTIVE}"
mkdir "${ACTIVE}"
chmod 700 "${ACTIVE}"
write_journal running

set_phase downloading '正在下载目标版本'
verify_bundle
set_phase snapshotting '正在创建升级快照'
record_snapshot
set_phase installing '正在原子安装新版本'
install_bundle
export HY2PANEL_MAINTENANCE_LOCK_HELD=1
/usr/local/sbin/hy2panel-sing-box install "${SING_BOX_VERSION:-$(jq -r '.versions[] | select(.status == "recommended") | .version' /usr/local/share/hy2panel/sing-box-versions.json)}"
set_phase migrating '正在迁移兼容配置'
systemctl stop hy2panel.service
/usr/local/bin/hy2panel migrate --config "${BASE}/panel.json"
/usr/local/bin/hy2panel render --config "${BASE}/panel.json" --output "${BASE}/sing-box.pending.json"
if ! hy2panel_lock_runtime; then
  false
fi
export HY2PANEL_RUNTIME_LOCK_HELD=1
set_phase restarting_core '正在应用代理配置'
systemctl stop hy2panel-apply.path >/dev/null 2>&1 || true
hy2panel_atomic_text "${BASE}/apply.request" hy2panel hy2panel 600 apply
/usr/local/sbin/hy2panel-apply
remove_legacy_firewall
systemctl enable hy2panel-recover.service hy2panel-apply.path hy2panel-bbr.path hy2panel-watchdog.timer hy2panel-upgrade.path hy2panel-sing-box.path >/dev/null
systemctl restart hy2panel-apply.path hy2panel-bbr.path hy2panel-watchdog.timer hy2panel-sing-box.path
systemctl start hy2panel-upgrade.path
set_phase restarting_panel '正在启动新面板'
systemctl restart hy2panel.service
set_phase health_check '正在验证新版本'
hy2panel_panel_healthy "${TARGET_VERSION}"
BINARY="$(hy2panel_sing_box_binary)"
hy2panel_sing_box_healthy "${BINARY}" "${BASE}/sing-box.json" "" 8
PHASE=committed
write_journal committed '升级完成'
atomic_json_result succeeded '升级完成' false false
