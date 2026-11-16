const BYTE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB']

function toNonNegativeBigInt(value) {
  if (typeof value === 'bigint') return value > 0n ? value : 0n
  if (typeof value === 'number') {
    if (!Number.isFinite(value) || value <= 0) return 0n
    return BigInt(Math.floor(value))
  }
  if (typeof value === 'string') {
    const normalized = value.trim()
    if (/^\d+$/.test(normalized)) {
      try {
        return BigInt(normalized)
      } catch {
        return 0n
      }
    }
  }
  return 0n
}

export function sumBytes(...values) {
  return values.reduce((total, value) => total + toNonNegativeBigInt(value), 0n)
}

export function byteValueAsNumber(value) {
  if (typeof value === 'string' && /^\d+$/.test(value.trim())) {
    const parsed = Number(value)
    return Number.isFinite(parsed) && parsed > 0 ? parsed : 0
  }
  const numeric = typeof value === 'number' ? value : 0
  return Number.isFinite(numeric) && numeric > 0 ? numeric : 0
}

export function formatBytes(value, decimals = 1) {
  const bytes = toNonNegativeBigInt(value)
  if (bytes === 0n) return '0 B'

  let unitIndex = 0
  let divisor = 1n
  while (unitIndex < BYTE_UNITS.length - 1 && bytes >= divisor * 1024n) {
    divisor *= 1024n
    unitIndex += 1
  }

  if (unitIndex === 0) return `${bytes} B`
  const places = Math.max(0, Math.min(3, Math.trunc(Number(decimals) || 0)))
  if (places === 0) return `${(bytes + divisor / 2n) / divisor} ${BYTE_UNITS[unitIndex]}`

  const factor = 10n ** BigInt(places)
  const scaled = (bytes * factor + divisor / 2n) / divisor
  const whole = scaled / factor
  const fraction = (scaled % factor).toString().padStart(places, '0')
  return `${whole}.${fraction} ${BYTE_UNITS[unitIndex]}`
}

export function formatRate(value) {
  return `${formatBytes(value)}/s`
}

export function formatCount(value) {
  const integer = toNonNegativeBigInt(value).toString()
  return integer.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
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

export function formatTimestamp(value, includeDate = false) {
  if (!value) return '尚无采样'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '时间未知'
  const pad = part => String(part).padStart(2, '0')
  const time = `${pad(date.getHours())}:${pad(date.getMinutes())}`
  return includeDate ? `${pad(date.getMonth() + 1)}/${pad(date.getDate())} ${time}` : time
}
