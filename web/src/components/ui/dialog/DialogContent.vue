<script setup>
import { DialogClose, DialogContent, DialogPortal } from 'reka-ui'
import { X } from '@lucide/vue'
import { cn } from '@/lib/utils'
import DialogOverlay from './DialogOverlay.vue'
const props = defineProps({
  forceMount: { type: Boolean, default: false },
  showCloseButton: { type: Boolean, default: true },
  class: { type: [String, Array, Object], default: undefined }
})
const emit = defineEmits(['escapeKeyDown', 'pointerDownOutside', 'focusOutside', 'interactOutside', 'openAutoFocus', 'closeAutoFocus'])
</script>

<template>
  <DialogPortal :force-mount="forceMount || undefined">
    <DialogOverlay :force-mount="forceMount" />
    <DialogContent
      :force-mount="forceMount || undefined"
      :class="cn('fixed left-1/2 top-1/2 z-50 grid w-[calc(100%-2rem)] max-w-lg -translate-x-1/2 -translate-y-1/2 gap-4 rounded-lg border bg-background p-6 shadow-lg duration-200 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95', props.class)"
      @escape-key-down="emit('escapeKeyDown', $event)"
      @pointer-down-outside="emit('pointerDownOutside', $event)"
      @focus-outside="emit('focusOutside', $event)"
      @interact-outside="emit('interactOutside', $event)"
      @open-auto-focus="emit('openAutoFocus', $event)"
      @close-auto-focus="emit('closeAutoFocus', $event)"
    >
      <slot />
      <DialogClose v-if="showCloseButton" class="absolute right-4 top-4 rounded-xs opacity-70 ring-offset-background transition-opacity hover:opacity-100 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:pointer-events-none data-[state=open]:bg-accent">
        <X class="size-4" />
        <span class="sr-only">关闭</span>
      </DialogClose>
    </DialogContent>
  </DialogPortal>
</template>
