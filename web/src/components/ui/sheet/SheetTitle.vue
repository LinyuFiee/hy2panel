<script setup>
import { DialogTitle } from 'reka-ui'
import { cn } from '@/lib/utils'
const props = defineProps({ as: { type: [String, Object], default: 'h2' }, asChild: { type: Boolean, default: false }, class: { type: [String, Array, Object], default: undefined } })
</script>
<template><DialogTitle :as="as" :as-child="asChild" :class="cn('font-semibold text-foreground', props.class)"><slot /></DialogTitle></template>
