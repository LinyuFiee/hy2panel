<script setup>
import { computed, useAttrs } from 'vue'
import { ChevronDown } from '@lucide/vue'
import { cn } from '@/lib/utils'

defineOptions({ inheritAttrs: false })
const props = defineProps({
  modelValue: { type: [String, Number], default: undefined },
  defaultValue: { type: [String, Number], default: undefined },
  placeholder: { type: String, default: undefined },
  size: { type: String, default: 'default' },
  class: { type: [String, Array, Object], default: undefined }
})
const emit = defineEmits(['update:modelValue', 'change'])
const attrs = useAttrs()
const value = computed(() => props.modelValue ?? props.defaultValue ?? '')

function onChange(event) {
  emit('update:modelValue', event.target.value)
  emit('change', event)
}
</script>

<template>
  <div data-slot="native-select-wrapper" class="relative w-fit has-[select:disabled]:opacity-50">
    <select
      v-bind="attrs"
      :value="value"
      data-slot="native-select"
      :class="cn('h-9 w-full min-w-24 appearance-none rounded-md border border-input bg-transparent py-1 pl-3 pr-8 text-sm shadow-xs transition-[color,box-shadow] outline-none disabled:pointer-events-none disabled:cursor-not-allowed data-[size=sm]:h-8 data-[size=sm]:py-0 dark:bg-input/30 aria-invalid:border-destructive aria-invalid:ring-[3px] aria-invalid:ring-destructive/20 focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50', props.class)"
      :data-size="size"
      @change="onChange"
    >
      <option v-if="placeholder" value="" disabled>{{ placeholder }}</option>
      <slot />
    </select>
    <ChevronDown aria-hidden="true" class="pointer-events-none absolute right-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
  </div>
</template>
