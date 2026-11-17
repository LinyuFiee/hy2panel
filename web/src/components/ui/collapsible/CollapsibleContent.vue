<script setup>
import { CollapsibleContent } from 'reka-ui'
import { cn } from '@/lib/utils'
const props = defineProps({
  forceMount: { type: Boolean, default: false },
  as: { type: [String, Object], default: 'div' },
  asChild: { type: Boolean, default: false },
  class: { type: [String, Array, Object], default: undefined }
})
</script>

<template>
  <CollapsibleContent :force-mount="forceMount || undefined" :as="as" :as-child="asChild" :class="cn('overflow-hidden data-[state=closed]:animate-collapsible-up data-[state=open]:animate-collapsible-down', props.class)">
    <slot />
  </CollapsibleContent>
</template>
