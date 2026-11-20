<script setup>
import { computed } from 'vue'
import { Laptop, Moon, Sun } from '@lucide/vue'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { colorMode, setColorMode } from '@/styles/theme'

const props = defineProps({
  compact: { type: Boolean, default: false },
  label: { type: String, default: '外观主题' }
})

const options = [
  { value: 'light', label: '浅色', icon: Sun },
  { value: 'dark', label: '深色', icon: Moon },
  { value: 'auto', label: '跟随系统', icon: Laptop }
]

const selectedMode = computed({
  get: () => colorMode.value || 'auto',
  set: value => {
    if (value) setColorMode(value)
  }
})
</script>

<template>
  <div class="theme-toggle" :class="{ 'is-compact': compact }">
    <span v-if="!compact" class="theme-toggle-label">{{ label }}</span>
    <ToggleGroup
      v-model="selectedMode"
      type="single"
      variant="outline"
      size="sm"
      class="theme-toggle-group"
      :aria-label="label"
    >
      <ToggleGroupItem
        v-for="option in options"
        :key="option.value"
        :value="option.value"
        :aria-label="option.label"
        :title="option.label"
        class="theme-toggle-item"
      >
        <component :is="option.icon" :size="15" />
        <span v-if="!compact">{{ option.label }}</span>
        <span v-else class="sr-only">{{ option.label }}</span>
      </ToggleGroupItem>
    </ToggleGroup>
  </div>
</template>

<style scoped>
/* Segmented-control visuals live in globals.css: reka's ToggleGroupItem does
   not inherit this component's scope attribute, so scoped rules never hit it. */
.theme-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  min-width: 0;
}

.theme-toggle-label {
  color: var(--muted-foreground);
  font-size: 0.75rem;
  font-weight: 600;
  white-space: nowrap;
}

@media (max-width: 640px) {
  .theme-toggle:not(.is-compact) {
    align-items: flex-start;
    flex-direction: column;
  }

  .theme-toggle:not(.is-compact) .theme-toggle-group {
    width: 100%;
  }

  .theme-toggle:not(.is-compact) .theme-toggle-item {
    flex: 1;
  }
}
</style>
