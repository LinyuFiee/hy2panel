<script setup>
import { cn } from '@/lib/utils'
const props = defineProps({
  orientation: { type: String, default: 'vertical' },
  class: { type: [String, Array, Object], default: undefined }
})
</script>

<template>
  <div data-slot="field-group" :data-orientation="orientation" :class="cn('flex w-full gap-5 data-[orientation=horizontal]:flex-row data-[orientation=vertical]:flex-col', props.class)">
    <slot />
  </div>
</template>
