<script setup>
import { cn } from '@/lib/utils'
const props = defineProps({ class: { type: [String, Array, Object], default: undefined } })
</script>

<template>
  <optgroup :class="cn(props.class)"><slot /></optgroup>
</template>
