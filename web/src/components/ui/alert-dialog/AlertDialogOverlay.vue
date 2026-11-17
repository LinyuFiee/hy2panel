<script setup>
import { AlertDialogOverlay } from 'reka-ui'
import { cn } from '@/lib/utils'
const props = defineProps({ forceMount: { type: Boolean, default: false }, class: { type: [String, Array, Object], default: undefined } })
</script>
<template><AlertDialogOverlay :force-mount="forceMount || undefined" :class="cn('fixed inset-0 z-50 bg-black/50 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0', props.class)" /></template>
