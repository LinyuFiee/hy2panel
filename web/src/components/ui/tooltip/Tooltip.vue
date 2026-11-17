<script setup>
import { TooltipRoot, useForwardPropsEmits } from 'reka-ui'
const props = defineProps({
  open: { type: Boolean, default: undefined },
  defaultOpen: { type: Boolean, default: false },
  delayDuration: { type: Number, default: undefined },
  disableHoverableContent: { type: Boolean, default: undefined },
  disabled: { type: Boolean, default: false }
})
const emit = defineEmits(['update:open'])
const forwarded = useForwardPropsEmits(props, emit)
</script>

<template>
  <TooltipRoot v-bind="forwarded"><slot /></TooltipRoot>
</template>
