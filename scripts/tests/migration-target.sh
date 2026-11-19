#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="${ROOT_DIR}/scripts/migrate-v04-to-v05.sh"

source "${SCRIPT}"

hy2panel_valid_migration_target "${TARGET_VERSION}"
hy2panel_valid_migration_target v0.5.2
hy2panel_valid_migration_target v0.6.1
hy2panel_valid_migration_target v0.12.34

for invalid in v1.0.0 0.6.1 v0.6 v0.6.1-rc1 v0.6.1/extra; do
  if hy2panel_valid_migration_target "${invalid}"; then
    printf 'expected invalid migration target: %s\n' "${invalid}" >&2
    exit 1
  fi
done

copy="$(mktemp)"
trap 'rm -f "${copy}"' EXIT
cp "${SCRIPT}" "${copy}"
sed -i 's/^TARGET_VERSION=.*/TARGET_VERSION=v0.9.8/' "${copy}"
source "${copy}"
[[ ${TARGET_VERSION} == v0.9.8 ]]
hy2panel_valid_migration_target "${TARGET_VERSION}"

printf 'migration target validation: ok\n'
