<script setup>
import { cn } from '@/lib/utils'
const props = defineProps({ class: { type: [String, Array, Object], default: undefined } })
</script>
<template><div :class="cn('flex flex-col gap-1.5 p-4', props.class)"><slot /></div></template>
