<script setup>
import { Label as LabelPrimitive } from 'reka-ui'
import { cn } from '@/lib/utils'
const props = defineProps({
  for: { type: String, default: undefined },
  as: { type: [String, Object], default: 'label' },
  asChild: { type: Boolean, default: false },
  class: { type: [String, Array, Object], default: undefined }
})
</script>

<template>
  <LabelPrimitive :for="props.for" :as="as" :as-child="asChild" :class="cn('flex items-center gap-2 text-sm leading-none font-medium select-none group-data-[disabled=true]:pointer-events-none group-data-[disabled=true]:opacity-50 peer-disabled:cursor-not-allowed peer-disabled:opacity-50', props.class)">
    <slot />
  </LabelPrimitive>
</template>
