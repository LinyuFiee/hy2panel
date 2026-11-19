<script setup>
import { cn } from '@/lib/utils'
const props = defineProps({ class: { type: [String, Array, Object], default: undefined } })
</script>

<template>
  <div data-slot="card-footer" :class="cn('flex items-center border-t border-border px-5 py-3.5', props.class)"><slot /></div>
</template>
