<script setup>
import { DialogClose } from 'reka-ui'
const props = defineProps({ as: { type: [String, Object], default: 'button' }, asChild: { type: Boolean, default: false }, disabled: { type: Boolean, default: false } })
</script>
<template><DialogClose v-bind="props"><slot /></DialogClose></template>
