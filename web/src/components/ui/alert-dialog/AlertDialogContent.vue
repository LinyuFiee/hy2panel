<script setup>
import { AlertDialogContent, AlertDialogPortal } from 'reka-ui'
import { cn } from '@/lib/utils'
import AlertDialogOverlay from './AlertDialogOverlay.vue'
const props = defineProps({ forceMount: { type: Boolean, default: false }, class: { type: [String, Array, Object], default: undefined } })
const emit = defineEmits(['escapeKeyDown', 'pointerDownOutside', 'focusOutside', 'interactOutside', 'openAutoFocus', 'closeAutoFocus'])
</script>

<template>
  <AlertDialogPortal :force-mount="forceMount || undefined">
    <AlertDialogOverlay :force-mount="forceMount" />
    <AlertDialogContent
      :force-mount="forceMount || undefined"
      :class="cn('fixed left-1/2 top-1/2 z-50 grid w-[calc(100%-2rem)] max-w-lg -translate-x-1/2 -translate-y-1/2 gap-4 rounded-lg border bg-background p-6 shadow-lg duration-200 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95', props.class)"
      @escape-key-down="emit('escapeKeyDown', $event)"
      @pointer-down-outside="emit('pointerDownOutside', $event)"
      @focus-outside="emit('focusOutside', $event)"
      @interact-outside="emit('interactOutside', $event)"
      @open-auto-focus="emit('openAutoFocus', $event)"
      @close-auto-focus="emit('closeAutoFocus', $event)"
    ><slot /></AlertDialogContent>
  </AlertDialogPortal>
</template>
