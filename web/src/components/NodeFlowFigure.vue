<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

/*
 * Link story for a single personal relay node, in solid glyph style:
 * a phone sends plain traffic into a cloud server (sing-box); inside the
 * server the feed meets a junction and spreads like a lying 人-shape into
 * Reality / relay / Hysteria2 lanes, then fans out to well-known external
 * sites (YouTube, Chrome, Telegram, web). Dots flow left to right along
 * every line; the pointer gently bends nearby lines toward itself.
 */
const props = defineProps({
  play: { type: Boolean, default: false },
  intensity: { type: Number, default: 0.5 },
  label: { type: String, default: '数据链路示意' }
})

const X0 = 74
const X1 = 884
const SAMPLES = 22

function cubicPoints(p0, p1, p2, p3) {
  const points = []
  for (let i = 0; i < SAMPLES; i += 1) {
    const t = i / (SAMPLES - 1)
    const u = 1 - t
    const x = u * u * u * p0[0] + 3 * u * u * t * p1[0] + 3 * u * t * t * p2[0] + t * t * t * p3[0]
    const y = u * u * u * p0[1] + 3 * u * u * t * p1[1] + 3 * u * t * t * p2[1] + t * t * t * p3[1]
    points.push([x, y])
  }
  return points
}

function straightPoints(x0, x1, y) {
  const points = []
  for (let i = 0; i < SAMPLES; i += 1) {
    const t = i / (SAMPLES - 1)
    points.push([x0 + (x1 - x0) * t, y])
  }
  return points
}

function segmentPoints(p0, p1) {
  const points = []
  for (let i = 0; i < SAMPLES; i += 1) {
    const t = i / (SAMPLES - 1)
    points.push([p0[0] + (p1[0] - p0[0]) * t, p0[1] + (p1[1] - p0[1]) * t])
  }
  return points
}

const EXTERNAL_LINES = [
  { id: 'in-phone', tone: 'gray', pts: straightPoints(74, 455, 132) },
  { id: 'out-youtube', tone: 'reality', pts: cubicPoints([540, 108], [670, 108], [760, 64], [878, 64]) },
  { id: 'out-chrome', tone: 'hysteria2', pts: cubicPoints([540, 156], [670, 156], [760, 108], [878, 108]) },
  { id: 'out-telegram', tone: 'reality', pts: cubicPoints([540, 108], [700, 108], [790, 152], [878, 152]) },
  { id: 'out-web', tone: 'relay', pts: cubicPoints([540, 132], [690, 132], [790, 196], [878, 196]) }
]

// The lying-人: every leg leaves the junction at (455, 132) and spreads
// across the server body toward its right edge.
const CORE_LINES = [
  { id: 'leg-reality', tone: 'reality', pts: segmentPoints([455, 132], [540, 108]) },
  { id: 'leg-relay', tone: 'relay', pts: segmentPoints([455, 132], [540, 132]) },
  { id: 'leg-hysteria2', tone: 'hysteria2', pts: segmentPoints([455, 132], [540, 156]) }
]

const ALL_LINES = [...EXTERNAL_LINES, ...CORE_LINES]

const LANES = [
  { tone: 'reality', y: 108 },
  { tone: 'relay', y: 132 },
  { tone: 'hysteria2', y: 156 }
]

const rootEl = ref(null)
const progress = ref(0.5)
const pointer = ref({ x: 480, y: 130 })
const visible = ref(false)
const pageVisible = ref(true)
const reducedMotion = ref(false)
const interacted = ref(false)
const mounted = ref(false)
let rafId = 0
let observer = null
let motionQuery = null

const amplitude = computed(() => 11 * Math.max(0, Math.min(1, props.intensity)))
const shouldFlow = computed(() => (
  mounted.value && props.play && visible.value && pageVisible.value && !reducedMotion.value
))
const pathsCanRespond = computed(() => props.play && !reducedMotion.value)

const animationState = computed(() => {
  if (!props.play) return 'static'
  if (reducedMotion.value) return 'reduced-motion'
  return visible.value && pageVisible.value ? 'playing' : 'paused'
})

function bump(baseY, x) {
  const dx = x - pointer.value.x
  const sigma = 80
  const falloff = Math.exp(-(dx * dx) / (2 * sigma * sigma))
  const direction = Math.max(-1, Math.min(1, (pointer.value.y - baseY) / 50))
  return amplitude.value * falloff * direction
}

function linePath(line) {
  const displace = pathsCanRespond.value
  const parts = []
  line.pts.forEach(([x, y], index) => {
    const renderedY = displace ? y + bump(y, x) : y
    parts.push(`${index === 0 ? 'M' : 'L'}${x.toFixed(1)},${renderedY.toFixed(1)}`)
  })
  return parts.join(' ')
}

function cancelRebuild() {
  if (rafId && typeof cancelAnimationFrame === 'function') cancelAnimationFrame(rafId)
  rafId = 0
}

function scheduleRebuild() {
  if (rafId || !shouldFlow.value || typeof requestAnimationFrame !== 'function') return
  rafId = requestAnimationFrame(() => {
    rafId = 0
    if (!shouldFlow.value) return
    pointer.value = { ...pointer.value }
  })
}

