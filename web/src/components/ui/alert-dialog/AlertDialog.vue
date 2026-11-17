<script setup>
import { AlertDialogRoot, useForwardPropsEmits } from 'reka-ui'
const props = defineProps({ open: { type: Boolean, default: undefined }, defaultOpen: { type: Boolean, default: false } })
const emit = defineEmits(['update:open'])
const forwarded = useForwardPropsEmits(props, emit)
</script>
<template><AlertDialogRoot v-bind="forwarded"><slot /></AlertDialogRoot></template>
