<script setup>
import { computed } from 'vue'
import { AlertTriangle, LoaderCircle } from '@lucide/vue'
import HairlineFigure from './HairlineFigure.vue'
import { ToggleGroup, ToggleGroupItem } from './ui/toggle-group'
import { formatBytes, formatTimestamp } from '../utils'

const props = defineProps({
  history: { type: Object, default: null },
  range: { type: String, default: '1h' },
  loading: { type: Boolean, default: false }
})

const emit = defineEmits(['range'])

const rangeOptions = [
  { value: '1h', label: '1 小时', compact: '1H' },
  { value: '6h', label: '6 小时', compact: '6H' },
  { value: '24h', label: '24 小时', compact: '24H' },
  { value: '7d', label: '7 天', compact: '7D' }
]
const protocols = ['reality', 'hysteria2']
const width = 720
const height = 190
const left = 20
const right = 700
const top = 16
const bottom = 164
const horizontalGuides = [top, 53, 90, 127, bottom]
const verticalGuides = [left, 190, 360, 530, right]

const points = computed(() => Array.isArray(props.history?.points) ? props.history.points : [])
const maximum = computed(() => {
  const values = points.value.flatMap(point => {
    if (point?.gap) return []
    return protocols.map(protocol => trafficTotal(point, protocol))
  }).filter(Number.isFinite)
  return niceMaximum(Math.max(0, ...values))
})

