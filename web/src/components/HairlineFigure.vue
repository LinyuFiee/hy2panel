<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

const VIEWBOX_WIDTH = 400
const VIEWBOX_HEIGHT = 320
const BLADE_TOP = 100
const BLADE_PITCH = 27
const BLADE_HEIGHT = 21
const BLADE_COUNT = 6
const PORT_COUNT = 8
const PORT_START_X = 121
const PORT_PITCH = 20.5
const PORT_WIDTH = 13.5
const PORT_Y = 61
const PORT_HEIGHT = 12
const PORT_BY_BLADE = [0, 2, 3, 4, 6, 7]
const DEFAULT_LABEL = '服务器机架流量路径：从入口经过配线端口和服务器刀片到达出口'

const props = defineProps({
  play: { type: Boolean, default: false },
  intensity: { type: Number, default: 0.5 },
  theme: {
    type: String,
    default: 'auto',
    validator: value => ['auto', 'light', 'dark'].includes(value)
  },
  label: { type: String, default: DEFAULT_LABEL },
  density: {
    type: String,
    default: 'regular',
    validator: value => ['regular', 'compact'].includes(value)
  },
  interactive: { type: Boolean, default: true }
})

const emit = defineEmits(['read'])

const host = ref(null)
const svg = ref(null)
const selectedBlade = ref(2)
const progress = ref(0.68)
const intersecting = ref(true)
const pageVisible = ref(true)
const reducedMotion = ref(false)
const pointerActive = ref(false)
const mounted = ref(false)

const blades = Array.from({ length: BLADE_COUNT }, (_, index) => ({
  number: index + 1,
  y: BLADE_TOP + index * BLADE_PITCH
}))
const ports = Array.from({ length: PORT_COUNT }, (_, index) => ({
  number: index + 1,
  x: PORT_START_X + index * PORT_PITCH
}))

const resolvedLabel = computed(() => props.label?.trim() || DEFAULT_LABEL)
const resolvedIntensity = computed(() => {
  const value = Number(props.intensity)
  return Number.isFinite(value) ? Math.min(1, Math.max(0, value)) : 0.5
})
const activePortIndex = computed(() => PORT_BY_BLADE[selectedBlade.value])
const activePortNumber = computed(() => activePortIndex.value + 1)
const activePortX = computed(() => PORT_START_X + activePortIndex.value * PORT_PITCH + PORT_WIDTH / 2)
const bladeCenterY = computed(() => BLADE_TOP + selectedBlade.value * BLADE_PITCH + BLADE_HEIGHT / 2)
const selectedPull = computed(() => {
  const compactScale = props.density === 'compact' ? 0.72 : 1
  return (4 + resolvedIntensity.value * 8) * compactScale
})

const routePoints = computed(() => [
  { x: 45, y: 157 },
  { x: 76, y: 157 },
  { x: 76, y: 67 },
  { x: activePortX.value, y: 67 },
  { x: activePortX.value, y: 91 },
  { x: 101, y: 91 },
  { x: 101, y: bladeCenterY.value },
  { x: 122 + selectedPull.value, y: bladeCenterY.value },
  { x: 276 + selectedPull.value, y: bladeCenterY.value },
  { x: 320, y: bladeCenterY.value },
  { x: 320, y: 157 },
  { x: 355, y: 157 }
])
const routePath = computed(() => routePoints.value.map((point, index) => `${index ? 'L' : 'M'} ${point.x} ${point.y}`).join(' '))
const routeDash = computed(() => `${Math.max(0, Math.min(1, progress.value))} 1`)
const flowPoint = computed(() => pointAlongRoute(routePoints.value, progress.value))
const animationState = computed(() => {
  if (!props.play) return 'idle'
  if (!pageVisible.value) return 'paused-hidden'
  if (!intersecting.value) return 'paused-offscreen'
  if (reducedMotion.value) return 'reduced-motion'
  if (pointerActive.value) return 'interactive'
  return 'playing'
})
const shouldAnimate = computed(() => (
  mounted.value &&
  props.play &&
  pageVisible.value &&
  intersecting.value &&
  !reducedMotion.value &&
  !pointerActive.value
))

let animationFrame = null
let lastFrameTime = null
let tourPhase = (selectedBlade.value + progress.value) / BLADE_COUNT
let observer = null
let motionQuery = null
let coarseQuery = null
let tapResumeTimer = null
let lastReadKey = ''

function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value))
}

