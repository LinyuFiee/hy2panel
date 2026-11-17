<script setup>
import { CollapsibleTrigger } from 'reka-ui'
const props = defineProps({
  as: { type: [String, Object], default: 'button' },
  asChild: { type: Boolean, default: false }
})
</script>

<template>
  <CollapsibleTrigger v-bind="props"><slot /></CollapsibleTrigger>
</template>
