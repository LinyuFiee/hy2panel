<script setup>
import { TooltipTrigger } from 'reka-ui'
const props = defineProps({
  as: { type: [String, Object], default: 'button' },
  asChild: { type: Boolean, default: false }
})
</script>

<template>
  <TooltipTrigger v-bind="props"><slot /></TooltipTrigger>
</template>
