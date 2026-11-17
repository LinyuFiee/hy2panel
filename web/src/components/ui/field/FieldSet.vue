<script setup>
import { cn } from '@/lib/utils'
const props = defineProps({ class: { type: [String, Array, Object], default: undefined } })
</script>

<template>
  <fieldset data-slot="field-set" :class="cn('flex flex-col gap-6', props.class)"><slot /></fieldset>
</template>
