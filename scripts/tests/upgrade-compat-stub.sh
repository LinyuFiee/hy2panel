#!/usr/bin/env bash
# External boundaries only. No host units, sockets, GitHub API, proxy or listeners.
set -Eeuo pipefail
gate=/opt/gate
[[ -f /.dockerenv && -f $gate/upgrade-compat.sh ]] || exit 2
cmd=${0##*/}
case $cmd in
  curl)
    out='' url=''
    while (($#)); do
      case $1 in -o|--output) shift; out=$1 ;; https://*) url=$1 ;; esac
      shift
    done
    release=https://github.com/LinyuFiee/hy2panel/releases/download/v0.9.0/hy2panel-bundle-linux-amd64.tar.gz
    case $url in
      "$release"|"$release.sha256") [[ -n $out ]]; cp "$gate/${url##*/}" "$out"; exit ;;
      https://panel.example.test:8443/api/health) /usr/local/bin/hy2panel version --json | jq '{status:"ok",version}'; exit ;;
    esac ;;
  systemd-run)
    # Emulate just the synchronous transient-unit command and RuntimeDirectory cleanup.
    while (($#)) && [[ $1 != /bin/bash ]]; do shift; done
    [[ $# == 4 && $3 == --from-v080 && $4 =~ ^/run/hy2panel-v080\.[a-zA-Z0-9]{8}$ && $2 == "$4/hy2panel-upgrade.sh" ]]
    stage=$4
    cmp -s "$2" "$gate/candidate/scripts/hy2panel-upgrade.sh"
    cmp -s "$stage/hy2panel-transaction.sh" "$gate/candidate/scripts/lib/hy2panel-transaction.sh"
    jq -er '.id' "$stage/upgrade.request" > "$gate/staged-id"
    trap 'rm -rf -- "$stage"' EXIT
    cd /; umask 022
    env -i PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin "$@"
    exit ;;
  systemctl)
    case "$*" in
      'show -p ActiveState --value hy2panel-'*.service) printf 'inactive\n'; exit ;;
      'show sing-box.service -p NRestarts --value') printf '0\n'; exit ;;
      'restart hy2panel.service')
        if [[ -f $gate/fault-armed ]]; then
          # Do not count an incidental failure: new binary and config must already be live.
          [[ $(/usr/local/bin/hy2panel version) == 0.9.0 && $(</etc/hy2panel/apply.result) == ok ]]
          if cmp -s /etc/hy2panel/sing-box.json "$gate/core.before"; then exit 2; fi
          mv "$gate/fault-armed" "$gate/fault-fired"
          if [[ $(<"$gate/fault-fired") == recovery ]]; then
            # Only the staged updater, not PID 1, the fixture, or its caller.
            grep -azEq '^/run/hy2panel-v080\.[[:alnum:]]{8}/hy2panel-upgrade\.sh$' "/proc/$PPID/cmdline"
            kill -KILL "$PPID"
          fi
          exit 1
        fi
        exit 0 ;;
      is-*hy2panel-firewall.service) exit 1 ;;
      is-active\ *|is-enabled\ *|start\ *|stop\ *|restart\ *|--no-block\ *|enable\ *|disable\ *|daemon-reload) exit 0 ;;
    esac ;;
  ss) printf 'fixture listener\n'; exit ;;
  nft) [[ $1 != list ]]; exit ;; # No pre-existing firewall table.
  sing-box)
    case $1 in
      version) path=$(readlink -f "$0"); printf 'sing-box version %s\n' "$(basename "${path%/*}")"; exit ;;
      check)
        [[ $# == 3 && $2 == -c ]]
        if [[ $3 == /etc/hy2panel/sing-box.pending.json ]]; then
          [[ ! -L $3 && $(stat -c '%U:%G:%a' "$3") == hy2panel:hy2panel:600 ]]
        fi
        jq -e '.inbounds|length>0' "$3" >/dev/null; exit ;;
    esac ;;
esac
printf 'Unexpected fixture %s invocation\n' "$cmd" >&2
exit 2
