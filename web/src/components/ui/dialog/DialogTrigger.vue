<script setup>
import { DialogTrigger } from 'reka-ui'
const props = defineProps({ as: { type: [String, Object], default: 'button' }, asChild: { type: Boolean, default: false } })
</script>
<template><DialogTrigger v-bind="props"><slot /></DialogTrigger></template>
