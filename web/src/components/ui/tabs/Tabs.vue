<script setup>
import { TabsRoot, useForwardPropsEmits } from 'reka-ui'
const props = defineProps({
  modelValue: { type: [String, Number], default: undefined },
  defaultValue: { type: [String, Number], default: undefined },
  orientation: { type: String, default: 'horizontal' },
  activationMode: { type: String, default: 'automatic' },
  dir: { type: String, default: undefined },
  unmountOnHide: { type: Boolean, default: true },
  as: { type: [String, Object], default: 'div' },
  asChild: { type: Boolean, default: false }
})
const emit = defineEmits(['update:modelValue'])
const forwarded = useForwardPropsEmits(props, emit)
</script>

<template>
  <TabsRoot v-bind="forwarded"><slot /></TabsRoot>
</template>
