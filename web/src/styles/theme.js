import { useColorMode } from '@vueuse/core'

const SUPPORTED_MODES = new Set(['light', 'dark', 'auto'])
const THEME_COLORS = { light: '#f7f7f6', dark: '#0b0b0c' }

function resolveColorMode(mode) {
  if (mode !== 'auto') return mode
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function syncThemeChrome(mode) {
  const resolved = resolveColorMode(mode)
  const themeColor = document.querySelector('meta[name="theme-color"]')
  if (themeColor) themeColor.setAttribute('content', THEME_COLORS[resolved])
  document.documentElement.dataset.themePreference = mode
  document.documentElement.style.colorScheme = resolved
}

export const colorMode = useColorMode({
  selector: 'html',
  attribute: 'class',
  initialValue: 'auto',
  storageKey: 'hy2panel-theme',
  emitAuto: true,
  onChanged(mode, defaultHandler) {
    const preference = SUPPORTED_MODES.has(mode) ? mode : 'auto'
    const resolved = resolveColorMode(preference)
    defaultHandler(resolved)
    syncThemeChrome(preference)
  }
})

export function setColorMode(mode = 'auto') {
  colorMode.value = SUPPORTED_MODES.has(mode) ? mode : 'auto'
}
