<script setup>
import { DialogRoot, useForwardPropsEmits } from 'reka-ui'
const props = defineProps({
  open: { type: Boolean, default: undefined },
  defaultOpen: { type: Boolean, default: false },
  modal: { type: Boolean, default: true },
  unmountOnHide: { type: Boolean, default: true }
})
const emit = defineEmits(['update:open'])
const forwarded = useForwardPropsEmits(props, emit)
</script>

<template>
  <DialogRoot v-bind="forwarded"><slot /></DialogRoot>
</template>
