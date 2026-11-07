#!/usr/bin/env bash
set -Eeuo pipefail

if [[ ${EUID} -ne 0 ]]; then
  printf '请使用 root 运行。\n' >&2
  exit 1
fi

REPOSITORY="${HY2PANEL_REPOSITORY:-LinyuFiee/hy2panel}"
if [[ ! ${REPOSITORY} =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
  printf 'GitHub 仓库地址无效。\n' >&2
  exit 1
fi

source /etc/os-release
if [[ ${ID} != debian || ${VERSION_ID} != 12 ]]; then
  printf '一键安装仅支持 Debian 12。\n' >&2
  exit 1
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y ca-certificates curl tar gzip coreutils

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

RELEASE_PATH=latest/download
if [[ -n ${HY2PANEL_VERSION:-} ]]; then
  if [[ ! ${HY2PANEL_VERSION} =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    printf 'HY2PANEL_VERSION 必须是 vX.Y.Z 格式。\n' >&2
    exit 1
  fi
  RELEASE_PATH="download/${HY2PANEL_VERSION}"
fi

BASE_URL="https://github.com/${REPOSITORY}/releases/${RELEASE_PATH}"
BUNDLE=hy2panel-bundle-linux-amd64.tar.gz
curl -fL --connect-timeout 20 --max-time 300 "${BASE_URL}/${BUNDLE}" -o "${WORK_DIR}/${BUNDLE}"
curl -fL --connect-timeout 20 --max-time 60 "${BASE_URL}/${BUNDLE}.sha256" -o "${WORK_DIR}/${BUNDLE}.sha256"
(
  cd "${WORK_DIR}"
  sha256sum -c "${BUNDLE}.sha256"
)
mkdir "${WORK_DIR}/bundle"
tar -xzf "${WORK_DIR}/${BUNDLE}" -C "${WORK_DIR}/bundle"
bash "${WORK_DIR}/bundle/scripts/install.sh"