<script setup>
import { DialogDescription } from 'reka-ui'
import { cn } from '@/lib/utils'
const props = defineProps({ as: { type: [String, Object], default: 'p' }, asChild: { type: Boolean, default: false }, class: { type: [String, Array, Object], default: undefined } })
</script>
<template><DialogDescription :as="as" :as-child="asChild" :class="cn('text-sm text-muted-foreground', props.class)"><slot /></DialogDescription></template>
