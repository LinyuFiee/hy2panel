<script setup>
import { TabsList } from 'reka-ui'
import { cn } from '@/lib/utils'
const props = defineProps({
  loop: { type: Boolean, default: true },
  as: { type: [String, Object], default: 'div' },
  asChild: { type: Boolean, default: false },
  class: { type: [String, Array, Object], default: undefined }
})
</script>

<template>
  <TabsList :loop="loop" :as="as" :as-child="asChild" :class="cn('inline-flex h-9 w-fit items-center justify-center rounded-lg border border-border bg-muted p-[2px] text-muted-foreground', props.class)">
    <slot />
  </TabsList>
</template>
