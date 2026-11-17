<script setup>
import { cn } from '@/lib/utils'
const props = defineProps({ class: { type: [String, Array, Object], default: undefined } })
</script>

<template>
  <div data-slot="card" :class="cn('flex flex-col gap-6 rounded-xl border bg-card py-6 text-card-foreground shadow-sm', props.class)">
    <slot />
  </div>
</template>
