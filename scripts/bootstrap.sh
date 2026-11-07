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

INTERACTIVE=0
if [[ ${HY2PANEL_NONINTERACTIVE:-0} != 1 && -r /dev/tty && -w /dev/tty ]]; then
  INTERACTIVE=1
fi
PROMPTED=0
MISSING=0

ask_value() {
  local variable=$1
  local prompt=$2
  local default_value=${3:-}
  local value
  if [[ -n ${!variable:-} ]]; then
    return
  fi
  if [[ ${INTERACTIVE} -eq 0 ]]; then
    printf '缺少必填配置 %s。非交互安装请通过环境变量提供。\n' "${variable}" >&2
    MISSING=1
    return
  fi
  if [[ -n ${default_value} ]]; then
    read -r -p "${prompt} [${default_value}]: " value < /dev/tty
  else
    read -r -p "${prompt}: " value < /dev/tty
  fi
  value="${value:-${default_value}}"
  if [[ -z ${value} ]]; then
    printf '%s 不能为空。\n' "${variable}" >&2
    MISSING=1
    return
  fi
  printf -v "${variable}" '%s' "${value}"
  PROMPTED=1
}

ask_optional() {
  local variable=$1
  local prompt=$2
  local default_value=${3:-}
  local value
  if [[ -n ${!variable:-} || ${INTERACTIVE} -eq 0 ]]; then
    return
  fi
  if [[ -n ${default_value} ]]; then
    read -r -p "${prompt} [${default_value}]: " value < /dev/tty
  else
    read -r -p "${prompt}: " value < /dev/tty
  fi
  printf -v "${variable}" '%s' "${value:-${default_value}}"
  PROMPTED=1
}

ask_secret() {
  local variable=$1
  local prompt=$2
  local value
  if [[ -n ${!variable:-} || ${INTERACTIVE} -eq 0 ]]; then
    return
  fi
  read -r -s -p "${prompt}: " value < /dev/tty
  printf '\n' > /dev/tty
  printf -v "${variable}" '%s' "${value}"
  PROMPTED=1
}

DEFAULT_PUBLIC_HOST="${PUBLIC_HOST:-}"
if [[ -z ${DEFAULT_PUBLIC_HOST} ]]; then
  DEFAULT_PUBLIC_HOST="$(hostname -I)"
  DEFAULT_PUBLIC_HOST="${DEFAULT_PUBLIC_HOST%% *}"
fi
DEFAULT_ADMIN_IP="${ADMIN_IP:-}"
if [[ -z ${DEFAULT_ADMIN_IP} && -n ${SSH_CLIENT:-} ]]; then
  DEFAULT_ADMIN_IP="${SSH_CLIENT%% *}"
fi

if [[ ${INTERACTIVE} -eq 1 ]]; then
  printf '\n\033[1;32mhy2panel 交互式安装\033[0m\n'
  printf '直接回车接受默认值；RackNerd 凭据可留空，稍后在面板配置。\n\n'
fi

ask_value PANEL_DOMAIN '面板域名，例如 panel.example.com'
ask_value HY2_DOMAIN 'Hysteria2 证书域名，例如 hy2.example.com'
ask_value PUBLIC_HOST 'Reality 公网地址' "${DEFAULT_PUBLIC_HOST}"
ask_value ADMIN_IP '允许访问面板的公网 IP 或 CIDR' "${DEFAULT_ADMIN_IP}"
ask_optional ACME_EMAIL '证书通知邮箱（可留空）'
ask_secret RACKNERD_API_KEY 'RackNerd API Key（可留空）'
ask_secret RACKNERD_API_HASH 'RackNerd API Hash（可留空）'

if [[ ${MISSING} -ne 0 ]]; then
  exit 1
fi
if [[ -n ${RACKNERD_API_KEY:-} && -z ${RACKNERD_API_HASH:-} || -z ${RACKNERD_API_KEY:-} && -n ${RACKNERD_API_HASH:-} ]]; then
  printf 'RackNerd API Key 和 Hash 必须同时填写，或同时留空。\n' >&2
  exit 1
fi

export PANEL_DOMAIN HY2_DOMAIN PUBLIC_HOST ADMIN_IP ACME_EMAIL RACKNERD_API_KEY RACKNERD_API_HASH

if [[ ${PROMPTED} -eq 1 ]]; then
  printf '\n安装摘要\n面板: https://%s:8443\nReality: %s\nHysteria2: %s\n管理白名单: %s\n' "${PANEL_DOMAIN}" "${PUBLIC_HOST}" "${HY2_DOMAIN}" "${ADMIN_IP}"
  read -r -p '确认开始安装？[Y/n] ' CONFIRM < /dev/tty
  if [[ ${CONFIRM:-Y} =~ ^[Nn]$ ]]; then
    printf '已取消安装。\n'
    exit 0
  fi
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