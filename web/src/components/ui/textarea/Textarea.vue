<script setup>
import { useAttrs } from 'vue'
import { cn } from '@/lib/utils'

defineOptions({ inheritAttrs: false })
const props = defineProps({
  modelValue: { type: String, default: undefined },
  defaultValue: { type: String, default: undefined },
  class: { type: [String, Array, Object], default: undefined }
})
const emit = defineEmits(['update:modelValue'])
const attrs = useAttrs()
</script>

<template>
  <textarea
    v-bind="attrs"
    :value="modelValue ?? defaultValue"
    :class="cn('flex field-sizing-content min-h-16 w-full rounded-md border border-input bg-card px-3 py-2 text-base shadow-none transition-[color,border-color,box-shadow] outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50 md:text-sm dark:bg-card aria-invalid:border-destructive aria-invalid:ring-[3px] aria-invalid:ring-destructive/15 focus-visible:border-foreground/30 focus-visible:ring-[3px] focus-visible:ring-ring/20', props.class)"
    @input="emit('update:modelValue', $event.target.value)"
  />
</template>
