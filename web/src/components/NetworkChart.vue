<script setup>
import { computed } from 'vue'
import { ArrowDownToLine, ArrowUpFromLine } from '@lucide/vue'
import { formatRate } from '../utils'

const props = defineProps({
  samples: { type: Array, default: () => [] }
})

const width = 640
const left = 12
const right = 628
const top = 16
const bottom = 174

const normalizedSamples = computed(() => props.samples.map(sample => ({
  rx: finiteNonNegative(sample?.rx),
  tx: finiteNonNegative(sample?.tx)
})))
const maximum = computed(() => Math.max(1, ...normalizedSamples.value.flatMap(item => [item.rx, item.tx])) * 1.15)
const latest = computed(() => normalizedSamples.value.at(-1) || { rx: 0, tx: 0 })

function finiteNonNegative(value) {
  const numeric = Number(value)
  return Number.isFinite(numeric) && numeric > 0 ? numeric : 0
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

function line(key) {
  return coordinates(key)
    .map(([x, y], index) => `${index ? 'L' : 'M'}${x.toFixed(1)},${y.toFixed(1)}`)
    .join(' ')
}

function area(key) {
  const points = coordinates(key)
  if (!points.length) return ''
  return `${line(key)} L${points.at(-1)[0].toFixed(1)},${bottom} L${points[0][0].toFixed(1)},${bottom} Z`
}
</script>

<template>
  <div class="network-chart" data-testid="host-network-chart">
    <div class="network-legend" aria-label="当前主机网络速率">
      <span class="download-metric">
        <i aria-hidden="true"><ArrowDownToLine :size="13" /></i>
        <span>下载</span>
        <strong>{{ formatRate(latest.rx) }}</strong>
      </span>
      <span class="upload-metric">
        <i aria-hidden="true"><ArrowUpFromLine :size="13" /></i>
        <span>上传</span>
        <strong>{{ formatRate(latest.tx) }}</strong>
      </span>
    </div>

    <div class="network-canvas" :class="{ empty: !normalizedSamples.length }">
      <svg viewBox="0 0 640 190" role="img" aria-label="最近主机网络下载与上传速率曲线" preserveAspectRatio="none">
        <defs>
          <linearGradient id="host-download-fill" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stop-color="var(--chart-1)" stop-opacity="0.22" />
            <stop offset="1" stop-color="var(--chart-1)" stop-opacity="0.01" />
          </linearGradient>
          <linearGradient id="host-upload-fill" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stop-color="var(--chart-2)" stop-opacity="0.14" />
            <stop offset="1" stop-color="var(--chart-2)" stop-opacity="0.01" />
          </linearGradient>
        </defs>
        <line v-for="y in [16, 55.5, 95, 134.5, 174]" :key="y" x1="12" :y1="y" x2="628" :y2="y" class="chart-grid" />
        <line v-for="x in [12, 166, 320, 474, 628]" :key="x" :x1="x" y1="16" :x2="x" y2="174" class="chart-grid vertical" />
        <path v-if="normalizedSamples.length" :d="area('rx')" class="chart-area download-area" />
        <path v-if="normalizedSamples.length" :d="area('tx')" class="chart-area upload-area" />
        <path v-if="normalizedSamples.length" :d="line('rx')" class="chart-line download-line" />
        <path v-if="normalizedSamples.length" :d="line('tx')" class="chart-line upload-line" />
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
  display: flex;
  flex-wrap: wrap;
  gap: 0.6rem;
  margin-bottom: 0.65rem;
}

.network-legend > span {
  min-width: min(11rem, 100%);
  display: grid;
  grid-template-columns: auto 1fr;
  align-items: center;
  gap: 0.12rem 0.42rem;
  padding: 0.55rem 0.65rem;
  border: 1px solid var(--border);
  border-radius: 0.6rem;
  background: color-mix(in oklab, var(--muted) 62%, var(--card));
}

.network-legend i {
  grid-row: 1 / 3;
  width: 1.55rem;
  height: 1.55rem;
  display: grid;
  place-items: center;
  border-radius: 0.45rem;
  background: color-mix(in oklab, var(--chart-1) 12%, var(--card));
  color: var(--chart-1);
}

.network-legend .upload-metric i {
  background: color-mix(in oklab, var(--chart-2) 12%, var(--card));
  color: var(--chart-2);
}

.network-legend span span {
  color: var(--muted-foreground);
  font-size: 0.52rem;
}

.network-legend strong {
  overflow-wrap: anywhere;
  font-size: 0.68rem;
}

.network-canvas {
  position: relative;
  min-width: 0;
  display: flex;
  flex: 1;
  align-items: stretch;
  overflow: hidden;
  border: 1px solid color-mix(in oklab, var(--border) 85%, transparent);
  border-radius: 0.65rem;
  background: color-mix(in oklab, var(--background) 70%, var(--card));
}

.network-canvas.empty {
  min-height: 10rem;
}

.network-canvas svg {
  width: 100%;
  min-height: 10.5rem;
  display: block;
  overflow: hidden;
}

.chart-grid {
  stroke: var(--border);
  stroke-width: 1;
  vector-effect: non-scaling-stroke;
}

.chart-grid.vertical {
  opacity: 0.45;
}

.chart-line {
  fill: none;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
  vector-effect: non-scaling-stroke;
}

.download-line {
  stroke: var(--chart-1);
}

.upload-line {
  stroke: var(--chart-2);
}

.chart-area.download-area {
  fill: url(#host-download-fill);
}

.chart-area.upload-area {
  fill: url(#host-upload-fill);
}

.chart-empty {
  position: absolute;
  inset: 50% 1rem auto;
  margin: 0;
  transform: translateY(-50%);
  color: var(--muted-foreground);
  font-size: 0.65rem;
  text-align: center;
}

@media (max-width: 520px) {
  .network-legend {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .network-legend > span {
    min-width: 0;
  }

  .network-canvas svg {
    min-height: 9rem;
  }
}

@media (max-width: 370px) {
  .network-legend {
    grid-template-columns: 1fr;
  }
}
</style>
