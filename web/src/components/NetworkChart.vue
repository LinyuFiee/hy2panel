<script setup>
import { computed } from 'vue'
import { formatRate } from '../utils'

const props = defineProps({
  samples: { type: Array, default: () => [] }
})

const maximum = computed(() => Math.max(1, ...props.samples.flatMap(item => [item.rx, item.tx])) * 1.2)

function coordinates(key) {
  const samples = props.samples
  if (!samples.length) return []
  if (samples.length === 1) {
    const y = 178 - samples[0][key] / maximum.value * 156
    return [[12, y], [628, y]]
  }
  return samples.map((item, index) => {
    const x = 12 + index * (616 / (samples.length - 1))
    const y = 178 - item[key] / maximum.value * 156
    return [x, y]
  })
}

function line(key) {
  return coordinates(key).map(([x, y], index) => `${index ? 'L' : 'M'}${x.toFixed(1)},${y.toFixed(1)}`).join(' ')
}

function area(key) {
  const points = coordinates(key)
  if (!points.length) return ''
  return `${line(key)} L${points.at(-1)[0].toFixed(1)},178 L${points[0][0].toFixed(1)},178 Z`
}

const latest = computed(() => props.samples.at(-1) || { rx: 0, tx: 0 })
</script>

<template>
  <div class="network-chart">
    <div class="chart-legend">
      <span><i class="legend-dot download"></i>下载 <strong>{{ formatRate(latest.rx) }}</strong></span>
      <span><i class="legend-dot upload"></i>上传 <strong>{{ formatRate(latest.tx) }}</strong></span>
    </div>
    <svg viewBox="0 0 640 190" role="img" aria-label="最近网络速率曲线" preserveAspectRatio="none">
      <line v-for="y in [22, 61, 100, 139, 178]" :key="y" x1="12" :y1="y" x2="628" :y2="y" class="chart-grid" />
      <path v-if="samples.length" :d="area('rx')" class="chart-area download-area" />
      <path v-if="samples.length" :d="area('tx')" class="chart-area upload-area" />
      <path v-if="samples.length" :d="line('rx')" class="chart-line download-line" />
      <path v-if="samples.length" :d="line('tx')" class="chart-line upload-line" />
    </svg>
    <p v-if="!samples.length" class="chart-empty">正在采集网络数据…</p>
  </div>
</template>