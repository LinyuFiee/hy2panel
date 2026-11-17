<script setup>
import { cn } from '@/lib/utils'
const props = defineProps({ class: { type: [String, Array, Object], default: undefined } })
</script>

<template>
  <legend data-slot="field-legend" :class="cn('mb-3 font-medium', props.class)"><slot /></legend>
</template>