function pointAlongRoute(points, amount) {
  const safeAmount = clamp(Number(amount) || 0, 0, 1)
  const segments = []
  let totalLength = 0

  for (let index = 1; index < points.length; index += 1) {
    const start = points[index - 1]
    const end = points[index]
    const length = Math.hypot(end.x - start.x, end.y - start.y)
    segments.push({ start, end, length })
    totalLength += length
  }

  let remaining = safeAmount * totalLength
  for (const segment of segments) {
    if (remaining <= segment.length) {
      const ratio = segment.length ? remaining / segment.length : 0
      return {
        x: segment.start.x + (segment.end.x - segment.start.x) * ratio,
        y: segment.start.y + (segment.end.y - segment.start.y) * ratio
      }
    }
    remaining -= segment.length
  }

  return points.at(-1) || { x: 0, y: 0 }
}

function bladeTransform(index) {
  return index === selectedBlade.value ? `translate(${selectedPull.value} 0)` : 'translate(0 0)'
}

function readText() {
  const percent = Math.round(progress.value * 100)
  return `入口到出口，配线端口 ${activePortNumber.value}，服务器 ${selectedBlade.value + 1}U，路径 ${percent}%`
}

function publishRead(force = false) {
  const bucket = Math.round(progress.value * 20) * 5
  const key = `${selectedBlade.value}:${activePortNumber.value}:${bucket}`
  if (!force && key === lastReadKey) return
  lastReadKey = key
  emit('read', readText())
}

function stopAnimation() {
  if (animationFrame !== null) {
    window.cancelAnimationFrame(animationFrame)
    animationFrame = null
  }
  lastFrameTime = null
}

function requestAnimation() {
  if (!shouldAnimate.value || animationFrame !== null) return
  animationFrame = window.requestAnimationFrame(runTour)
}

function runTour(timestamp) {
  animationFrame = null
  if (!shouldAnimate.value) {
    lastFrameTime = null
    return
  }

  if (lastFrameTime !== null) {
    const elapsed = Math.min(80, Math.max(0, timestamp - lastFrameTime))
    const duration = 9800 - resolvedIntensity.value * 3600
    tourPhase = (tourPhase + elapsed / duration) % 1
  }
  lastFrameTime = timestamp

  const position = tourPhase * BLADE_COUNT
  const nextBlade = Math.min(BLADE_COUNT - 1, Math.floor(position))
  const localProgress = position - nextBlade
  selectedBlade.value = nextBlade
  progress.value = localProgress * localProgress * (3 - 2 * localProgress)

  requestAnimation()
}

function syncAnimation() {
  if (shouldAnimate.value) requestAnimation()
  else stopAnimation()
}

function pointerPosition(event) {
  const bounds = svg.value?.getBoundingClientRect()
  if (!bounds?.width || !bounds?.height) return null

  const scale = Math.min(bounds.width / VIEWBOX_WIDTH, bounds.height / VIEWBOX_HEIGHT)
  if (!scale) return null
  const drawnWidth = VIEWBOX_WIDTH * scale
  const drawnHeight = VIEWBOX_HEIGHT * scale
  const offsetX = bounds.left + (bounds.width - drawnWidth) / 2
  const offsetY = bounds.top + (bounds.height - drawnHeight) / 2

  return {
    x: clamp((event.clientX - offsetX) / scale, 0, VIEWBOX_WIDTH),
    y: clamp((event.clientY - offsetY) / scale, 0, VIEWBOX_HEIGHT)
  }
}

function selectFromPointer(event) {
  if (!props.interactive) return
  const point = pointerPosition(event)
  if (!point) return

  const bladeBandTop = BLADE_TOP + BLADE_HEIGHT / 2 - BLADE_PITCH / 2
  const blade = Math.floor((point.y - bladeBandTop) / BLADE_PITCH)
  selectedBlade.value = clamp(blade, 0, BLADE_COUNT - 1)
  progress.value = clamp(point.x / VIEWBOX_WIDTH, 0, 1)
  tourPhase = (selectedBlade.value + progress.value) / BLADE_COUNT
  publishRead()
}

function handlePointerEnter(event) {
  if (!props.interactive || event.pointerType === 'touch' || coarseQuery?.matches) return
  pointerActive.value = true
  selectFromPointer(event)
}

function handlePointerMove(event) {
  if (!props.interactive || event.pointerType === 'touch' || coarseQuery?.matches) return
  pointerActive.value = true
  selectFromPointer(event)
}

function handlePointerLeave() {
  if (!pointerActive.value) return
  pointerActive.value = false
}

