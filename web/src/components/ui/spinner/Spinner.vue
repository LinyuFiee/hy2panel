<script setup>
import { LoaderCircle } from '@lucide/vue'
import { cn } from '@/lib/utils'
const props = defineProps({
  label: { type: String, default: '加载中' },
  class: { type: [String, Array, Object], default: undefined }
})
</script>

<template>
  <LoaderCircle role="status" :aria-label="label" :class="cn('size-4 animate-spin', props.class)" />
</template>
