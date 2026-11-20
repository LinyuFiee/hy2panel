<script setup>
import { TabsTrigger } from 'reka-ui'
import { cn } from '@/lib/utils'
const props = defineProps({
  value: { type: [String, Number], required: true },
  disabled: { type: Boolean, default: false },
  as: { type: [String, Object], default: 'button' },
  asChild: { type: Boolean, default: false },
  class: { type: [String, Array, Object], default: undefined }
})
</script>

<template>
  <TabsTrigger
    :value="value"
    :disabled="disabled"
    :as="as"
    :as-child="asChild"
    :class="cn('inline-flex h-[calc(100%-1px)] flex-1 items-center justify-center gap-1.5 rounded-md border border-transparent px-2 py-1 text-sm font-medium whitespace-nowrap text-foreground/65 transition-[color,background-color,box-shadow] outline-none data-[state=active]:border-border data-[state=active]:bg-card data-[state=active]:text-foreground data-[state=active]:shadow-[0_1px_3px_rgba(16,16,16,0.08)] focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/25 disabled:pointer-events-none disabled:opacity-50 dark:data-[state=active]:border-border dark:data-[state=active]:bg-card [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*=size-])]:size-4', props.class)"
  >
    <slot />
  </TabsTrigger>
</template>
