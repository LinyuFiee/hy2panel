// Theme bootstrap: runs before the app bundle to avoid a light/dark flash.
// Loaded as an external same-origin script so it passes CSP script-src 'self'.
(() => {
  try {
    const mode = localStorage.getItem('hy2panel-theme') || 'auto'
    const dark = mode === 'dark' || (mode === 'auto' && matchMedia('(prefers-color-scheme: dark)').matches)
    const root = document.documentElement
    root.classList.toggle('dark', dark)
    root.classList.toggle('light', !dark)
    root.dataset.themePreference = mode
    root.style.colorScheme = dark ? 'dark' : 'light'
    const themeColor = document.querySelector('meta[name="theme-color"]')
    if (themeColor) themeColor.setAttribute('content', dark ? '#0b0b0c' : '#f7f7f6')
  } catch {}
})()
