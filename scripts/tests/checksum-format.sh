#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
source "${ROOT_DIR}/scripts/lib/hy2panel-transaction.sh"

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT
BUNDLE=hy2panel-bundle-linux-amd64.tar.gz
printf 'checksum fixture\n' > "${WORK_DIR}/${BUNDLE}"
DIGEST="$(sha256sum "${WORK_DIR}/${BUNDLE}" | cut -d' ' -f1)"
CHECKSUM="${WORK_DIR}/${BUNDLE}.sha256"

expect_success() {
  local name=$1 content=$2
  printf '%b' "${content}" > "${CHECKSUM}"
  if ! hy2panel_verify_sha256_file "${CHECKSUM}" "${BUNDLE}" "${WORK_DIR}"; then
    printf 'expected success: %s\n' "${name}" >&2
    exit 1
  fi
}

expect_failure() {
  local name=$1 content=$2
  printf '%b' "${content}" > "${CHECKSUM}"
  if hy2panel_verify_sha256_file "${CHECKSUM}" "${BUNDLE}" "${WORK_DIR}"; then
    printf 'expected failure: %s\n' "${name}" >&2
    exit 1
  fi
}

expect_success text-mode "${DIGEST}  ${BUNDLE}\n"
expect_success binary-mode "${DIGEST} *${BUNDLE}\n"
expect_success crlf "${DIGEST}  ${BUNDLE}\r\n"

expect_failure wrong-digest "$(printf '0%.0s' {1..64})  ${BUNDLE}\n"
expect_failure wrong-name "${DIGEST}  other.tar.gz\n"
expect_failure relative-path "${DIGEST}  ./${BUNDLE}\n"
expect_failure parent-path "${DIGEST}  ../${BUNDLE}\n"
expect_failure absolute-path "${DIGEST}  /tmp/${BUNDLE}\n"
expect_failure extra-line "${DIGEST}  ${BUNDLE}\nextra\n"
expect_failure blank-line "${DIGEST}  ${BUNDLE}\n\n"
expect_failure trailing-data "${DIGEST}  ${BUNDLE}\nextra"
expect_failure tab-marker "${DIGEST}\t${BUNDLE}\n"
expect_failure three-spaces "${DIGEST}   ${BUNDLE}\n"
expect_failure extra-field "${DIGEST}  ${BUNDLE} extra\n"
expect_failure short-digest "${DIGEST:0:63}  ${BUNDLE}\n"
expect_failure uppercase-digest "${DIGEST^^}  ${BUNDLE}\n"
expect_failure non-hex "g${DIGEST:1}  ${BUNDLE}\n"

printf '%s  %s\n' "${DIGEST}" "${BUNDLE}" > "${CHECKSUM}"
[[ $(wc -c < "${CHECKSUM}") -eq $((64 + 2 + ${#BUNDLE} + 1)) ]]

rm -f "${CHECKSUM}"
ln -s "${BUNDLE}" "${CHECKSUM}"
if hy2panel_verify_sha256_file "${CHECKSUM}" "${BUNDLE}" "${WORK_DIR}"; then
  printf 'expected symlink checksum rejection\n' >&2
  exit 1
fi

printf 'checksum format compatibility: ok\n'