function handlePointerDown(event) {
  if (!props.interactive) return
  const coarsePointer = event.pointerType === 'touch' || coarseQuery?.matches
  if (!coarsePointer) return

  selectFromPointer(event)
  pointerActive.value = true
  window.clearTimeout(tapResumeTimer)
  tapResumeTimer = window.setTimeout(() => {
    pointerActive.value = false
  }, 900)
}

function handleVisibilityChange() {
  pageVisible.value = !document.hidden
}

function handleMotionChange(event) {
  reducedMotion.value = event.matches
}

watch(shouldAnimate, syncAnimation, { flush: 'post' })
watch(selectedBlade, () => publishRead())
watch(() => props.play, playing => {
  if (playing) tourPhase = (selectedBlade.value + progress.value) / BLADE_COUNT
})
watch(() => props.interactive, enabled => {
  if (!enabled) pointerActive.value = false
})

onMounted(() => {
  mounted.value = true
  pageVisible.value = !document.hidden

  motionQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
  coarseQuery = window.matchMedia('(pointer: coarse)')
  reducedMotion.value = motionQuery.matches
  if (motionQuery.addEventListener) motionQuery.addEventListener('change', handleMotionChange)
  else motionQuery.addListener?.(handleMotionChange)

  document.addEventListener('visibilitychange', handleVisibilityChange)

  if ('IntersectionObserver' in window) {
    intersecting.value = false
    observer = new window.IntersectionObserver(entries => {
      const entry = entries.find(item => item.target === host.value) || entries[0]
      intersecting.value = Boolean(entry?.isIntersecting && entry.intersectionRatio > 0)
    }, { threshold: 0.01 })
    observer.observe(host.value)
  }

  publishRead(true)
  syncAnimation()
})

onBeforeUnmount(() => {
  mounted.value = false
  stopAnimation()
  observer?.disconnect()
  observer = null
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  if (motionQuery?.removeEventListener) motionQuery.removeEventListener('change', handleMotionChange)
  else motionQuery?.removeListener?.(handleMotionChange)
  window.clearTimeout(tapResumeTimer)
})
</script>

