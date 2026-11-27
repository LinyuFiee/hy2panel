#!/usr/bin/env bash
# PID 1 in the CI-built Debian image ONLY. No mounts, host units, or network.
set -Eeuo pipefail
if [[ ${GITHUB_ACTIONS:-} != true || ! -f /.dockerenv || $$ != 1 || $EUID != 0 || $PWD != /opt/gate || -e /run/systemd/private || -e /var/run/docker.sock ]]; then
  printf 'Refusing to run outside the isolated CI fixture.\n' >&2
  exit 2
fi
grep -qx 'VERSION_ID="12"' /etc/os-release
grep -qx 'ID=debian' /etc/os-release
# Fail closed if the runner ever adds a workspace, data volume or socket mount.
mounts=$(findmnt -rn -o TARGET)
while IFS= read -r target; do
  case $target in
    /|/proc|/proc/*|/sys|/sys/*|/dev|/dev/*|/etc/hosts|/etc/hostname|/etc/resolv.conf) ;;
    *) printf 'Unexpected container mount: %s\n' "$target" >&2; exit 2 ;;
  esac
done <<< "$mounts"
scenario=${1:?missing scenario}
case $scenario in bare-forward|success|rollback|recovery) ;; *) exit 2 ;; esac
base=/etc/hy2panel
data=/var/lib/hy2panel
active=/var/lib/hy2panel-system/transactions/active
panel=/usr/local/bin/hy2panel
bundle=hy2panel-bundle-linux-amd64.tar.gz
fail() { printf 'upgrade gate [%s]: %s\n' "$scenario" "$*" >&2; exit 1; }
trap 'printf "upgrade gate [%s] failed at line %s\n" "$scenario" "$LINENO" >&2; if [[ -f $active/journal.json ]]; then jq "{status,phase}" "$active/journal.json" >&2; fi' ERR
as_panel() { setpriv --reuid=hy2panel --regid=hy2panel --clear-groups -- "$@"; }
metadata() {
  local actual
  actual=$(stat -c '%U:%G:%a' -- "$1")
  [[ ! -L $1 && $actual == "$2" ]] || fail "$1: expected $2, got $actual"
}

# Recreate the v0.8.0 install footprint, not certbot, sysctl, swap or live services.
useradd --system --user-group --home-dir "$data" --shell /usr/sbin/nologin hy2panel
install -d -o root -g hy2panel -m 1770 "$base"
install -d -o root -g hy2panel -m 750 "$base/certs"
install -d -o hy2panel -g hy2panel -m 700 "$data"
install -d /etc/systemd/system /usr/local/lib/hy2panel /usr/local/share/hy2panel
install -m 755 base/dist/hy2panel-linux-amd64 "$panel"
install -m 755 base/scripts/hy2 /usr/local/bin/hy2
for f in base/scripts/hy2panel-*.sh; do
  name=${f##*/}; install -m 750 "$f" "/usr/local/sbin/${name%.sh}"
