<script setup>
import { computed } from 'vue'
import { formatRate } from '../utils'

const props = defineProps({
  samples: { type: Array, default: () => [] }
})

const width = 640
const height = 190
const left = 20
const right = 620
const top = 14
const bottom = 174
const horizontalGuides = [top, 54, 94, 134, bottom]
const verticalGuides = [left, 170, 320, 470, right]

const normalizedSamples = computed(() => props.samples.map(sample => ({
  rx: finiteNonNegative(sample?.rx),
  tx: finiteNonNegative(sample?.tx)
})))
const maximum = computed(() => niceMaximum(Math.max(0, ...normalizedSamples.value.flatMap(item => [item.rx, item.tx]))))
const latest = computed(() => normalizedSamples.value.at(-1) || { rx: 0, tx: 0 })
const peak = computed(() => Math.max(0, ...normalizedSamples.value.flatMap(item => [item.rx, item.tx])))
const chartTitle = computed(() => normalizedSamples.value.length
  ? `最近主机网络速率：下载 ${formatRate(latest.value.rx)}，上传 ${formatRate(latest.value.tx)}`
  : '最近主机网络速率：正在建立网卡速率基线')
const chartDescription = computed(() => normalizedSamples.value.length
  ? `共 ${normalizedSamples.value.length} 个样本，纵轴上限 ${formatRate(maximum.value)}，样本峰值 ${formatRate(peak.value)}。实线表示下载，虚线表示上传。`
  : '目前没有网卡速率样本；网格仅用于建立图表基线。')

function finiteNonNegative(value) {
  const numeric = Number(value)
  return Number.isFinite(numeric) && numeric > 0 ? numeric : 0
}

function niceMaximum(value) {
  if (!Number.isFinite(value) || value <= 0) return 1
  const padded = value * 1.08
  const magnitude = 10 ** Math.floor(Math.log10(padded))
  const normalized = padded / magnitude
  const step = normalized <= 1 ? 1 : normalized <= 2 ? 2 : normalized <= 2.5 ? 2.5 : normalized <= 5 ? 5 : 10
  return step * magnitude
}

function coordinates(key) {
  const samples = normalizedSamples.value
  if (!samples.length) return []
  if (samples.length === 1) {
    const y = bottom - samples[0][key] / maximum.value * (bottom - top)
    return [[left, y], [right, y]]
  }
  return samples.map((item, index) => {
    const x = left + index * ((right - left) / (samples.length - 1))
    const y = bottom - item[key] / maximum.value * (bottom - top)
    return [x, y]
  })
}

function number(value) {
  return Number(value.toFixed(2))
}

function line(key) {
  return coordinates(key)
    .map(([x, y], index) => `${index ? 'L' : 'M'} ${number(x)} ${number(y)}`)
    .join(' ')
}

function area(key) {
  const points = coordinates(key)
  if (!points.length) return ''
  return `${line(key)} L ${number(points.at(-1)[0])} ${bottom} L ${number(points[0][0])} ${bottom} Z`
}
</script>

<template>
  <div class="network-chart" data-testid="host-network-chart">
    <div class="network-legend" aria-label="当前主机网络速率">
      <span class="metric download-metric">
        <i class="series-dot" aria-hidden="true"></i>
        <span class="metric-label">下载</span>
        <strong>{{ formatRate(latest.rx) }}</strong>
      </span>
      <span class="metric upload-metric">
        <i class="series-dot" aria-hidden="true"></i>
        <span class="metric-label">上传</span>
        <strong>{{ formatRate(latest.tx) }}</strong>
      </span>
    </div>

    <div class="network-canvas" :class="{ empty: !normalizedSamples.length }">
      <svg
        :viewBox="`0 0 ${width} ${height}`"
        role="img"
        :aria-labelledby="`host-network-title host-network-description`"
        preserveAspectRatio="none"
        focusable="false"
      >
        <title id="host-network-title">{{ chartTitle }}</title>
        <desc id="host-network-description">{{ chartDescription }}</desc>
        <defs>
          <linearGradient id="host-download-fill" :x1="0" :y1="top" :x2="0" :y2="bottom" gradientUnits="userSpaceOnUse">
            <stop offset="0" stop-color="var(--chart-1)" stop-opacity="0.1" />
            <stop offset="1" stop-color="var(--chart-1)" stop-opacity="0" />
          </linearGradient>
          <linearGradient id="host-upload-fill" :x1="0" :y1="top" :x2="0" :y2="bottom" gradientUnits="userSpaceOnUse">
            <stop offset="0" stop-color="var(--chart-2)" stop-opacity="0.08" />
            <stop offset="1" stop-color="var(--chart-2)" stop-opacity="0" />
          </linearGradient>
          <clipPath id="host-network-plot-clip">
            <rect :x="left" :y="top" :width="right - left" :height="bottom - top" />
          </clipPath>
        </defs>
        <g aria-hidden="true">
          <line v-for="y in horizontalGuides" :key="`h-${y}`" :x1="left" :y1="y" :x2="right" :y2="y" class="chart-grid" />
          <line v-for="x in verticalGuides" :key="`v-${x}`" :x1="x" :y1="top" :x2="x" :y2="bottom" class="chart-grid vertical" />
        </g>
        <g v-if="normalizedSamples.length" clip-path="url(#host-network-plot-clip)" aria-hidden="true">
          <path :d="area('rx')" class="chart-area download-area" />
          <path :d="area('tx')" class="chart-area upload-area" />
          <path :d="line('rx')" class="chart-line download-line" />
          <path :d="line('tx')" class="chart-line upload-line" />
        </g>
      </svg>
      <p v-if="!normalizedSamples.length" class="chart-empty" role="status">正在建立网卡速率基线…</p>
    </div>
  </div>
</template>

<style scoped>
.network-chart {
  width: 100%;
  min-width: 0;
  display: flex;
  flex: 1;
  flex-direction: column;
}

.network-legend {
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

.upload-metric .series-dot {
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

.network-canvas {
  position: relative;
  min-width: 0;
  display: flex;
  flex: 1;
  align-items: stretch;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--border) 64%, transparent);
  border-radius: 0.35rem;
  background: color-mix(in srgb, var(--muted) 18%, transparent);
}

.network-canvas.empty {
  min-height: 10rem;
}

.network-canvas svg {
  width: 100%;
  min-height: 10.25rem;
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

.download-line {
  stroke: var(--chart-1);
}

.upload-line {
  stroke: var(--chart-2);
  stroke-dasharray: 4 2.75;
}

.chart-area.download-area {
  fill: url(#host-download-fill);
}

.chart-area.upload-area {
  fill: url(#host-upload-fill);
}

.chart-empty {
  position: absolute;
  inset: 50% 0.75rem auto;
  margin: 0;
  transform: translateY(-50%);
  color: var(--muted-foreground);
  font-size: 0.67rem;
  line-height: 1.45;
  text-align: center;
}

@media (max-width: 520px) {
  .network-canvas svg {
    min-height: 8.75rem;
  }
}

@media (max-width: 360px) {
  .network-legend {
    display: grid;
    gap: 0.3rem;
  }

  .metric {
    justify-content: flex-start;
  }
}
</style>
