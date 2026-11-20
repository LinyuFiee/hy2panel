<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { padlock, plot, relay } from '@lucasmarkes/hairline'
import { cn } from '@/lib/utils'

const figures = { padlock, relay, plot }

const props = defineProps({
  name: {
    type: String,
    default: undefined,
    validator: value => value === undefined || ['padlock', 'relay', 'plot'].includes(value)
  },
  figure: {
    type: String,
    default: undefined,
    validator: value => value === undefined || ['padlock', 'relay', 'plot'].includes(value)
  },
  play: { type: Boolean, default: false },
  intensity: { type: Number, default: 0.5 },
  theme: {
    type: String,
    default: 'auto',
    validator: value => ['auto', 'light', 'dark'].includes(value)
  },
  label: { type: String, default: undefined },
  class: { type: [String, Array, Object], default: undefined }
})

const host = ref(null)
let instance = null

function selectedFigure() {
  return props.figure || props.name
}

function options() {
  return {
    play: props.play,
    intensity: props.intensity,
    theme: props.theme,
    label: props.label
  }
}

function destroyFigure() {
  instance?.destroy()
  instance = null
}

function mountFigure() {
  destroyFigure()
  const createFigure = figures[selectedFigure()]
  if (host.value && createFigure) instance = createFigure(host.value, options())
}

watch(() => [props.figure, props.name], mountFigure, { flush: 'post' })
watch(
  () => [props.play, props.intensity, props.theme, props.label],
  () => instance?.update(options()),
  { flush: 'post' }
)

onMounted(mountFigure)
onBeforeUnmount(destroyFigure)
</script>

<template>
  <div ref="host" :class="cn('hairline-figure', props.class)"></div>
</template>

<style scoped>
.hairline-figure {
  width: 100%;
  min-width: 0;
  aspect-ratio: 5 / 4;
}
</style>
