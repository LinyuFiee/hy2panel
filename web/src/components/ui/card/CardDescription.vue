<script setup>
import { cn } from '@/lib/utils'
const props = defineProps({ class: { type: [String, Array, Object], default: undefined } })
</script>

<template>
  <p data-slot="card-description" :class="cn('text-sm text-muted-foreground', props.class)"><slot /></p>
</template>
