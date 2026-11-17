<script setup>
import { AlertDialogTrigger } from 'reka-ui'
const props = defineProps({ as: { type: [String, Object], default: 'button' }, asChild: { type: Boolean, default: false } })
</script>
<template><AlertDialogTrigger v-bind="props"><slot /></AlertDialogTrigger></template>
