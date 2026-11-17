<script setup>
import { TabsContent } from 'reka-ui'
import { cn } from '@/lib/utils'
const props = defineProps({
  value: { type: [String, Number], required: true },
  forceMount: { type: Boolean, default: false },
  as: { type: [String, Object], default: 'div' },
  asChild: { type: Boolean, default: false },
  class: { type: [String, Array, Object], default: undefined }
})
</script>

<template>
  <TabsContent :value="value" :force-mount="forceMount || undefined" :as="as" :as-child="asChild" :class="cn('flex-1 outline-none', props.class)">
    <slot />
  </TabsContent>
</template>