<template>
  <div
    ref="host"
    class="hairline-figure"
    :class="{ 'is-interactive': interactive }"
    data-figure="server-rack-flow"
    data-rack-units="6"
    data-rack-ports="8"
    data-route-order="ingress-port-blade-egress"
    :data-theme="theme"
    :data-density="density"
    :data-play="play ? 'true' : 'false'"
    :data-interactive="interactive ? 'true' : 'false'"
    :data-selected-blade="selectedBlade + 1"
    :data-active-port="activePortNumber"
    :data-progress="progress.toFixed(3)"
    :data-animation-state="animationState"
    role="img"
    :aria-label="resolvedLabel"
    @pointerenter="handlePointerEnter"
    @pointermove="handlePointerMove"
    @pointerleave="handlePointerLeave"
    @pointercancel="handlePointerLeave"
    @pointerdown="handlePointerDown"
  >
    <svg
      ref="svg"
      class="server-rack-flow"
      width="400"
      height="320"
      viewBox="0 0 400 320"
      preserveAspectRatio="xMidYMid meet"
      aria-hidden="true"
      focusable="false"
    >
      <rect class="figure-plate" x="1" y="1" width="398" height="318" rx="20" />

      <g class="rack-shadow" aria-hidden="true">
        <path d="M 108 294 L 296 294 L 316 304 L 88 304 Z" />
        <path d="M 304 31 L 314 41 L 314 279 L 304 292 Z" />
      </g>

      <g class="rack-frame" data-rack="6u-server-rack">
        <rect x="96" y="18" width="208" height="276" rx="9" />
        <path d="M 105 45 H 295 M 105 88 H 295 M 105 268 H 295" />
        <path class="rack-rail" d="M 106 46 V 267 M 294 46 V 267" />
        <circle cx="106" cy="34" r="2" />
        <circle cx="294" cy="34" r="2" />
        <text class="rack-heading" x="116" y="38">SERVER RACK / 6U</text>
      </g>

      <g class="patch-panel" data-patch-panel="8-port">
        <rect x="110" y="48" width="180" height="38" rx="4" />
        <text class="rack-microcopy" x="116" y="57">PATCH / 8</text>
        <g
          v-for="port in ports"
          :key="port.number"
          class="rack-port"
          :class="{ active: port.number === activePortNumber }"
          :data-rack-port="port.number"
          :data-active="port.number === activePortNumber ? 'true' : 'false'"
        >
          <rect :x="port.x" :y="PORT_Y" :width="PORT_WIDTH" :height="PORT_HEIGHT" rx="2" />
          <circle :cx="port.x + PORT_WIDTH / 2" :cy="PORT_Y + PORT_HEIGHT / 2" r="1.7" />
        </g>
      </g>

      <g class="rack-units">
        <g
          v-for="(blade, index) in blades"
          :key="blade.number"
          class="rack-unit"
          :class="{ active: index === selectedBlade }"
          :data-rack-unit="blade.number"
          :data-active="index === selectedBlade ? 'true' : 'false'"
          :transform="bladeTransform(index)"
        >
          <rect x="113" :y="blade.y" width="174" :height="BLADE_HEIGHT" rx="3" />
          <circle cx="122" :cy="blade.y + BLADE_HEIGHT / 2" r="2.6" />
          <path class="rack-vents" :d="`M 134 ${blade.y + 7} H 226 M 134 ${blade.y + 11} H 226 M 134 ${blade.y + 15} H 226`" />
          <circle cx="269" :cy="blade.y + 8" r="1.7" />
          <circle cx="277" :cy="blade.y + 8" r="1.7" />
          <text class="unit-label" x="244" :y="blade.y + 16">{{ blade.number }}U</text>
        </g>
      </g>

      <path class="route-guide" :d="routePath" />
      <path
        class="route-active"
        data-route="ingress-port-blade-egress"
        :data-port="activePortNumber"
        :data-blade="selectedBlade + 1"
        :d="routePath"
        pathLength="1"
        :stroke-dasharray="routeDash"
      />
      <circle
        class="flow-pulse"
        :class="{ hidden: progress <= 0.001 }"
        :cx="flowPoint.x"
        :cy="flowPoint.y"
        r="4.4"
      />
      <circle
        class="flow-core"
        :class="{ hidden: progress <= 0.001 }"
        :cx="flowPoint.x"
        :cy="flowPoint.y"
        r="1.8"
      />

      <g class="endpoint ingress" data-node="ingress">
        <circle cx="31" cy="157" r="14" />
        <path d="M 22 157 H 38 M 34 152 L 39 157 L 34 162" />
        <text class="endpoint-label" x="31" y="183" text-anchor="middle">INGRESS</text>
      </g>
      <g class="endpoint egress" data-node="egress">
        <circle cx="369" cy="157" r="14" />
        <path d="M 360 157 H 376 M 372 152 L 377 157 L 372 162" />
        <text class="endpoint-label" x="369" y="183" text-anchor="middle">EGRESS</text>
      </g>

      <g class="route-readout">
        <text x="104" y="286">PORT {{ activePortNumber }}</text>
        <text x="200" y="286" text-anchor="middle">{{ selectedBlade + 1 }}U BLADE</text>
        <text x="296" y="286" text-anchor="end">{{ Math.round(progress * 100) }}%</text>
      </g>
    </svg>
  </div>
</template>

