<script setup>
import { cn } from '@/lib/utils'
const props = defineProps({ class: { type: [String, Array, Object], default: undefined } })
</script>

<template>
  <p data-slot="field-description" :class="cn('text-sm leading-normal text-muted-foreground', props.class)"><slot /></p>
</template>
