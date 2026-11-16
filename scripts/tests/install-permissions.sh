#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"

[[ $(grep -F 'install -d -o root -g hy2panel -m 1770 /etc/hy2panel' "${ROOT_DIR}/scripts/install.sh" | wc -l) -eq 1 ]]
[[ $(grep -F 'chmod 755 "${staging}"' "${ROOT_DIR}/scripts/hy2panel-sing-box.sh" | wc -l) -eq 1 ]]
[[ $(grep -F 'ReadWritePaths=/etc/hy2panel /var/lib/hy2panel' "${ROOT_DIR}/systemd/hy2panel.service" | wc -l) -eq 1 ]]
if grep -q '^ReadWritePaths=/etc/hy2panel/.*\.' "${ROOT_DIR}/systemd/hy2panel.service"; then
  printf 'hy2panel.service still references optional individual files\n' >&2
  exit 1
fi

printf 'installation permissions: ok\n'
