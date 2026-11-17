<script setup>
import { Separator as SeparatorPrimitive } from 'reka-ui'
import { cn } from '@/lib/utils'
const props = defineProps({
  orientation: { type: String, default: 'horizontal' },
  decorative: { type: Boolean, default: true },
  class: { type: [String, Array, Object], default: undefined }
})
</script>

<template>
  <SeparatorPrimitive
    :orientation="orientation"
    :decorative="decorative"
    :class="cn('shrink-0 bg-border data-[orientation=horizontal]:h-px data-[orientation=horizontal]:w-full data-[orientation=vertical]:h-full data-[orientation=vertical]:w-px', props.class)"
  />
</template>
