import { Activity, Gauge, ServerCog, Settings2, Wrench } from '@lucide/vue'

export const DEFAULT_SECTION = 'overview'

export const NAV_ITEMS = Object.freeze([
  {
    id: 'overview',
    label: '概览',
    shortLabel: '概览',
    eyebrow: 'CONTROL PLANE',
    title: '运行概览',
    description: '主机资源、服务健康与连接状态',
    icon: Activity
  },
  {
    id: 'nodes',
    label: '节点',
    shortLabel: '节点',
    eyebrow: 'ENDPOINTS',
    title: '节点管理',
    description: '管理 Reality、Hysteria2 与客户端接入',
    icon: ServerCog
  },
  {
    id: 'traffic',
    label: '流量',
    shortLabel: '流量',
    eyebrow: 'TELEMETRY',
    title: '流量监控',
    description: '查看协议归属、实时速率与历史趋势',
    icon: Gauge
  },
  {
    id: 'maintenance',
    label: '维护',
    shortLabel: '维护',
    eyebrow: 'OPERATIONS',
    title: '维护与诊断',
    description: '运行自检、读取日志并管理故障恢复',
    icon: Wrench
  },
  {
    id: 'settings',
    label: '设置',
    shortLabel: '设置',
    eyebrow: 'SYSTEM',
    title: '系统设置',
    description: '核心版本、网络优化、更新与访问安全',
    icon: Settings2
  }
])

const SECTION_IDS = new Set(NAV_ITEMS.map(item => item.id))

export function normalizeSection(value) {
  const section = String(value || '').trim().replace(/^#\/?/, '').split(/[/?&]/, 1)[0].toLowerCase()
  return SECTION_IDS.has(section) ? section : DEFAULT_SECTION
}

export function sectionFromHash(hash = '') {
  return normalizeSection(hash)
}

export function hashForSection(section) {
  return `#/${normalizeSection(section)}`
}

export function findNavItem(section) {
  return NAV_ITEMS.find(item => item.id === normalizeSection(section)) || NAV_ITEMS[0]
}
