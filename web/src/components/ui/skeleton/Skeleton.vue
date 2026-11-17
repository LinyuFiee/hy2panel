<script setup>
import { cn } from '@/lib/utils'
const props = defineProps({ class: { type: [String, Array, Object], default: undefined } })
</script>

<template>
  <div aria-hidden="true" :class="cn('animate-pulse rounded-md bg-muted', props.class)" />
</template>
