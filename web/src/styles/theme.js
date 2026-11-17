import { useColorMode } from '@vueuse/core'

const SUPPORTED_MODES = new Set(['light', 'dark', 'auto'])

export const colorMode = useColorMode({
  selector: 'html',
  attribute: 'class',
  initialValue: 'auto',
  storageKey: 'hy2panel-theme',
  emitAuto: true,
  onChanged(mode, defaultHandler) {
    const resolved = mode === 'auto'
      ? (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
      : mode
    defaultHandler(resolved)
    document.documentElement.dataset.themePreference = mode
  }
})

export function setColorMode(mode = 'auto') {
  colorMode.value = SUPPORTED_MODES.has(mode) ? mode : 'auto'
}
