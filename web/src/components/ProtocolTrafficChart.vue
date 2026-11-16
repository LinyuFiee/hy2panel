<script setup>
import { computed } from 'vue'
import { formatBytes, formatTimestamp } from '../utils'

const props = defineProps({
  history: { type: Object, default: null },
  range: { type: String, default: '1h' },
  loading: { type: Boolean, default: false }
})
const emit = defineEmits(['range'])

const rangeOptions = [
  { value: '1h', label: '1 小时' },
  { value: '6h', label: '6 小时' },
  { value: '24h', label: '24 小时' },
  { value: '7d', label: '7 天' }
]
const width = 720
const top = 18
const bottom = 166
const points = computed(() => Array.isArray(props.history?.points) ? props.history.points : [])
const maximum = computed(() => {
  const values = points.value.flatMap(point => {
    if (point?.gap) return []
    const reality = point?.traffic?.reality || {}
    const hysteria2 = point?.traffic?.hysteria2 || {}
    return [
      Number(reality.upload || 0) + Number(reality.download || 0),
      Number(hysteria2.upload || 0) + Number(hysteria2.download || 0)
    ]
  }).filter(Number.isFinite)
  return Math.max(1, ...values) * 1.12
})
const chartPoints = computed(() => {
  const source = points.value
  const count = source.length
  return source.map((point, index) => {
    const x = count <= 1 ? 12 : 12 + index * ((width - 24) / (count - 1))
    const values = {}
    for (const protocol of ['reality', 'hysteria2']) {
      const traffic = point?.traffic?.[protocol] || {}
      const total = Number(traffic.upload || 0) + Number(traffic.download || 0)
      values[protocol] = Number.isFinite(total) && total > 0 ? total : 0
    }
    return { ...point, x, values }
  })
})
const axisLabels = computed(() => {
  if (!chartPoints.value.length) return []
  const indices = [...new Set([0, Math.floor((chartPoints.value.length - 1) / 2), chartPoints.value.length - 1])]
  return indices.map(index => ({
    x: chartPoints.value[index].x,
    text: formatTimestamp(chartPoints.value[index].minute, props.range === '24h' || props.range === '7d')
  }))
})
const gapCount = computed(() => chartPoints.value.filter(point => point.gap).length)
const latestPoint = computed(() => [...chartPoints.value].reverse().find(point => !point.gap) || null)

function yFor(value) {
  return bottom - (value / maximum.value) * (bottom - top)
}

function segments(protocol) {
  const result = []
  let current = []
  for (const point of chartPoints.value) {
    if (point.gap) {
      if (current.length) result.push(current)
      current = []
      continue
    }
    current.push([point.x, yFor(point.values[protocol])])
  }
  if (current.length) result.push(current)
  return result
}

function line(segment) {
  if (!segment.length) return ''
  if (segment.length === 1) {
    const [[x, y]] = segment
    const edge = Math.min(width - 12, x + 0.01)
    return `M${x.toFixed(1)},${y.toFixed(1)} L${edge.toFixed(2)},${y.toFixed(1)}`
  }
  return segment.map(([x, y], index) => `${index ? 'L' : 'M'}${x.toFixed(1)},${y.toFixed(1)}`).join(' ')
}

function gapX(point) {
  return Math.max(11, Math.min(width - 13, point.x - 1.5))
}
</script>

<template>
  <div class="protocol-chart" data-testid="protocol-traffic-chart">
    <div class="protocol-chart-toolbar">
      <div class="chart-range-tabs" role="group" aria-label="协议流量时间范围">
        <button
          v-for="option in rangeOptions"
          :key="option.value"
          type="button"
          :class="{ active: range === option.value }"
          :aria-pressed="range === option.value"
          :disabled="loading"
          @click="emit('range', option.value)"
        >{{ option.label }}</button>
      </div>
      <span v-if="loading" class="chart-loading">正在更新…</span>
      <span v-else-if="gapCount" class="chart-gap-note">{{ gapCount }} 个时间段数据中断</span>
      <span v-else class="chart-fresh-note">每分钟归档</span>
    </div>

    <div class="chart-legend protocol-legend">
      <span><i class="legend-dot reality"></i>Reality <strong>{{ formatBytes(latestPoint?.values?.reality || 0) }}/分钟</strong></span>
      <span><i class="legend-dot hysteria2"></i>Hysteria2 <strong>{{ formatBytes(latestPoint?.values?.hysteria2 || 0) }}/分钟</strong></span>
    </div>

    <div v-if="history?.available && chartPoints.length" class="protocol-chart-canvas">
      <svg viewBox="0 0 720 190" role="img" aria-label="协议每分钟流量历史曲线" preserveAspectRatio="none">
        <line v-for="y in [18, 55, 92, 129, 166]" :key="y" x1="12" :y1="y" x2="708" :y2="y" class="chart-grid" />
        <rect
          v-for="point in chartPoints.filter(item => item.gap)"
          :key="`gap-${point.minute}`"
          :x="gapX(point)"
          y="18"
          width="3"
          height="148"
          rx="1.5"
          class="protocol-gap"
        />
        <path v-for="(segment, index) in segments('reality')" :key="`reality-${index}`" :d="line(segment)" class="chart-line reality-line" />
        <path v-for="(segment, index) in segments('hysteria2')" :key="`hysteria2-${index}`" :d="line(segment)" class="chart-line hysteria2-line" />
      </svg>
      <div class="protocol-chart-axis" aria-hidden="true">
        <span v-for="label in axisLabels" :key="`${label.x}-${label.text}`" :style="{ left: `${label.x / width * 100}%` }">{{ label.text }}</span>
      </div>
    </div>
    <div v-else class="empty-state protocol-chart-empty">
      <p>{{ history?.available === false ? '协议历史暂不可用，请稍后重试。' : '正在等待协议历史数据…' }}</p>
    </div>
  </div>
</template>
