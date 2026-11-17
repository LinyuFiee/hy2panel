<script setup>
import { AlertDialogCancel } from 'reka-ui'
import { cn } from '@/lib/utils'
import { buttonVariants } from '@/components/ui/button'
const props = defineProps({ as: { type: [String, Object], default: 'button' }, asChild: { type: Boolean, default: false }, class: { type: [String, Array, Object], default: undefined } })
</script>
<template><AlertDialogCancel :as="as" :as-child="asChild" :class="cn(buttonVariants({ variant: 'outline' }), props.class)"><slot /></AlertDialogCancel></template>
