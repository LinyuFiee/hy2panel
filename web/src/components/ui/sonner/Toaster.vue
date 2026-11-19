<script setup>
import { computed } from 'vue'
import { Toaster as Sonner } from 'vue-sonner'
import { colorMode } from '@/styles/theme'

const props = defineProps({
  theme: { type: String, default: undefined },
  position: { type: String, default: 'bottom-right' },
  richColors: { type: Boolean, default: false },
  closeButton: { type: Boolean, default: true },
  expand: { type: Boolean, default: false },
  duration: { type: Number, default: 4000 },
  visibleToasts: { type: Number, default: 4 },
  class: { type: String, default: 'toaster group' }
})

const resolvedTheme = computed(() => {
  const preference = props.theme || colorMode.value || 'auto'
  return preference === 'auto' ? 'system' : preference
})
</script>

<template>
  <Sonner
    :theme="resolvedTheme"
    :position="position"
    :rich-colors="richColors"
    :close-button="closeButton"
    :expand="expand"
    :duration="duration"
    :visible-toasts="visibleToasts"
    :class="props.class"
    :toast-options="{
      classes: {
        toast: 'group toast border-border bg-popover text-popover-foreground shadow-lg',
        description: 'text-muted-foreground',
        actionButton: 'bg-primary text-primary-foreground',
        cancelButton: 'bg-muted text-muted-foreground'
      }
    }"
  />
</template>
