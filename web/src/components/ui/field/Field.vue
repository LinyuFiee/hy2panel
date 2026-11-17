<script setup>
import { cn } from '@/lib/utils'
const props = defineProps({
  orientation: { type: String, default: 'vertical' },
  class: { type: [String, Array, Object], default: undefined }
})
</script>

<template>
  <div
    role="group"
    data-slot="field"
    :data-orientation="orientation"
    :class="cn('group/field flex w-full gap-3 data-[orientation=horizontal]:items-center data-[orientation=horizontal]:justify-between data-[orientation=vertical]:flex-col', props.class)"
  >
    <slot />
  </div>
</template>