<style scoped>
.hairline-figure {
  position: relative;
  width: 100%;
  min-width: 0;
  min-height: 180px;
  aspect-ratio: 5 / 4;
  overflow: hidden;
  color: var(--hairline-edge, #4f8977);
  user-select: none;
  touch-action: pan-y;
  -webkit-tap-highlight-color: transparent;
}

.hairline-figure.is-interactive {
  cursor: crosshair;
}

.hairline-figure[data-theme='light'] {
  --hairline-plate: #edf5f0;
  --hairline-hi: #ffffff;
  --hairline-edge: #4f8977;
  --hairline-mid: #9bc3b4;
  --hairline-lo: #dceae2;
  --hairline-stroke: 1;
}

.hairline-figure[data-theme='dark'],
:global(.dark) .hairline-figure[data-theme='auto'],
:global([data-theme='dark']) .hairline-figure[data-theme='auto'] {
  --hairline-plate: #101814;
  --hairline-hi: #dff7ec;
  --hairline-edge: #79b69e;
  --hairline-mid: #4f7e6c;
  --hairline-lo: #21372e;
  --hairline-stroke: 1;
}

.server-rack-flow {
  display: block;
  width: 100%;
  height: 100%;
  overflow: visible;
  shape-rendering: geometricPrecision;
}

.figure-plate {
  fill: var(--hairline-plate, #edf5f0);
  stroke: none;
}

.rack-shadow path {
  fill: var(--hairline-lo, #dceae2);
  opacity: 0.72;
  stroke: none;
}

.rack-frame rect,
.patch-panel > rect,
.rack-unit > rect {
  fill: var(--hairline-plate, #edf5f0);
  stroke: var(--hairline-edge, #4f8977);
  stroke-width: var(--hairline-stroke, 1);
  vector-effect: non-scaling-stroke;
}

.rack-frame path,
.rack-frame circle,
.rack-unit circle,
.rack-unit path {
  fill: none;
  stroke: var(--hairline-mid, #9bc3b4);
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: var(--hairline-stroke, 1);
  vector-effect: non-scaling-stroke;
}

.rack-frame .rack-rail {
  stroke: var(--hairline-lo, #dceae2);
}

.rack-heading,
.rack-microcopy,
.unit-label,
.endpoint-label,
.route-readout text {
  fill: var(--hairline-edge, #4f8977);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 7px;
  font-weight: 650;
  letter-spacing: 0.08em;
  pointer-events: none;
}

.rack-microcopy,
.unit-label,
.route-readout text {
  fill: var(--hairline-mid, #9bc3b4);
  font-size: 6px;
}

.rack-port rect {
  fill: var(--hairline-lo, #dceae2);
  stroke: var(--hairline-mid, #9bc3b4);
  stroke-width: var(--hairline-stroke, 1);
  vector-effect: non-scaling-stroke;
}

.rack-port circle {
  fill: var(--hairline-mid, #9bc3b4);
}

.rack-port.active rect {
  fill: var(--hairline-hi, #ffffff);
  stroke: var(--hairline-edge, #4f8977);
}

.rack-port.active circle,
.rack-unit.active > circle:first-of-type,
.rack-unit.active > circle:nth-of-type(2) {
  fill: var(--hairline-edge, #4f8977);
  stroke: var(--hairline-edge, #4f8977);
}

.rack-unit {
  will-change: transform;
}

.rack-unit.active > rect {
  fill: var(--hairline-hi, #ffffff);
  stroke-width: calc(var(--hairline-stroke, 1) * 1.2);
}

.rack-unit.active .rack-vents {
  stroke: var(--hairline-edge, #4f8977);
}

.route-guide,
.route-active {
  fill: none;
  stroke-linecap: round;
  stroke-linejoin: round;
  vector-effect: non-scaling-stroke;
}

.route-guide {
  stroke: var(--hairline-lo, #dceae2);
  stroke-dasharray: 3 5;
  stroke-width: calc(var(--hairline-stroke, 1) * 1.15);
}

.route-active {
  stroke: var(--hairline-edge, #4f8977);
  stroke-width: calc(var(--hairline-stroke, 1) * 2);
}

.flow-pulse {
  fill: var(--hairline-hi, #ffffff);
  stroke: var(--hairline-edge, #4f8977);
  stroke-width: calc(var(--hairline-stroke, 1) * 1.2);
  vector-effect: non-scaling-stroke;
}

.flow-core {
  fill: var(--hairline-edge, #4f8977);
  stroke: none;
}

.flow-pulse.hidden,
.flow-core.hidden {
  opacity: 0;
}

.endpoint circle {
  fill: var(--hairline-plate, #edf5f0);
  stroke: var(--hairline-edge, #4f8977);
  stroke-width: calc(var(--hairline-stroke, 1) * 1.2);
  vector-effect: non-scaling-stroke;
}

.endpoint path {
  fill: none;
  stroke: var(--hairline-edge, #4f8977);
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: calc(var(--hairline-stroke, 1) * 1.2);
  vector-effect: non-scaling-stroke;
}

.hairline-figure[data-density='compact'] .rack-microcopy,
.hairline-figure[data-density='compact'] .unit-label,
.hairline-figure[data-density='compact'] .rack-vents,
.hairline-figure[data-density='compact'] .route-readout {
  display: none;
}

@media (prefers-color-scheme: dark) {
  .hairline-figure[data-theme='auto'] {
    --hairline-plate: #101814;
    --hairline-hi: #dff7ec;
    --hairline-edge: #79b69e;
    --hairline-mid: #4f7e6c;
    --hairline-lo: #21372e;
    --hairline-stroke: 1;
  }
}

@media (max-width: 580px), (pointer: coarse) {
  .rack-microcopy,
  .unit-label,
  .rack-vents,
  .route-readout,
  .endpoint-label {
    display: none;
  }

  .hairline-figure {
    min-height: 160px;
  }

  .rack-unit:not(.active) circle:nth-of-type(n + 2) {
    display: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .rack-unit {
    will-change: auto;
  }
}
</style>
