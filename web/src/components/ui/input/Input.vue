<script setup>
import { useAttrs } from 'vue'
import { cn } from '@/lib/utils'

defineOptions({ inheritAttrs: false })
const props = defineProps({
  modelValue: { type: [String, Number], default: undefined },
  defaultValue: { type: [String, Number], default: undefined },
  class: { type: [String, Array, Object], default: undefined }
})
const emit = defineEmits(['update:modelValue'])
const attrs = useAttrs()

function onInput(event) {
  emit('update:modelValue', event.target.value)
}
</script>

<template>
  <input
    v-bind="attrs"
    :value="modelValue ?? defaultValue"
    :class="cn('flex h-9 w-full min-w-0 rounded-md border border-input bg-card px-3 py-1 text-base shadow-none transition-[color,border-color,box-shadow] outline-none file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-muted-foreground selection:bg-primary selection:text-primary-foreground disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 md:text-sm dark:bg-card aria-invalid:border-destructive aria-invalid:ring-[3px] aria-invalid:ring-destructive/15 focus-visible:border-foreground/30 focus-visible:ring-[3px] focus-visible:ring-ring/20', props.class)"
    @input="onInput"
  />
</template>