done
install -m 750 base/scripts/cert-deploy.sh /usr/local/sbin/hy2panel-cert-deploy
install -Dm 750 base/scripts/cert-deploy.sh /etc/letsencrypt/renewal-hooks/deploy/hy2panel
install -m 644 base/scripts/lib/*.sh /usr/local/lib/hy2panel/
install -m 644 base/scripts/99-hy2panel.conf "$base/"
install -m 644 base/packaging/sing-box-versions.json /usr/local/share/hy2panel/
install -m 644 base/systemd/* /etc/systemd/system/
[[ $("$panel" version) == 0.8.0 && $(candidate/dist/hy2panel-linux-amd64 version) == 0.9.0 ]]

# Only these external boundaries are fake. Nothing shims install, mv, chown,
# chmod, the panel binaries, transaction locks, updater, apply, or recovery.
for cmd in curl systemctl systemd-run ss nft; do install -m 755 upgrade-compat-stub.sh "/usr/local/bin/$cmd"; done
for manifest in {base,candidate}/packaging/sing-box-versions.json; do
  core=$(jq -er '.versions[] | select(.status=="recommended").version' "$manifest")
  install -Dm 755 upgrade-compat-stub.sh "/usr/local/lib/hy2panel/sing-box/versions/$core/sing-box"
done
/usr/local/sbin/hy2panel-sing-box install
printf '%s' 'fixture-only-password' | as_panel env RACKNERD_API_KEY=gate-key RACKNERD_API_HASH=gate-hash "$panel" init \
  --panel-domain panel.example.test --hy2-domain hy2.example.test --public-host 192.0.2.123
as_panel "$panel" render --output "$base/sing-box.pending.json"
as_panel sh -c 'umask 077; printf "apply\n" > /etc/hy2panel/apply.request'
/usr/local/sbin/hy2panel-apply
[[ $(<"$base/apply.result") == ok ]]; rm "$base/apply.result"
# Valid JSON with intentionally different bytes: a post-apply rollback must restore it.
as_panel sh -c 'printf "\n" >> /etc/hy2panel/sing-box.json'
cp "$base/panel.json" panel.before
cp "$base/sing-box.json" core.before
printf 'PANEL_DOMAIN=panel.example.test\n' > "$base/domains.env"; chmod 600 "$base/domains.env"
printf 'fixture TLS key sentinel\n' | install -o root -g hy2panel -m 640 /dev/stdin "$base/certs/panel.key"
printf '%s\n' '{"schema_version":1,"initialized":true,"global":{"download":1234567}}' |
  install -o hy2panel -g hy2panel -m 600 /dev/stdin "$data/traffic.json"
sha256sum "$base/domains.env" "$base/certs/panel.key" "$data/traffic.json" > preserved.sha256

tar -czf "$bundle" -C candidate .
sha256sum "$bundle" > "$bundle.sha256"
read -r digest _ < "$bundle.sha256"
sed "s/@BUNDLE_SHA256@/$digest/" upgrade-from-v080.sh > bridge.sh
if [[ $scenario == success ]]; then
  # These must fail in the public wrapper, before launching a root transaction.
  reject_bridge() {
    if bash bridge.sh --apply > bridge.log 2>&1; then fail "bridge accepted $1"; fi
    grep -Fq "$2" bridge.log || fail "unexpected rejection for $1"
    [[ ! -e staged-id && ! -e $active && ! -e $base/upgrade.result ]] || fail "transaction started for $1"
  }
  as_panel touch "$base/apply.request"
  reject_bridge 'busy request' 'request, pending configuration'
  rm "$base/apply.request"
  printf '%064d  %s\n' 0 "$bundle" > "$bundle.sha256"
  reject_bridge 'wrong sidecar' 'Published checksum differs'
  sha256sum "$bundle" > "$bundle.sha256"
  size=$(stat -c %s "$bundle"); printf 'x' >> "$bundle"
  reject_bridge 'wrong bundle digest' 'Downloaded bundle differs'
  truncate -s "$size" "$bundle"
  sha256sum --check --status "$bundle.sha256"
fi
rc=0
case $scenario in
  bare-forward)
    # Only forward success is supported here: the immutable old rollback chmods the parent.
    as_panel sh -c 'umask 077; printf "%s\n" "$1" > /etc/hy2panel/upgrade.request' sh \
      '{"schema":1,"action":"upgrade","id":"gate-bare-forward-080","target_version":"0.9.0"}'
    /usr/local/sbin/hy2panel-upgrade ;;
  *)
    case $scenario in rollback|recovery) printf '%s\n' "$scenario" > fault-armed ;; esac
    bash bridge.sh --apply > bridge.log 2>&1 || rc=$?
    [[ -f staged-id ]] || fail 'public bridge rejected v0.8.0 before launching the staged transaction'
    if [[ $scenario == rollback ]]; then grep -q '^failed_rolled_back:' bridge.log || fail 'bridge did not confirm rollback'; fi
    ;;
esac

expected=candidate
if [[ $scenario == rollback || $scenario == recovery ]]; then
  [[ $rc != 0 && -f fault-fired ]] || fail 'expected failure after candidate apply, not during preflight'
  phase=rolled_back
  if [[ $scenario == recovery ]]; then
    jq -e '.status=="running" and .phase=="restarting_panel"' "$active/journal.json" >/dev/null
    cmp -s candidate/scripts/hy2panel-recover.sh /usr/local/sbin/hy2panel-recover
    /usr/local/sbin/hy2panel-recover
    phase=recovery
  fi
  jq -e --arg phase "$phase" '.status=="failed_rolled_back" and .phase==$phase and .rollback=={attempted:true,succeeded:true}' "$base/upgrade.result" >/dev/null
  jq -e --arg phase "$phase" '.status=="rolled_back" and .phase==$phase' "$active/journal.json" >/dev/null
  cmp -s panel.before "$base/panel.json" || fail 'rollback changed panel config bytes'
  cmp -s core.before "$base/sing-box.json" || fail 'rollback did not restore the old applied config'
  expected=base
else
  [[ $rc == 0 && $(<"$base/apply.result") == ok && ! -e $base/sing-box.pending.json ]]
  jq -e '.status=="succeeded" and .phase=="committed" and .rollback=={attempted:false,succeeded:false}' "$base/upgrade.result" >/dev/null
  jq -e '.status=="committed" and .phase=="committed"' "$active/journal.json" >/dev/null
  # Every old config value (including generated keys, password hash, ports and
  # RackNerd settings) survives; allow new fields and non-semantic formatting.
  jq -e --slurpfile old panel.before '. as $new | $old[0] as $base | all($base|paths(scalars); . as $p | ($new|getpath($p))==($base|getpath($p)))' "$base/panel.json" >/dev/null
  as_panel "$panel" render --output "$base/.expected-core"
  cmp -s "$base/.expected-core" "$base/sing-box.json" || fail 'active core config differs from candidate render'
fi
id=gate-bare-forward-080; [[ $scenario == bare-forward ]] || id=$(<staged-id)
jq -e --arg id "$id" '.id==$id and .target_version=="0.9.0"' "$base/upgrade.result" >/dev/null
cmp -s "$expected/dist/hy2panel-linux-amd64" "$panel" || fail 'incorrect installed panel binary'
for name in upgrade apply recover; do cmp -s "$expected/scripts/hy2panel-$name.sh" "/usr/local/sbin/hy2panel-$name" || fail "incorrect installed $name"; done
sha256sum --check --status preserved.sha256 || fail 'certificate, domains or traffic sentinel changed'
cmp -s "$expected/scripts/lib/hy2panel-transaction.sh" /usr/local/lib/hy2panel/hy2panel-transaction.sh || fail 'incorrect transaction library'
metadata "$panel" root:root:755
metadata /usr/local/sbin/hy2panel-upgrade root:root:750
metadata "${active%/*}" root:root:700
metadata "$active" root:root:700
metadata "$base" root:hy2panel:1770
metadata "$base/certs" root:hy2panel:750
metadata "$base/certs/panel.key" root:hy2panel:640
metadata "$base/domains.env" root:root:600
metadata "$base/99-hy2panel.conf" root:root:644
metadata "$data" hy2panel:hy2panel:700
for f in "$base/"{panel.json,sing-box.json,upgrade.result} "$data/traffic.json"; do metadata "$f" hy2panel:hy2panel:600; done
as_panel "$panel" check
for dir in "$base" "$data"; do
  as_panel sh -ec 'umask 077; p=$(mktemp "$1/.write.XXXXXX"); printf pending > "$p"; mv "$p" "$1/write.probe"; rm "$1/write.probe"' sh "$dir"
done
printf 'upgrade gate [%s]: ok\n' "$scenario"
