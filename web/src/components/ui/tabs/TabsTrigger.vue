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
    :class="cn('inline-flex h-[calc(100%-1px)] flex-1 items-center justify-center gap-1.5 rounded-md border border-transparent px-2 py-1 text-sm font-medium whitespace-nowrap text-foreground/70 transition-[color,box-shadow] outline-none data-[state=active]:border-input data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-sm focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50 dark:data-[state=active]:border-input dark:data-[state=active]:bg-input/30 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*=size-])]:size-4', props.class)"
  >
    <slot />
  </TabsTrigger>
</template>
