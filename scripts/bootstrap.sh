#!/usr/bin/env bash
# 兼容入口。安装逻辑已并入 hy2，此处仅获取最新 hy2 并转交 hy2 install。
set -Eeuo pipefail

REPOSITORY="${HY2PANEL_REPOSITORY:-LinyuFiee/hy2panel}"
if [[ ! ${REPOSITORY} =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
  printf 'GitHub 仓库地址无效。\n' >&2
  exit 1
fi

target="$(mktemp)"
curl -fsSL --proto '=https' --proto-redir '=https' --connect-timeout 20 --max-time 120 \
  "https://raw.githubusercontent.com/${REPOSITORY}/main/scripts/hy2" -o "${target}"
bash -n "${target}"

status=0
bash "${target}" install || status=$?
rm -f "${target}"
exit "${status}"
