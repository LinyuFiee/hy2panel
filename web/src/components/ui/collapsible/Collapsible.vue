<script setup>
import { CollapsibleRoot, useForwardPropsEmits } from 'reka-ui'
const props = defineProps({
  open: { type: Boolean, default: undefined },
  defaultOpen: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false },
  unmountOnHide: { type: Boolean, default: true },
  as: { type: [String, Object], default: 'div' },
  asChild: { type: Boolean, default: false }
})
const emit = defineEmits(['update:open'])
const forwarded = useForwardPropsEmits(props, emit)
</script>

<template>
  <CollapsibleRoot v-bind="forwarded"><slot /></CollapsibleRoot>
</template>
