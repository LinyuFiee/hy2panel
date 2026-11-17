<script setup>
import { SwitchRoot, SwitchThumb, useForwardPropsEmits } from 'reka-ui'
import { computed } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps({
  modelValue: { type: [Boolean, String, Number], default: undefined },
  defaultValue: { type: [Boolean, String, Number], default: undefined },
  disabled: { type: Boolean, default: false },
  name: { type: String, default: undefined },
  required: { type: Boolean, default: false },
  value: { type: String, default: 'on' },
  id: { type: String, default: undefined },
  class: { type: [String, Array, Object], default: undefined }
})
const emit = defineEmits(['update:modelValue'])
const delegated = computed(() => {
  const { class: _class, ...rest } = props
  return rest
})
const forwarded = useForwardPropsEmits(delegated, emit)
</script>

<template>
  <SwitchRoot
    v-bind="forwarded"
    :class="cn('peer inline-flex h-5 w-9 shrink-0 items-center rounded-full border border-transparent shadow-xs transition-all outline-none data-[state=checked]:bg-primary data-[state=unchecked]:bg-input focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 dark:data-[state=unchecked]:bg-input/80', props.class)"
  >
    <SwitchThumb class="pointer-events-none block size-4 rounded-full bg-background ring-0 transition-transform data-[state=checked]:translate-x-4 data-[state=unchecked]:translate-x-0 dark:data-[state=checked]:bg-primary-foreground dark:data-[state=unchecked]:bg-foreground" />
  </SwitchRoot>
</template>
