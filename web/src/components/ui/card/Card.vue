<script setup>
import { cn } from '@/lib/utils'
const props = defineProps({ class: { type: [String, Array, Object], default: undefined } })
</script>

<template>
  <div data-slot="card" :class="cn('flex min-w-0 flex-col gap-0 overflow-hidden rounded-xl border border-border bg-card text-card-foreground shadow-[var(--shadow-card)]', props.class)">
    <slot />
  </div>
</template>
