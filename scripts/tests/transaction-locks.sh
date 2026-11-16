#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
LIB="${ROOT_DIR}/scripts/lib/hy2panel-transaction.sh"
TEMP="$(mktemp -d)"
trap 'rm -rf "${TEMP}"' EXIT

cat > "${TEMP}/install" <<'SH'
#!/usr/bin/env bash
mkdir -p "${@: -1}"
SH
cat > "${TEMP}/flock" <<'SH'
#!/usr/bin/env bash
[[ ${HY2PANEL_TEST_FLOCK_FAIL:-0} != 1 ]]
SH
chmod +x "${TEMP}/install" "${TEMP}/flock"

PATH="${TEMP}:${PATH}" HY2PANEL_MAINTENANCE_LOCK="${TEMP}/maintenance.lock" HY2PANEL_TEST_FLOCK_FAIL=1 bash -c '
  source "$1"
  if hy2panel_lock_maintenance; then
    exit 1
  fi
  [[ ${HY2PANEL_MAINTENANCE_LOCK_HELD:-0} != 1 ]]
' bash "${LIB}"

PATH="${TEMP}:${PATH}" HY2PANEL_RUNTIME_LOCK="${TEMP}/runtime.lock" HY2PANEL_TEST_FLOCK_FAIL=1 bash -c '
  source "$1"
  if hy2panel_lock_runtime; then
    exit 1
  fi
  [[ ${HY2PANEL_RUNTIME_LOCK_HELD:-0} != 1 ]]
' bash "${LIB}"

printf 'transaction lock failure propagation: ok\n'
