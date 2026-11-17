<script setup>
import { computed } from 'vue'
import { AlertTriangle, LoaderCircle } from '@lucide/vue'
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
const left = 12
const right = 708
const top = 18
const bottom = 166

const points = computed(() => Array.isArray(props.history?.points) ? props.history.points : [])
const maximum = computed(() => {
  const values = points.value.flatMap(point => {
    if (point?.gap) return []
    return protocols.map(protocol => trafficTotal(point, protocol))
  }).filter(Number.isFinite)
  return Math.max(1, ...values) * 1.12
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

function trafficTotal(point, protocol) {
  const traffic = point?.traffic?.[protocol] || {}
  const upload = Number(traffic.upload || 0)
  const download = Number(traffic.download || 0)
  const total = (Number.isFinite(upload) && upload > 0 ? upload : 0) + (Number.isFinite(download) && download > 0 ? download : 0)
  return Number.isFinite(total) ? total : 0
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

function line(segment) {
  if (!segment.length) return ''
  if (segment.length === 1) {
    const [[x, y]] = segment
    const edge = Math.min(right, x + 0.01)
    return `M${x.toFixed(1)},${y.toFixed(1)} L${edge.toFixed(2)},${y.toFixed(1)}`
  }
  return segment
    .map(([x, y], index) => `${index ? 'L' : 'M'}${x.toFixed(1)},${y.toFixed(1)}`)
    .join(' ')
}

function gapX(point) {
  return Math.max(left - 1, Math.min(right - 1, point.x - 1.5))
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
        class="history-toggle-group"
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
          class="history-toggle-item"
        >
          <span class="wide-label">{{ option.label }}</span>
          <span class="compact-label" aria-hidden="true">{{ option.compact }}</span>
        </ToggleGroupItem>
      </ToggleGroup>

      <div class="archive-state" aria-live="polite">
        <template v-if="loading">
          <LoaderCircle class="loading-icon" :size="13" />正在更新
        </template>
        <template v-else-if="gapCount">
          <AlertTriangle :size="13" />{{ gapCount }} 个时间段数据中断
        </template>
        <template v-else>数据连续</template>
      </div>
    </div>

    <div class="protocol-legend" aria-label="最新一分钟协议流量">
      <span class="reality-metric">
        <i aria-hidden="true"></i>
        <span>Reality</span>
        <strong>{{ formatBytes(latestPoint?.values?.reality || 0) }}/分钟</strong>
      </span>
      <span class="hysteria2-metric">
        <i aria-hidden="true"></i>
        <span>Hysteria2</span>
        <strong>{{ formatBytes(latestPoint?.values?.hysteria2 || 0) }}/分钟</strong>
      </span>
    </div>

    <div v-if="history?.available && chartPoints.length" class="protocol-chart-canvas">
      <svg viewBox="0 0 720 190" role="img" aria-label="协议每分钟流量历史曲线；断线时曲线留有缺口" preserveAspectRatio="none">
        <line v-for="y in [18, 55, 92, 129, 166]" :key="y" x1="12" :y1="y" x2="708" :y2="y" class="chart-grid" />
        <line v-for="x in [12, 186, 360, 534, 708]" :key="x" :x1="x" y1="18" :x2="x" y2="166" class="chart-grid vertical" />
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
      </svg>
      <div class="protocol-chart-axis" aria-hidden="true">
        <span
          v-for="label in axisLabels"
          :key="`${label.x}-${label.text}`"
          :style="{ left: `${label.x / width * 100}%` }"
        >{{ label.text }}</span>
      </div>
    </div>

    <div v-else class="protocol-chart-empty" role="status">
      <span class="empty-orbit" aria-hidden="true"><i></i></span>
      <strong>{{ history?.available === false ? '协议历史暂不可用' : '等待历史数据' }}</strong>
      <p>{{ history?.available === false ? '实时卡与主机网络不受影响，请稍后重试。' : '完成第一批归档后会在这里显示趋势。' }}</p>
    </div>
  </div>
</template>

<style scoped>
.protocol-chart {
  min-width: 0;
  color: var(--foreground);
}

.protocol-chart-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.8rem;
}

.history-toggle-group {
  max-width: 100%;
}

.history-toggle-item {
  min-width: 4rem;
  color: var(--muted-foreground);
  font-size: 0.61rem;
}

.history-toggle-item[data-state='on'] {
  border-color: color-mix(in oklab, var(--primary) 38%, var(--border));
  background: color-mix(in oklab, var(--primary) 10%, var(--card));
  color: var(--primary);
}

.compact-label {
  display: none;
}

.archive-state {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.32rem;
  color: var(--muted-foreground);
  font-size: 0.58rem;
  text-align: right;
}

.archive-state:has(svg:not(.loading-icon)) {
  color: color-mix(in oklab, var(--chart-3) 72%, var(--foreground));
}

.loading-icon {
  animation: rotate 0.9s linear infinite;
}

.protocol-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 0.55rem;
  margin-bottom: 0.7rem;
}

.protocol-legend > span {
  min-width: min(13rem, 100%);
  display: grid;
  grid-template-columns: auto 1fr;
  align-items: center;
  gap: 0.12rem 0.42rem;
  padding: 0.5rem 0.65rem;
  border: 1px solid var(--border);
  border-radius: 0.58rem;
  background: color-mix(in oklab, var(--muted) 58%, var(--card));
}

.protocol-legend i {
  grid-row: 1 / 3;
  width: 0.48rem;
  height: 0.48rem;
  border-radius: 50%;
  background: var(--chart-1);
  box-shadow: 0 0 0 0.18rem color-mix(in oklab, var(--chart-1) 12%, transparent);
}

.protocol-legend .hysteria2-metric i {
  background: var(--chart-2);
  box-shadow: 0 0 0 0.18rem color-mix(in oklab, var(--chart-2) 12%, transparent);
}

.protocol-legend span span {
  color: var(--muted-foreground);
  font-size: 0.53rem;
}

.protocol-legend strong {
  overflow-wrap: anywhere;
  font-size: 0.68rem;
}

.protocol-chart-canvas {
  position: relative;
  min-width: 0;
  overflow: hidden;
  padding-bottom: 1.35rem;
  border: 1px solid color-mix(in oklab, var(--border) 85%, transparent);
  border-radius: 0.68rem;
  background: color-mix(in oklab, var(--background) 70%, var(--card));
}

.protocol-chart-canvas svg {
  width: 100%;
  min-height: 12rem;
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

.reality-line {
  stroke: var(--chart-1);
}

.hysteria2-line {
  stroke: var(--chart-2);
}

.protocol-gap {
  fill: color-mix(in oklab, var(--chart-3) 42%, transparent);
  stroke: color-mix(in oklab, var(--chart-3) 65%, transparent);
  stroke-width: 0.6;
  vector-effect: non-scaling-stroke;
}

.protocol-chart-axis {
  position: absolute;
  inset: auto 0.7rem 0.35rem;
  height: 0.75rem;
  color: var(--muted-foreground);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.5rem;
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
  min-height: 12rem;
  display: grid;
  align-content: center;
  justify-items: center;
  padding: 1.2rem;
  border: 1px dashed var(--border);
  border-radius: 0.68rem;
  background: color-mix(in oklab, var(--muted) 38%, var(--card));
  text-align: center;
}

.empty-orbit {
  width: 2.2rem;
  height: 2.2rem;
  display: grid;
  place-items: center;
  border: 1px solid color-mix(in oklab, var(--primary) 25%, var(--border));
  border-radius: 50%;
  background: color-mix(in oklab, var(--primary) 7%, transparent);
}

.empty-orbit i {
  width: 0.42rem;
  height: 0.42rem;
  border-radius: 50%;
  background: var(--primary);
}

.protocol-chart-empty strong {
  margin-top: 0.65rem;
  font-size: 0.73rem;
}

.protocol-chart-empty p {
  max-width: 22rem;
  margin: 0.3rem 0 0;
  color: var(--muted-foreground);
  font-size: 0.6rem;
  line-height: 1.5;
}

@keyframes rotate {
  to { transform: rotate(360deg); }
}

@media (max-width: 620px) {
  .protocol-chart-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .history-toggle-group {
    width: 100%;
  }

  .history-toggle-item {
    min-width: 0;
    flex: 1;
  }

  .archive-state {
    justify-content: flex-start;
    text-align: left;
  }

  .protocol-legend {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .protocol-legend > span {
    min-width: 0;
  }

  .protocol-chart-canvas svg {
    min-height: 9.5rem;
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
    grid-template-columns: 1fr;
  }
}

@media (prefers-reduced-motion: reduce) {
  .loading-icon {
    animation: none;
  }
}
</style>
