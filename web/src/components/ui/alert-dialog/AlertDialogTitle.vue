<script setup>
import { AlertDialogTitle } from 'reka-ui'
import { cn } from '@/lib/utils'
const props = defineProps({ as: { type: [String, Object], default: 'h2' }, asChild: { type: Boolean, default: false }, class: { type: [String, Array, Object], default: undefined } })
</script>
<template><AlertDialogTitle :as="as" :as-child="asChild" :class="cn('text-lg font-semibold', props.class)"><slot /></AlertDialogTitle></template>
