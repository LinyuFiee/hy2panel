<script setup>
import { computed } from 'vue'
import { ProgressIndicator, ProgressRoot } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps({
  modelValue: { type: Number, default: 0 },
  max: { type: Number, default: 100 },
  class: { type: [String, Array, Object], default: undefined }
})
const percentage = computed(() => {
  if (!Number.isFinite(props.modelValue) || !Number.isFinite(props.max) || props.max <= 0) return 0
  return Math.min(100, Math.max(0, props.modelValue / props.max * 100))
})
</script>

<template>
  <ProgressRoot :model-value="modelValue" :max="max" :class="cn('relative h-1.5 w-full overflow-hidden rounded-full bg-secondary', props.class)">
    <ProgressIndicator class="h-full w-full flex-1 bg-chart-1 transition-transform" :style="{ transform: `translateX(-${100 - percentage}%)` }" />
  </ProgressRoot>
</template>
