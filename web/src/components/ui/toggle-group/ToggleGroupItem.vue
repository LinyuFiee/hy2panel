<script setup>
import { ToggleGroupItem as ToggleGroupItemPrimitive } from 'reka-ui'
import { cn } from '@/lib/utils'
import { toggleGroupItemVariants } from '.'

const props = defineProps({
  value: { type: [String, Number], required: true },
  disabled: { type: Boolean, default: false },
  as: { type: [String, Object], default: 'button' },
  asChild: { type: Boolean, default: false },
  variant: { type: String, default: undefined },
  size: { type: String, default: undefined },
  class: { type: [String, Array, Object], default: undefined }
})
</script>

<template>
  <ToggleGroupItemPrimitive
    :value="value"
    :disabled="disabled"
    :as="as"
    :as-child="asChild"
    :class="cn(toggleGroupItemVariants({ variant: variant || 'default', size: size || 'default' }), 'group-data-[variant=outline]/toggle-group:border-l-0 first:group-data-[variant=outline]/toggle-group:border-l group-data-[orientation=vertical]/toggle-group:w-full group-data-[orientation=vertical]/toggle-group:rounded-none group-data-[orientation=vertical]/toggle-group:border-t-0 first:group-data-[orientation=vertical]/toggle-group:rounded-t-md first:group-data-[orientation=vertical]/toggle-group:border-t last:group-data-[orientation=vertical]/toggle-group:rounded-b-md group-data-[orientation=horizontal]/toggle-group:rounded-none group-data-[orientation=horizontal]/toggle-group:first:rounded-l-md group-data-[orientation=horizontal]/toggle-group:last:rounded-r-md', props.class)"
  >
    <slot />
  </ToggleGroupItemPrimitive>
</template>
