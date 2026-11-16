#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"

[[ $(git -C "${ROOT_DIR}" check-attr eol -- scripts/hy2) == 'scripts/hy2: eol: lf' ]]
if grep -q $'\r' "${ROOT_DIR}/scripts/hy2"; then
  printf 'scripts/hy2 contains CR characters\n' >&2
  exit 1
fi
[[ $(head -n 1 "${ROOT_DIR}/scripts/hy2") == '#!/usr/bin/env bash' ]]

printf 'hy2 line endings: ok\n'
