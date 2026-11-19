#!/usr/bin/env bash

TARGET_VERSION=v0.6.1

hy2panel_valid_migration_target() {
  local version=${1:-}
  [[ ${version} =~ ^v0\.[0-9]+\.[0-9]+$ ]]
}

if [[ ${BASH_SOURCE[0]} != "$0" ]]; then
  return 0
fi

set -Eeuo pipefail

REPOSITORY=${HY2PANEL_REPOSITORY:-LinyuFiee/hy2panel}
hy2panel_valid_migration_target "${TARGET_VERSION}" || { printf '迁移器目标版本无效。\n' >&2; exit 1; }

if [[ ${EUID} -ne 0 ]]; then
  printf '请使用 root 运行。\n' >&2
  exit 1
fi
[[ -f /etc/hy2panel/panel.json ]] || { printf '未检测到现有 hy2panel 配置。\n' >&2; exit 1; }

verify_sha256_file() {
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

BUNDLE=hy2panel-bundle-linux-amd64.tar.gz
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT
BASE_URL="https://github.com/${REPOSITORY}/releases/download/${TARGET_VERSION}"
curl -fL --proto '=https' --proto-redir '=https' --connect-timeout 20 --max-time 300 "${BASE_URL}/${BUNDLE}" -o "${WORK_DIR}/${BUNDLE}"
curl -fL --proto '=https' --proto-redir '=https' --connect-timeout 20 --max-time 60 "${BASE_URL}/${BUNDLE}.sha256" -o "${WORK_DIR}/${BUNDLE}.sha256"
verify_sha256_file "${WORK_DIR}/${BUNDLE}.sha256" "${BUNDLE}" "${WORK_DIR}"
mkdir "${WORK_DIR}/bundle"
tar -xzf "${WORK_DIR}/${BUNDLE}" -C "${WORK_DIR}/bundle" --no-same-owner --no-same-permissions

BUNDLE_ROOT="${WORK_DIR}/bundle"
install -d -o root -g root -m 755 /usr/local/lib/hy2panel /usr/local/share/hy2panel
install -d -o root -g hy2panel -m 750 /var/lib/hy2panel-system /var/lib/hy2panel-system/sing-box
install -d -o root -g root -m 700 /var/lib/hy2panel-system/transactions
install -o root -g root -m 644 "${BUNDLE_ROOT}/scripts/lib/hy2panel-transaction.sh" /usr/local/lib/hy2panel/hy2panel-transaction.sh
install -o root -g root -m 644 "${BUNDLE_ROOT}/packaging/sing-box-versions.json" /usr/local/share/hy2panel/sing-box-versions.json
install -o root -g root -m 750 "${BUNDLE_ROOT}/scripts/hy2panel-upgrade.sh" /usr/local/sbin/hy2panel-upgrade
install -o root -g root -m 750 "${BUNDLE_ROOT}/scripts/hy2panel-recover.sh" /usr/local/sbin/hy2panel-recover
install -o root -g root -m 750 "${BUNDLE_ROOT}/scripts/hy2panel-sing-box.sh" /usr/local/sbin/hy2panel-sing-box
install -o root -g root -m 644 "${BUNDLE_ROOT}/systemd/hy2panel-recover.service" /etc/systemd/system/hy2panel-recover.service
systemctl daemon-reload

REQUEST="$(mktemp /etc/hy2panel/.upgrade-request.XXXXXX)"
jq -n --arg id "$(openssl rand -hex 16)" --arg target "${TARGET_VERSION#v}" '{schema:1,id:$id,action:"upgrade",target_version:$target}' > "${REQUEST}"
chown hy2panel:hy2panel "${REQUEST}"
chmod 600 "${REQUEST}"
mv -f "${REQUEST}" /etc/hy2panel/upgrade.request
/usr/local/sbin/hy2panel-upgrade

printf 'v0.4.x → %s 迁移完成。请执行 hy2 status 与 hy2 check。\n' "${TARGET_VERSION}"