const chartPoints = computed(() => {
  const source = points.value
  const count = source.length
  return source.map((point, index) => {
    const x = count <= 1 ? left : left + index * ((right - left) / (count - 1))
    return {
      ...point,
      x,
      values: Object.fromEntries(protocols.map(protocol => [protocol, trafficTotal(point, protocol)]))
    }
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
const validPointCount = computed(() => chartPoints.value.length - gapCount.value)
const historyUnavailable = computed(() => props.history?.available === false)
const emptyTitle = computed(() => historyUnavailable.value ? '协议历史暂不可用' : '等待历史数据')
const emptyDescription = computed(() => historyUnavailable.value
  ? '此处为状态示意图，不是协议流量数据。实时卡与主机网络不受影响，请稍后重试。'
  : '此处为状态示意图，不是协议流量数据。完成第一批归档后会在这里显示趋势。')
const emptyFigureLabel = computed(() => historyUnavailable.value
  ? '协议历史不可用状态示意图，非真实流量数据'
  : '协议历史等待归档状态示意图，非真实流量数据')
const chartTitle = computed(() => `协议每分钟流量历史：Reality ${formatBytes(latestPoint.value?.values?.reality || 0)} 每分钟，Hysteria2 ${formatBytes(latestPoint.value?.values?.hysteria2 || 0)} 每分钟`)
const chartDescription = computed(() => `共 ${chartPoints.value.length} 个时间段，其中 ${validPointCount.value} 个有效时间段，${gapCount.value} 个数据中断时间段。实线为 Reality，虚线为 Hysteria2；断线不插值。纵轴上限 ${formatBytes(maximum.value)} 每分钟。`)

function trafficTotal(point, protocol) {
  const traffic = point?.traffic?.[protocol] || {}
  const upload = Number(traffic.upload || 0)
  const download = Number(traffic.download || 0)
  const total = (Number.isFinite(upload) && upload > 0 ? upload : 0) + (Number.isFinite(download) && download > 0 ? download : 0)
  return Number.isFinite(total) ? total : 0
}

function niceMaximum(value) {
  if (!Number.isFinite(value) || value <= 0) return 1
  const padded = value * 1.08
  const magnitude = 10 ** Math.floor(Math.log10(padded))
  const normalized = padded / magnitude
  const step = normalized <= 1 ? 1 : normalized <= 2 ? 2 : normalized <= 2.5 ? 2.5 : normalized <= 5 ? 5 : 10
  return step * magnitude
}

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

function number(value) {
  return Number(value.toFixed(2))
}

function line(segment) {
  if (!segment.length) return ''
  if (segment.length === 1) {
    const [[x, y]] = segment
    const edge = Math.min(right, x + 0.5)
    return `M ${number(x)} ${number(y)} L ${number(edge)} ${number(y)}`
  }
  return segment
    .map(([x, y], index) => `${index ? 'L' : 'M'} ${number(x)} ${number(y)}`)
    .join(' ')
}

function gapWidth() {
  const spacing = chartPoints.value.length > 1 ? (right - left) / (chartPoints.value.length - 1) : 4
  return Math.max(2, Math.min(6, spacing * 0.64))
}

function gapX(point) {
  const width = gapWidth()
  return Math.max(left, Math.min(right - width, point.x - width / 2))
}

function changeRange(value) {
  if (typeof value !== 'string' || !value || value === props.range || props.loading) return
  emit('range', value)
}
</script>

<template>
  <div class="protocol-chart" data-testid="protocol-traffic-chart">
    <div class="protocol-chart-toolbar">
      <ToggleGroup
        :model-value="range"
        type="single"
        variant="outline"
        size="sm"
        :disabled="loading"
        class="range-group"
        aria-label="协议流量时间范围"
        @update:model-value="changeRange"
      >
        <ToggleGroupItem
          v-for="option in rangeOptions"
          :key="option.value"
          :value="option.value"
          :aria-label="option.label"
          :aria-pressed="range === option.value"
          :disabled="loading"
          class="range-item"
        >
          <span class="wide-label">{{ option.label }}</span>
          <span class="compact-label" aria-hidden="true">{{ option.compact }}</span>
        </ToggleGroupItem>
      </ToggleGroup>

      <div class="archive-state" aria-live="polite">
        <template v-if="loading">
          <LoaderCircle class="loading-icon" :size="13" aria-hidden="true" />正在更新
        </template>
        <template v-else-if="gapCount">
          <AlertTriangle :size="13" aria-hidden="true" />{{ gapCount }} 个时间段数据中断
        </template>
        <template v-else>数据连续</template>
      </div>
    </div>

    <div class="protocol-legend" aria-label="最新一分钟协议流量">
      <span class="metric reality-metric">
        <i class="series-dot" aria-hidden="true"></i>
        <span class="metric-label">Reality</span>
        <strong>{{ formatBytes(latestPoint?.values?.reality || 0) }}/分钟</strong>
      </span>
      <span class="metric hysteria2-metric">
        <i class="series-dot hysteria2-dot" aria-hidden="true"></i>
        <span class="metric-label">Hysteria2</span>
        <strong>{{ formatBytes(latestPoint?.values?.hysteria2 || 0) }}/分钟</strong>
      </span>
    </div>

    <div v-if="history?.available && chartPoints.length" class="protocol-chart-canvas">
      <svg
        :viewBox="`0 0 ${width} ${height}`"
        role="img"
        aria-labelledby="protocol-history-title protocol-history-description"
        preserveAspectRatio="none"
        focusable="false"
      >
        <title id="protocol-history-title">{{ chartTitle }}</title>
        <desc id="protocol-history-description">{{ chartDescription }}</desc>
        <defs>
          <clipPath id="protocol-history-plot-clip">
            <rect :x="left" :y="top" :width="right - left" :height="bottom - top" />
          </clipPath>
          <pattern id="protocol-gap-hatch" width="4" height="4" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
            <line x1="0" y1="0" x2="0" y2="4" class="protocol-gap-hatch-line" />
          </pattern>
        </defs>
        <g aria-hidden="true">
          <line v-for="y in horizontalGuides" :key="`h-${y}`" :x1="left" :y1="y" :x2="right" :y2="y" class="chart-grid" />
          <line v-for="x in verticalGuides" :key="`v-${x}`" :x1="x" :y1="top" :x2="x" :y2="bottom" class="chart-grid vertical" />
        </g>
        <g clip-path="url(#protocol-history-plot-clip)" aria-hidden="true">
          <rect
            v-for="point in chartPoints.filter(item => item.gap)"
            :key="`gap-${point.minute}`"
            :x="gapX(point)"
            :y="top"
            :width="gapWidth()"
            :height="bottom - top"
            class="protocol-gap"
          />
          <path
            v-for="(segment, index) in segments('reality')"
            :key="`reality-${index}`"
            :d="line(segment)"
            class="chart-line reality-line"
          />
          <path
            v-for="(segment, index) in segments('hysteria2')"
            :key="`hysteria2-${index}`"
            :d="line(segment)"
            class="chart-line hysteria2-line"
          />
        </g>
      </svg>
      <div class="protocol-chart-axis" aria-hidden="true">
        <span
          v-for="label in axisLabels"
          :key="`${label.x}-${label.text}`"
          :style="{ left: `${label.x / width * 100}%` }"
        >{{ label.text }}</span>
      </div>
    </div>

    <div v-else class="protocol-chart-empty" role="status" aria-live="polite">
      <HairlineFigure
        figure="plot"
        name="plot"
        :play="false"
        theme="auto"
        :label="emptyFigureLabel"
        class="empty-figure"
      />
      <div class="empty-copy">
        <strong>{{ emptyTitle }}</strong>
        <p>{{ emptyDescription }}</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.protocol-chart {
  min-width: 0;
  color: var(--foreground);
}

.protocol-chart-toolbar {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.65rem;
  margin-bottom: 0.5rem;
}

/* Segmented-control visuals live in globals.css: reka's ToggleGroupItem does
   not inherit this component's scope attribute, so scoped rules never hit it. */

.compact-label {
  display: none;
}

.archive-state {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.3rem;
  color: var(--muted-foreground);
  font-size: 0.67rem;
  line-height: 1.35;
  text-align: right;
}

.loading-icon {
  animation: rotate 0.9s linear infinite;
}

.protocol-legend {
  min-width: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem 0.9rem;
  margin-bottom: 0.45rem;
}

.metric {
  min-width: 0;
  display: inline-flex;
  align-items: baseline;
  gap: 0.35rem;
}

.series-dot {
  width: 0.42rem;
  height: 0.42rem;
  align-self: center;
  border-radius: 50%;
  background: var(--chart-1);
}

.hysteria2-dot {
  border: 1px solid var(--chart-2);
  background: transparent;
}

.metric-label {
  color: var(--muted-foreground);
  font-size: 0.67rem;
}

.metric strong {
  min-width: 0;
  overflow-wrap: anywhere;
  font-size: 0.73rem;
  font-weight: 650;
  letter-spacing: -0.006em;
  font-variant-numeric: tabular-nums;
}

.protocol-chart-canvas {
  position: relative;
  min-width: 0;
  overflow: hidden;
  padding-bottom: 1.25rem;
  border: 1px solid color-mix(in srgb, var(--border) 64%, transparent);
  border-radius: 0.35rem;
  background: color-mix(in srgb, var(--muted) 18%, transparent);
}

.protocol-chart-canvas svg {
  width: 100%;
  min-height: 11rem;
  display: block;
  overflow: hidden;
  shape-rendering: geometricPrecision;
}

.chart-grid {
  stroke: color-mix(in srgb, var(--border) 78%, transparent);
  stroke-width: 0.75;
  vector-effect: non-scaling-stroke;
}

.chart-grid.vertical {
  stroke-dasharray: 1.5 4.5;
  opacity: 0.62;
}

.chart-line {
  fill: none;
  stroke-width: 1.35;
  stroke-linecap: round;
  stroke-linejoin: round;
  vector-effect: non-scaling-stroke;
}

.reality-line {
  stroke: var(--chart-1);
}

.hysteria2-line {
  stroke: var(--chart-2);
  stroke-dasharray: 4 2.75;
}

.protocol-gap {
  fill: url(#protocol-gap-hatch);
  stroke: color-mix(in srgb, var(--muted-foreground) 34%, transparent);
  stroke-width: 0.55;
  vector-effect: non-scaling-stroke;
}

.protocol-gap-hatch-line {
  stroke: color-mix(in srgb, var(--muted-foreground) 42%, transparent);
  stroke-width: 0.65;
}

.protocol-chart-axis {
  position: absolute;
  inset: auto 0.7rem 0.3rem;
  height: 0.7rem;
  color: var(--muted-foreground);
  font-size: 0.58rem;
  font-variant-numeric: tabular-nums;
  line-height: 1;
}

.protocol-chart-axis span {
  position: absolute;
  transform: translateX(-50%);
  white-space: nowrap;
}

.protocol-chart-axis span:first-child {
  transform: none;
}

.protocol-chart-axis span:last-child {
  transform: translateX(-100%);
}

.protocol-chart-empty {
  min-width: 0;
  min-height: 11rem;
  display: grid;
  grid-template-columns: minmax(8.5rem, 0.72fr) minmax(0, 1fr);
  align-items: center;
  gap: clamp(0.75rem, 3vw, 1.5rem);
  padding: 0.8rem clamp(0.8rem, 3vw, 1.25rem);
  border: 1px dashed color-mix(in srgb, var(--border) 80%, transparent);
  border-radius: 0.35rem;
  background: color-mix(in srgb, var(--muted) 18%, transparent);
}

.empty-figure {
  width: min(100%, 12.5rem);
  min-height: 0;
  max-height: 8.5rem;
  justify-self: end;
  opacity: 0.72;
  pointer-events: none;
}

.empty-copy {
  min-width: 0;
}

.protocol-chart-empty strong {
  display: block;
  font-size: 0.78rem;
  line-height: 1.35;
}

.protocol-chart-empty p {
  max-width: 25rem;
  margin: 0.32rem 0 0;
  color: var(--muted-foreground);
  font-size: 0.67rem;
  line-height: 1.52;
}

@keyframes rotate {
  to { transform: rotate(360deg); }
}

@media (max-width: 620px) {
  .protocol-chart-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .archive-state {
    justify-content: flex-start;
    text-align: left;
  }

  .protocol-chart-canvas svg {
    min-height: 9.25rem;
  }
}

@media (max-width: 480px) {
  .protocol-chart-empty {
    grid-template-columns: 1fr;
    justify-items: center;
    gap: 0.45rem;
    padding-block: 0.65rem 0.85rem;
    text-align: center;
  }

  .empty-figure {
    width: min(72%, 10rem);
    min-height: 0;
    max-height: 6.5rem;
    aspect-ratio: 2 / 1;
    justify-self: center;
  }
}

@media (max-width: 390px) {
  .wide-label {
    display: none;
  }

  .compact-label {
    display: inline;
  }

  .protocol-legend {
    display: grid;
    gap: 0.3rem;
  }
}

@media (prefers-reduced-motion: reduce) {
  .loading-icon {
    animation: none;
  }
}
</style>
