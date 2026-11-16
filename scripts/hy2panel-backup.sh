#!/usr/bin/env bash
set -Eeuo pipefail

DESTINATION=${1:-}
BASE=/etc/hy2panel
BACKUP_ROOT=/root/hy2panel-backups
WORK_DIR=""

source /usr/local/lib/hy2panel/hy2panel-transaction.sh

if [[ ${EUID} -ne 0 ]]; then
  printf '请使用 root 运行。\n' >&2
  exit 1
fi
if ! hy2panel_lock_maintenance; then
  printf '另一个维护操作正在运行。\n' >&2
  exit 1
fi

install -d -o root -g root -m 700 "${BACKUP_ROOT}"
if [[ -z ${DESTINATION} ]]; then
  DESTINATION="${BACKUP_ROOT}/hy2panel-$(date -u +%Y%m%d-%H%M%S).tar.gz"
fi
case ${DESTINATION} in
  /*) ;;
  *) DESTINATION="$(pwd)/${DESTINATION}" ;;
esac
install -d -o root -g root -m 700 "$(dirname -- "${DESTINATION}")"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT
install -d -o root -g root -m 700 "${WORK_DIR}/payload/etc/hy2panel" "${WORK_DIR}/payload/root"

for path in panel.json domains.env 99-hy2panel.conf sysctl.backup; do
  [[ ! -e ${BASE}/${path} ]] || cp -a "${BASE}/${path}" "${WORK_DIR}/payload/etc/hy2panel/"
done
[[ ! -d ${BASE}/certs ]] || cp -a "${BASE}/certs" "${WORK_DIR}/payload/etc/hy2panel/"
[[ ! -f /root/hy2panel-credentials ]] || cp -a /root/hy2panel-credentials "${WORK_DIR}/payload/root/"
[[ ! -f /var/lib/hy2panel/traffic.json ]] || { install -d "${WORK_DIR}/payload/var/lib/hy2panel"; cp -a /var/lib/hy2panel/traffic.json "${WORK_DIR}/payload/var/lib/hy2panel/"; }
[[ ! -f /var/lib/hy2panel-system/installed-files.json ]] || { install -d "${WORK_DIR}/payload/var/lib/hy2panel-system"; cp -a /var/lib/hy2panel-system/installed-files.json "${WORK_DIR}/payload/var/lib/hy2panel-system/"; }

(
  cd "${WORK_DIR}/payload"
  find . -type f -print0 | sort -z | xargs -0 sha256sum > "${WORK_DIR}/checksums.txt"
)
PANEL_VERSION="$(/usr/local/bin/hy2panel version --json | jq -r '.version')"
CONFIG_VERSION="$(jq -r '.version' "${BASE}/panel.json")"
jq -n --arg panel_version "${PANEL_VERSION}" --argjson config_version "${CONFIG_VERSION}" \
  '{backup_format_version:1,panel_version:$panel_version,config_version:$config_version,created_at:(now|todateiso8601)}' > "${WORK_DIR}/manifest.json"
cp "${WORK_DIR}/manifest.json" "${WORK_DIR}/payload/manifest.json"
cp "${WORK_DIR}/checksums.txt" "${WORK_DIR}/payload/checksums.txt"

TEMP_ARCHIVE="$(mktemp "$(dirname -- "${DESTINATION}")/.hy2panel-backup.XXXXXX")"
tar -czf "${TEMP_ARCHIVE}" -C "${WORK_DIR}/payload" .
chmod 600 "${TEMP_ARCHIVE}"
sync -f "${TEMP_ARCHIVE}" 2>/dev/null || true
mv -f "${TEMP_ARCHIVE}" "${DESTINATION}"
printf '备份已创建：%s\n该文件包含节点密码、私钥和 API 凭据，请安全保管。\n' "${DESTINATION}"
