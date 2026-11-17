<script setup>
import { computed } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps({
  errors: { type: Array, default: () => [] },
  class: { type: [String, Array, Object], default: undefined }
})

const message = computed(() => props.errors.map(error => typeof error === 'string' ? error : error?.message).filter(Boolean).join(', '))
</script>

<template>
  <div v-if="$slots.default || message" role="alert" data-slot="field-error" :class="cn('text-sm font-normal text-destructive', props.class)">
    <slot>{{ message }}</slot>
  </div>
</template>
