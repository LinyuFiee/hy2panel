<script setup>
import { Primitive } from 'reka-ui'
import { cn } from '@/lib/utils'
import { buttonVariants } from '.'

const props = defineProps({
  variant: { type: String, default: 'default' },
  size: { type: String, default: 'default' },
  as: { type: [String, Object], default: 'button' },
  asChild: { type: Boolean, default: false },
  type: { type: String, default: 'button' },
  disabled: { type: Boolean, default: false },
  class: { type: [String, Array, Object], default: undefined }
})
</script>

<template>
  <Primitive
    :as="as"
    :as-child="asChild"
    :type="as === 'button' && !asChild ? type : undefined"
    :disabled="disabled || undefined"
    :aria-disabled="disabled || undefined"
    :class="cn(buttonVariants({ variant, size }), props.class)"
  >
    <slot />
  </Primitive>
</template>
