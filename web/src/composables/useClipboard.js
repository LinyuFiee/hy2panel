import { onBeforeUnmount, ref } from 'vue'

export function useClipboardFeedback(duration = 1600) {
  const copied = ref('')
  let clearTimer

  async function writeText(value) {
    try {
      await navigator.clipboard.writeText(value)
      return
    } catch {
    }

    const textarea = document.createElement('textarea')
    textarea.value = value
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    try {
      document.execCommand('copy')
    } finally {
      textarea.remove()
    }
  }

  async function copy(name, value) {
    if (!value) return false
    await writeText(value)
    copied.value = name
    window.clearTimeout(clearTimer)
    clearTimer = window.setTimeout(() => {
      if (copied.value === name) copied.value = ''
    }, duration)
    return true
  }

  function reset() {
    window.clearTimeout(clearTimer)
    copied.value = ''
  }

  onBeforeUnmount(reset)

  return { copied, copy, reset }
}
