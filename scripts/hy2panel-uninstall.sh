#!/usr/bin/env bash
set -Eeuo pipefail

PURGE=0
if [[ ${1:-} == --purge-data ]]; then
  PURGE=1
elif [[ -n ${1:-} ]]; then
  printf '用法: hy2 uninstall [--purge-data]\n' >&2
  exit 2
fi
if [[ ${EUID} -ne 0 ]]; then
  printf '请使用 root 运行。\n' >&2
  exit 1
fi

source /usr/local/lib/hy2panel/hy2panel-transaction.sh
if ! hy2panel_lock_maintenance || ! hy2panel_lock_runtime; then
  printf '另一个维护操作正在运行。\n' >&2
  exit 1
fi

BACKUP="/root/hy2panel-backups/pre-uninstall-$(date -u +%Y%m%d-%H%M%S).tar.gz"
export HY2PANEL_MAINTENANCE_LOCK_HELD=1
/usr/local/sbin/hy2panel-backup "${BACKUP}"
printf '即将卸载项目服务。系统软件包、swap、fstab 和 Let\x27s Encrypt 账户不会被删除。\n备份：%s\n' "${BACKUP}"
if [[ -t 0 ]]; then
  read -r -p '输入 uninstall 确认: ' confirmation
  [[ ${confirmation} == uninstall ]] || { printf '已取消。\n'; exit 0; }
fi

units=(
  hy2panel.service sing-box.service
  hy2panel-apply.path hy2panel-apply.service
  hy2panel-bbr.path hy2panel-bbr.service
  hy2panel-watchdog.timer hy2panel-watchdog.service
  hy2panel-upgrade.path hy2panel-upgrade.service
  hy2panel-sing-box.path hy2panel-sing-box.service
  hy2panel-recover.service hy2panel-firewall.service
)
systemctl disable --now "${units[@]}" >/dev/null 2>&1 || true
nft delete table inet hy2panel >/dev/null 2>&1 || true

rm -f /etc/systemd/system/hy2panel*.service /etc/systemd/system/hy2panel*.path /etc/systemd/system/hy2panel*.timer /etc/systemd/system/sing-box.service
systemctl daemon-reload
systemctl reset-failed >/dev/null 2>&1 || true

rm -f /etc/letsencrypt/renewal-hooks/deploy/hy2panel
rm -f /usr/local/bin/hy2 /usr/local/bin/hy2panel
if [[ -L /usr/local/bin/sing-box && $(readlink -f /usr/local/bin/sing-box) == /usr/local/lib/hy2panel/sing-box/* ]]; then
  rm -f /usr/local/bin/sing-box
fi
rm -f /usr/local/sbin/hy2panel-apply /usr/local/sbin/hy2panel-bbr /usr/local/sbin/hy2panel-watchdog /usr/local/sbin/hy2panel-upgrade /usr/local/sbin/hy2panel-recover /usr/local/sbin/hy2panel-sing-box /usr/local/sbin/hy2panel-backup /usr/local/sbin/hy2panel-restore /usr/local/sbin/hy2panel-cert-deploy
rm -rf /usr/local/lib/hy2panel /usr/local/share/hy2panel

if [[ -f /etc/hy2panel/sysctl.backup ]]; then
  if [[ -s /etc/hy2panel/sysctl.backup ]]; then
    install -o root -g root -m 644 /etc/hy2panel/sysctl.backup /etc/sysctl.d/99-hy2panel.conf
    sysctl -p /etc/sysctl.d/99-hy2panel.conf >/dev/null 2>&1 || true
  else
    rm -f /etc/sysctl.d/99-hy2panel.conf
  fi
fi

if [[ ${PURGE} -eq 1 ]]; then
  rm -rf /etc/hy2panel /var/lib/hy2panel /var/lib/hy2panel-system
  rm -f /root/hy2panel-credentials
  if id hy2panel >/dev/null 2>&1; then
    userdel hy2panel >/dev/null 2>&1 || true
  fi
  printf '项目数据已清除；Let\x27s Encrypt 原始证书和账户仍保留。\n'
else
  printf '配置和运行数据已保留。若确认不再需要，可检查后手动删除 /etc/hy2panel 与 /var/lib/hy2panel。\n'
fi
printf 'hy2panel 已卸载。备份保存在：%s\n' "${BACKUP}"
