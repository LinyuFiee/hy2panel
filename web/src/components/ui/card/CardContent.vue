<script setup>
import { cn } from '@/lib/utils'
const props = defineProps({ class: { type: [String, Array, Object], default: undefined } })
</script>

<template>
  <div data-slot="card-content" :class="cn('px-5 py-4', props.class)"><slot /></div>
</template>
