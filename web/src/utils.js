export function formatBytes(value, decimals = 1) {
  const bytes = Number(value || 0)
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / 1024 ** index).toFixed(index === 0 ? 0 : decimals)} ${units[index]}`
}

export function formatRate(value) {
  return `${formatBytes(value)}/s`
}

export function formatMbps(value) {
  const mbps = Number(value || 0)
  if (mbps >= 100) return mbps.toFixed(1)
  if (mbps >= 10) return mbps.toFixed(1)
  return mbps.toFixed(2)
}

export function formatUptime(value) {
  const seconds = Number(value || 0)
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (days > 0) return `${days} 天 ${hours} 小时`
  if (hours > 0) return `${hours} 小时 ${minutes} 分钟`
  return `${minutes} 分钟`
}

export function percent(used, total) {
  if (!Number(total)) return 0
  return Math.min(100, Math.max(0, Number(used || 0) / Number(total) * 100))
}