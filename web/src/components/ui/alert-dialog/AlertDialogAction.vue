<script setup>
import { AlertDialogAction } from 'reka-ui'
import { cn } from '@/lib/utils'
import { buttonVariants } from '@/components/ui/button'
const props = defineProps({ as: { type: [String, Object], default: 'button' }, asChild: { type: Boolean, default: false }, class: { type: [String, Array, Object], default: undefined } })
</script>
<template><AlertDialogAction :as="as" :as-child="asChild" :class="cn(buttonVariants(), props.class)"><slot /></AlertDialogAction></template>
