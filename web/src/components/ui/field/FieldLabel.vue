<script setup>
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'
const props = defineProps({
  for: { type: String, default: undefined },
  class: { type: [String, Array, Object], default: undefined }
})
</script>

<template>
  <Label data-slot="field-label" :for="props.for" :class="cn('w-fit group-data-[disabled=true]/field:opacity-50', props.class)">
    <slot />
  </Label>
</template>
