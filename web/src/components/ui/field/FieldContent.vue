<script setup>
import { cn } from '@/lib/utils'
const props = defineProps({ class: { type: [String, Array, Object], default: undefined } })
</script>

<template>
  <div data-slot="field-content" :class="cn('flex flex-1 flex-col gap-1.5 leading-snug', props.class)"><slot /></div>
</template>