function handleVisibilityChange() {
  pageVisible.value = !document.hidden
}

function handleMotionChange(event) {
  reducedMotion.value = event.matches
}

function resetInteraction() {
  interacted.value = false
  pointer.value = { x: 480, y: 130 }
  progress.value = 0.5
}

function handlePointerMove(event) {
  if (!props.play) return
  const el = rootEl.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  if (!rect.width || !rect.height) return
  interacted.value = true
  const scale = Math.min(rect.width / 960, rect.height / 180)
  const offsetX = (rect.width - 960 * scale) / 2
  const offsetY = (rect.height - 180 * scale) / 2
  const x = Math.min(X1, Math.max(X0, (event.clientX - rect.left - offsetX) / scale))
  const y = 40 + Math.min(180, Math.max(0, (event.clientY - rect.top - offsetY) / scale))
  pointer.value = { x, y }
  progress.value = Number(((x - X0) / (X1 - X0)).toFixed(4))
  scheduleRebuild()
}

function handlePointerLeave() {
  if (!props.play) return
  resetInteraction()
  scheduleRebuild()
}

watch(shouldFlow, flowing => {
  if (flowing) scheduleRebuild()
  else cancelRebuild()
}, { flush: 'post' })

watch(() => props.play, playing => {
  if (!playing) resetInteraction()
})

watch(() => props.intensity, () => {
  if (shouldFlow.value && interacted.value) scheduleRebuild()
})

onMounted(() => {
  mounted.value = true
  pageVisible.value = !document.hidden
  document.addEventListener('visibilitychange', handleVisibilityChange)

  motionQuery = window.matchMedia?.('(prefers-reduced-motion: reduce)') ?? null
  reducedMotion.value = Boolean(motionQuery?.matches)
  if (motionQuery?.addEventListener) motionQuery.addEventListener('change', handleMotionChange)
  else motionQuery?.addListener?.(handleMotionChange)

  if (typeof window.IntersectionObserver === 'function') {
    observer = new window.IntersectionObserver(entries => {
      const entry = entries.find(item => item.target === rootEl.value) || entries[0]
      visible.value = Boolean(entry?.isIntersecting)
    }, { threshold: 0.01 })
    if (rootEl.value) observer.observe(rootEl.value)
  } else {
    visible.value = true
  }
})

onBeforeUnmount(() => {
  mounted.value = false
  cancelRebuild()
  observer?.disconnect()
  observer = null
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  if (motionQuery?.removeEventListener) motionQuery.removeEventListener('change', handleMotionChange)
  else motionQuery?.removeListener?.(handleMotionChange)
  motionQuery = null
})
</script>

<template>
  <div
    ref="rootEl"
    class="node-flow"
    :class="{ 'is-paused': !shouldFlow }"
    data-figure="node-flow"
    role="img"
    :aria-label="label"
    :data-animation-state="animationState"
    :data-progress="progress"
    @pointermove="handlePointerMove"
    @pointerleave="handlePointerLeave"
  >
    <div class="flow-caption" aria-hidden="true">中转站 · sing-box</div>

    <svg viewBox="0 40 960 180" aria-hidden="true" preserveAspectRatio="xMidYMid meet">
      <!-- phone client (solid) -->
      <g class="solid">
        <rect x="40" y="106" width="28" height="52" rx="8" />
      </g>
      <rect x="44" y="112" width="20" height="36" rx="4" class="cutout" />
      <rect x="48" y="150" width="12" height="2.5" rx="1.25" class="cutout" />

      <!-- cloud server body -->
      <rect x="420" y="82" width="120" height="100" rx="12" class="solid" />
      <rect v-for="lane in LANES" :key="`slot-${lane.y}`" x="430" :y="lane.y - 3" width="104" height="6" rx="3" class="cutout" />
      <!-- cloud above the server -->
      <g class="solid">
        <circle cx="470" cy="62" r="9" />
        <circle cx="483" cy="56" r="11" />
        <circle cx="496" cy="62" r="8" />
        <rect x="468" y="60" width="26" height="10" rx="5" />
      </g>
      <line x1="480" y1="72" x2="480" y2="80" class="cloud-link" />

      <!-- every line, drawn over the server body -->
      <g v-for="line in ALL_LINES" :key="line.id">
        <path :d="linePath(line)" class="line-wire" />
        <path :d="linePath(line)" class="line-dots" :class="`tone-${line.tone}`" />
      </g>

      <!-- junction of the lying-人 -->
      <circle cx="455" cy="132" r="2.8" class="junction-dot" />
      <!-- lane LEDs on the server face -->
      <circle v-for="lane in LANES" :key="`led-${lane.y}`" cx="425" :cy="lane.y" r="2" class="lane-led" :class="`tone-${lane.tone}`" />

      <!-- target site icons (solid, small) -->
      <!-- YouTube -->
      <g>
        <rect x="886" y="53.5" width="30" height="21" rx="6" class="solid" />
        <path d="M897,58.8 L908,64 L897,69.2 Z" class="cutout" />
      </g>
      <!-- Chrome -->
      <g>
        <circle cx="901" cy="108" r="12" class="solid" />
        <circle cx="901" cy="108" r="4" class="cutout" />
        <line x1="905" y1="108" x2="913" y2="108" class="cutout-stroke" />
        <line x1="899" y1="111.5" x2="895" y2="118.4" class="cutout-stroke" />
        <line x1="899" y1="104.5" x2="895" y2="97.6" class="cutout-stroke" />
      </g>
      <!-- Telegram -->
      <g>
        <path d="M886,153 L916,144 L906,162.5 L899.5,156 Z" class="solid" />
        <line x1="916" y1="144" x2="900.5" y2="155.5" class="cutout-stroke" />
      </g>
      <!-- Web globe -->
      <g>
        <circle cx="901" cy="196" r="12" class="solid" />
        <line x1="889" y1="196" x2="913" y2="196" class="cutout-stroke" />
        <line x1="891.2" y1="189.5" x2="910.8" y2="189.5" class="cutout-stroke" />
        <line x1="891.2" y1="202.5" x2="910.8" y2="202.5" class="cutout-stroke" />
        <ellipse cx="901" cy="196" rx="4.5" ry="12" class="cutout-stroke" />
      </g>

      <!-- pointer scanline -->
      <line
        v-if="play && !reducedMotion && interacted"
        :x1="pointer.x"
        y1="50"
        :x2="pointer.x"
        y2="212"
        class="scanline"
      />
    </svg>

    <div class="flow-legend">
      <span><i class="legend-dot reality"></i>Reality</span>
      <span><i class="legend-dot hysteria2"></i>Hysteria2</span>
      <span><i class="legend-dot relay"></i>普通数据</span>
    </div>
  </div>
