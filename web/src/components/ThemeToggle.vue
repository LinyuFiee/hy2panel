<script setup>
import { computed } from 'vue'
import { Laptop, Moon, Sun } from '@lucide/vue'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
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
    <TooltipProvider :delay-duration="250">
      <ToggleGroup
        v-model="selectedMode"
        type="single"
        variant="outline"
        size="sm"
        class="theme-toggle-group"
        :aria-label="label"
      >
        <Tooltip v-for="option in options" :key="option.value">
          <TooltipTrigger as-child>
            <ToggleGroupItem
              :value="option.value"
              :aria-label="option.label"
              :title="option.label"
              class="theme-toggle-item"
            >
              <component :is="option.icon" :size="15" />
              <span v-if="!compact">{{ option.label }}</span>
              <span v-else class="sr-only">{{ option.label }}</span>
            </ToggleGroupItem>
          </TooltipTrigger>
          <TooltipContent v-if="compact" side="bottom">{{ option.label }}</TooltipContent>
        </Tooltip>
      </ToggleGroup>
    </TooltipProvider>
  </div>
</template>

<style scoped>
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
  font-weight: 650;
  white-space: nowrap;
}

.theme-toggle-group {
  width: auto;
  padding: 0.1875rem;
  border: 1px solid var(--border);
  border-radius: 0.65rem;
  background: color-mix(in oklab, var(--background) 88%, transparent);
  box-shadow: 0 1px 2px color-mix(in oklab, var(--foreground) 7%, transparent);
}

.theme-toggle-item {
  min-width: 2rem;
  height: 1.875rem;
  gap: 0.35rem;
  padding-inline: 0.55rem;
  border: 0 !important;
  border-radius: 0.45rem !important;
  color: var(--muted-foreground);
  box-shadow: none;
  font-size: 0.7rem;
}

.theme-toggle-item:hover {
  color: var(--foreground);
}

.theme-toggle-item[data-state='on'] {
  background: var(--card);
  color: var(--foreground);
  box-shadow: 0 1px 4px color-mix(in oklab, var(--foreground) 12%, transparent);
}

.is-compact .theme-toggle-group {
  padding: 0.125rem;
}

.is-compact .theme-toggle-item {
  width: 1.875rem;
  min-width: 1.875rem;
  height: 1.75rem;
  padding: 0;
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
