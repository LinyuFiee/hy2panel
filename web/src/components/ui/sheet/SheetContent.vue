<script setup>
import { DialogClose, DialogContent, DialogPortal } from 'reka-ui'
import { X } from '@lucide/vue'
import { cn } from '@/lib/utils'
import { sheetVariants } from '.'
import SheetOverlay from './SheetOverlay.vue'
const props = defineProps({
  side: { type: String, default: 'right' },
  forceMount: { type: Boolean, default: false },
  showCloseButton: { type: Boolean, default: true },
  class: { type: [String, Array, Object], default: undefined }
})
const emit = defineEmits(['escapeKeyDown', 'pointerDownOutside', 'focusOutside', 'interactOutside', 'openAutoFocus', 'closeAutoFocus'])
</script>

<template>
  <DialogPortal :force-mount="forceMount || undefined">
    <SheetOverlay :force-mount="forceMount" />
    <DialogContent
      :force-mount="forceMount || undefined"
      :class="cn(sheetVariants({ side }), props.class)"
      @escape-key-down="emit('escapeKeyDown', $event)"
      @pointer-down-outside="emit('pointerDownOutside', $event)"
      @focus-outside="emit('focusOutside', $event)"
      @interact-outside="emit('interactOutside', $event)"
      @open-auto-focus="emit('openAutoFocus', $event)"
      @close-auto-focus="emit('closeAutoFocus', $event)"
    >
      <slot />
      <DialogClose v-if="showCloseButton" class="absolute right-4 top-4 rounded-xs opacity-70 ring-offset-background transition-opacity hover:opacity-100 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:pointer-events-none">
        <X class="size-4" />
        <span class="sr-only">关闭</span>
      </DialogClose>
    </DialogContent>
  </DialogPortal>
</template>
