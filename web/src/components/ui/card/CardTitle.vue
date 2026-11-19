<script setup>
import { cn } from '@/lib/utils'
const props = defineProps({ class: { type: [String, Array, Object], default: undefined } })
</script>

<template>
  <h3 data-slot="card-title" :class="cn('m-0 text-sm leading-tight font-semibold tracking-[-0.015em]', props.class)"><slot /></h3>
</template>
