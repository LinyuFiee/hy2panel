<script setup>
import { TooltipArrow, TooltipContent, TooltipPortal } from 'reka-ui'
import { cn } from '@/lib/utils'
const props = defineProps({
  side: { type: String, default: 'top' },
  sideOffset: { type: Number, default: 4 },
  align: { type: String, default: 'center' },
  alignOffset: { type: Number, default: 0 },
  avoidCollisions: { type: Boolean, default: true },
  forceMount: { type: Boolean, default: false },
  ariaLabel: { type: String, default: undefined },
  class: { type: [String, Array, Object], default: undefined },
  showArrow: { type: Boolean, default: false }
})
</script>

<template>
  <TooltipPortal>
    <TooltipContent
      :side="side"
      :side-offset="sideOffset"
      :align="align"
      :align-offset="alignOffset"
      :avoid-collisions="avoidCollisions"
      :force-mount="forceMount || undefined"
      :aria-label="ariaLabel"
      :class="cn('z-50 w-fit max-w-xs origin-[var(--reka-tooltip-content-transform-origin)] rounded-md bg-primary px-3 py-1.5 text-xs text-primary-foreground shadow-md data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=delayed-open]:animate-in data-[state=delayed-open]:fade-in-0 data-[state=delayed-open]:zoom-in-95 data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2', props.class)"
    >
      <slot />
      <TooltipArrow v-if="showArrow" class="fill-primary" />
    </TooltipContent>
  </TooltipPortal>
</template>
