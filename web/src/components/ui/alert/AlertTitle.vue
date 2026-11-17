<script setup>
import { cn } from '@/lib/utils'
const props = defineProps({ class: { type: [String, Array, Object], default: undefined } })
</script>

<template>
  <h5 :class="cn('col-start-2 line-clamp-1 min-h-4 font-medium tracking-tight', props.class)">
    <slot />
  </h5>
</template>
