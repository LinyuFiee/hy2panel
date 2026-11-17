<script setup>
import { computed } from 'vue'
import { ToggleGroupRoot, useForwardPropsEmits } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps({
  modelValue: { type: [String, Number, Array], default: undefined },
  defaultValue: { type: [String, Number, Array], default: undefined },
  type: { type: String, default: 'single' },
  disabled: { type: Boolean, default: false },
  rovingFocus: { type: Boolean, default: true },
  orientation: { type: String, default: 'horizontal' },
  loop: { type: Boolean, default: true },
  name: { type: String, default: undefined },
  required: { type: Boolean, default: false },
  variant: { type: String, default: 'default' },
  size: { type: String, default: 'default' },
  class: { type: [String, Array, Object], default: undefined }
})
const emit = defineEmits(['update:modelValue'])
const delegated = computed(() => {
  const { class: _class, variant: _variant, size: _size, ...rest } = props
  return rest
})
const forwarded = useForwardPropsEmits(delegated, emit)
</script>

<template>
  <ToggleGroupRoot
    v-bind="forwarded"
    :data-variant="variant"
    :data-size="size"
    :class="cn('group/toggle-group flex w-fit items-center rounded-md data-[orientation=vertical]:flex-col', props.class)"
  >
    <slot />
  </ToggleGroupRoot>
</template>
