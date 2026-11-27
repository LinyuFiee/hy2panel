#!/bin/bash
# Standalone v0.9.0 release asset. The release workflow pins the finished bundle.
set -euo pipefail
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
umask 077

usage() {
  printf '%s\n' 'Usage: bash upgrade-from-v080.sh --apply' \
    'Only for the published v0.8.0 Debian 12 amd64 installation, upgrading to v0.9.0.' \
    'First verify this release asset, take a restorable VPS backup, and reserve an' \
    'exclusive maintenance window. Do not use the panel or other hy2 commands until done.'
}
die() { printf '%s\n' "$1" >&2; exit 1; }
[[ $# == 1 && $1 == --help ]] && { usage; exit 0; }
[[ $# == 1 && $1 == --apply ]] || { usage >&2; exit 2; }
pin='@BUNDLE_SHA256@'
[[ $pin =~ ^[a-f0-9]{64}$ ]] || die 'Not a generated release asset; nothing started.'
[[ $EUID == 0 ]] || die 'Run the verified release asset as root.'
[[ $(uname -m) == x86_64 ]] && grep -qx 'ID=debian' /etc/os-release && grep -qx 'VERSION_ID="12"' /etc/os-release || die 'Requires Debian 12 amd64.'

base=/etc/hy2panel
active=/var/lib/hy2panel-system/transactions/active
bundle=hy2panel-bundle-linux-amd64.tar.gz
url=https://github.com/LinyuFiee/hy2panel/releases/download/v0.9.0
unit=hy2panel-upgrade-from-v080.service
result=$base/upgrade.result

matches() {
  [[ -f $1 && ! -L $1 && $(stat -c '%u:%g:%a' "$1") == "0:0:$2" ]] &&
    [[ $(sha256sum "$1" | cut -d' ' -f1) == "$3" ]]
}
# Digests of the actual published v0.8.0 amd64 bundle members, not a local rebuild.
baseline() {
  matches /usr/local/bin/hy2panel 755 608ba77d8bee4ef6182dd9b676774dfe2c1fc5aed24a9cfffd294022c9820881 &&
    matches /usr/local/sbin/hy2panel-upgrade 750 d0f28ecb9fdb826095531059f7acddda4417130c6942afbd0506dd226c5844f4 &&
    matches /usr/local/lib/hy2panel/hy2panel-transaction.sh 644 3b4c14089c7295ad75bb754ed6da8406b8a89283418b343d58d028a50530c989
}
base_dir_ok() { [[ -d $base && ! -L $base && $(stat -c '%U:%G:%a' "$base") == root:hy2panel:1770 ]]; }
preflight() {
  local path op
  baseline || die 'Not the stock v0.8.0 binary, updater and transaction library; nothing started.'
  base_dir_ok || die 'Unexpected /etc/hy2panel owner or mode; nothing started.'
  for path in "$base/panel.json" "$base/sing-box.json"; do
    [[ -f $path && ! -L $path && $(stat -c '%U:%G:%a:%h' "$path") == hy2panel:hy2panel:600:1 ]] || die 'Unexpected installed configuration file or permissions.'
  done
  for path in "$base"/*.request "$base"/*.pending.json "$active/snapshot"; do
    [[ ! -e $path && ! -L $path ]] || die 'A request, pending configuration or previous snapshot needs attention.'
  done
  if [[ -e $active || -L $active ]]; then
    [[ -d $active && ! -L $active && -f $active/journal.json && ! -L $active/journal.json ]] &&
      jq -e '.status == "committed" or .status == "rolled_back" or .status == "failed_no_change"' "$active/journal.json" >/dev/null 2>&1 || die 'Previous upgrade is not in a known terminal state.'
  fi
  for op in upgrade recover apply bbr sing-box watchdog; do
    [[ $(systemctl show -p ActiveState --value "hy2panel-$op.service") == inactive ]] || die "hy2panel-$op.service is not idle."
  done
  flock -n /run/lock/hy2panel-maintenance.lock flock -n /run/lock/hy2panel-runtime.lock /bin/true || die 'A maintenance or runtime operation is active.'
}
preflight
stage=$(mktemp -d /run/hy2panel-v080.XXXXXXXX)
trap 'rm -rf -- "$stage"' EXIT
[[ $(stat -c '%u:%g:%a' "$stage") == 0:0:700 ]] || die 'Cannot create root-private staging.'
fetch=(curl --disable -fsSL --proto '=https' --proto-redir '=https' --connect-timeout 20)
"${fetch[@]}" --max-time 60 --max-filesize 1024 "$url/$bundle.sha256" -o "$stage/$bundle.sha256"
cmp -s <(printf '%s  %s\n' "$pin" "$bundle") "$stage/$bundle.sha256" || die 'Published checksum differs from this release asset.'
"${fetch[@]}" --max-time 300 --max-filesize 134217728 "$url/$bundle" -o "$stage/$bundle"
[[ $(sha256sum "$stage/$bundle" | cut -d' ' -f1) == "$pin" ]] || die 'Downloaded bundle differs from the pinned release.'
for member in scripts/hy2panel-upgrade.sh scripts/lib/hy2panel-transaction.sh; do
  tar -xzOf "$stage/$bundle" -- "./$member" > "$stage/${member##*/}"
done
id=$(< /proc/sys/kernel/random/uuid)
jq -n --arg id "$id" '{schema:1,action:"upgrade",target_version:"0.9.0",id:$id}' > "$stage/upgrade.request"
preflight

# No live request or ExecStart substitution. A disconnected client must not remove
# files from under the running service. PID 1 releases RuntimeDirectory on exit.
printf 'Operation %s (%s); keep the maintenance window open.\n' "$id" "$unit" >&2
trap - EXIT
rc=0
systemd-run --wait --collect --quiet --unit="$unit" \
  -p Type=oneshot -p TimeoutStartSec=infinity -p PrivateTmp=yes -p ProtectHome=yes \
  -p "RuntimeDirectory=${stage##*/}" -p RuntimeDirectoryMode=0700 \
  -p StandardOutput=null -p StandardError=null \
  /bin/bash "$stage/hy2panel-upgrade.sh" --from-v080 "$stage" || rc=$?

# Never print service output, panel data or arbitrary result messages. A stale
# result (including a previous success) is not a receipt for this transaction.
unknown='No confirmed terminal result. Keep the window open; inspect the unit and root transaction journal before retrying.'
[[ -f $result && ! -L $result ]] || die "$unknown"
status=$(jq -er --arg id "$id" 'select(.schema == 1 and .id == $id and .target_version == "0.9.0") | .status' "$result" 2>/dev/null) || die "$unknown"
base_dir_ok || die 'Unexpected /etc/hy2panel permissions after the transaction. Inspect before retrying.'
case $status in
  succeeded) [[ $rc == 0 ]] && jq -e --arg id "$id" '.id == $id and .status == "committed"' "$active/journal.json" >/dev/null 2>&1 || die "$unknown"; printf 'succeeded: v0.9.0\n' ;;
  failed_rolled_back) baseline && jq -e --arg id "$id" '.id == $id and .status == "rolled_back"' "$active/journal.json" >/dev/null 2>&1 || die "$unknown"; die 'failed_rolled_back: v0.8.0 restored. Inspect before retrying.' ;;
  failed_no_change) die 'failed_no_change: the updater reported no installation changes.' ;;
  rollback_failed) die 'rollback_failed: manual recovery required; do not retry.' ;;
  *) die "$unknown" ;;
esac