</template>

<style scoped>
.node-flow {
  position: relative;
  width: 100%;
  min-width: 0;
  touch-action: pan-y;
  user-select: none;
  -webkit-tap-highlight-color: transparent;
}

.node-flow svg {
  display: block;
  width: 100%;
  height: auto;
}

.solid {
  fill: var(--foreground);
}

.cutout {
  fill: var(--card);
}

.cutout-stroke {
  fill: none;
  stroke: var(--card);
  stroke-width: 1.4;
  vector-effect: non-scaling-stroke;
}

.flow-caption {
  position: absolute;
  top: 0.45rem;
  left: 0.9rem;
  color: var(--muted-foreground);
  font-size: 0.68rem;
  pointer-events: none;
  white-space: nowrap;
}

.cloud-link {
  stroke: var(--hairline-mid);
  stroke-width: 1;
  stroke-dasharray: 2 3;
  vector-effect: non-scaling-stroke;
}

.line-wire {
  fill: none;
  stroke: var(--hairline-mid);
  stroke-width: 1;
  stroke-linecap: round;
  stroke-linejoin: round;
  vector-effect: non-scaling-stroke;
}

.line-dots {
  fill: none;
  stroke-width: 2.2;
  stroke-linecap: round;
  stroke-linejoin: round;
  vector-effect: non-scaling-stroke;
  stroke-dasharray: 0.1 9.9;
  animation: lane-flow 3s linear infinite;
}

.tone-gray {
  stroke: var(--muted-foreground);
  opacity: 0.5;
}

.tone-reality {
  stroke: var(--chart-1);
}

.tone-hysteria2 {
  stroke: var(--chart-2);
}

.tone-relay {
  stroke: var(--muted-foreground);
  opacity: 0.45;
  animation-duration: 4.2s;
}

.is-paused .line-dots {
  animation-play-state: paused;
}

@keyframes lane-flow {
  to {
    stroke-dashoffset: -200;
  }
}

.junction-dot {
  fill: var(--foreground);
  stroke: var(--card);
  stroke-width: 1.5;
  vector-effect: non-scaling-stroke;
}

.lane-led.tone-reality {
  fill: var(--chart-1);
}

.lane-led.tone-hysteria2 {
  fill: var(--chart-2);
}

.lane-led.tone-relay {
  fill: var(--muted-foreground);
}

.scanline {
  stroke: var(--hairline-edge);
  stroke-width: 1;
  opacity: 0.3;
  vector-effect: non-scaling-stroke;
}

.flow-legend {
  position: absolute;
  left: 0.9rem;
  bottom: 0.5rem;
  display: flex;
  gap: 0.9rem;
  pointer-events: none;
}

.flow-legend span {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  color: var(--muted-foreground);
  font-size: 0.68rem;
}

.legend-dot {
  width: 0.45rem;
  height: 0.45rem;
  border-radius: 50%;
}

.legend-dot.reality {
  background: var(--chart-1);
}

.legend-dot.hysteria2 {
  background: var(--chart-2);
}

.legend-dot.relay {
  background: var(--muted-foreground);
  opacity: 0.45;
}

@media (max-width: 640px) {
  .node-flow svg {
    min-height: 150px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .line-dots {
    animation: none;
  }
}
</style>
