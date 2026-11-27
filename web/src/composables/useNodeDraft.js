import { reactive, ref, watch } from 'vue'

const DEFAULT_FORM = {
  public_host: '',
  reality: {
    enabled: true,
    port: 443,
    server_name: 'www.microsoft.com',
    uuid: '',
    private_key: '',
    public_key: '',
    short_id: ''
  },
  hysteria2: {
    enabled: false,
    port: 443,
    domain: '',
    password: '',
    up_mbps: 100,
    down_mbps: 100,
    bbr_profile: 'standard',
    obfs_type: '',
    obfs_password: ''
  }
}

function clone(value) {
  return JSON.parse(JSON.stringify(value))
}

function normalizedDraft(config) {
  return {
    public_host: config?.panel?.public_host || '',
    reality: { ...DEFAULT_FORM.reality, ...(config?.reality || {}) },
    hysteria2: { ...DEFAULT_FORM.hysteria2, ...(config?.hysteria2 || {}) }
  }
}

export function nodeConfigFingerprint(config) {
  return JSON.stringify(normalizedDraft(config))
}

// Shared by the draft and its remountable view, never serialized into the payload.
const toggleHistory = new WeakMap()

export function hysteriaToggleHistory(form) {
  if (!toggleHistory.has(form)) {
    toggleHistory.set(form, { passwords: new Map(), limitedRates: null, conflict: ref(false) })
  }
  return toggleHistory.get(form)
}

export function missingRate(hysteria, key) {
  return hysteria.enabled && String(hysteria[key] ?? '').trim() === ''
}

export function useNodeDraft() {
  const form = reactive(clone(DEFAULT_FORM))
  const dirty = ref(false)
  const history = hysteriaToggleHistory(form)
  const conflict = history.conflict
  let baseline = nodeConfigFingerprint(null)
  let lastIncoming = baseline
  let applying = false

  function clearToggleHistory() {
    history.passwords.clear()
    history.limitedRates = null
  }

  function replaceForm(next) {
    clearToggleHistory()
    applying = true
    form.public_host = next.public_host
    for (const key of Object.keys(form.reality)) delete form.reality[key]
    for (const key of Object.keys(form.hysteria2)) delete form.hysteria2[key]
    Object.assign(form.reality, next.reality)
    Object.assign(form.hysteria2, next.hysteria2)
    applying = false
  }

  function accept(config) {
    const next = normalizedDraft(config)
    replaceForm(next)
    baseline = JSON.stringify(next)
    lastIncoming = baseline
    dirty.value = false
    conflict.value = false
  }

  function reconcile(config) {
    if (!config) return 'ignored'
    const incoming = nodeConfigFingerprint(config)
    lastIncoming = incoming
    if (!dirty.value) {
      // An unchanged poll is not a replacement, even if a toggle round-trip
      // returned the effective configuration to its baseline.
      if (incoming !== baseline) accept(config)
      return 'synced'
    }
    conflict.value = incoming !== baseline
    return conflict.value ? 'conflict' : 'preserved'
  }

  watch(conflict, conflicted => {
    if (conflicted) clearToggleHistory()
  }, { flush: 'sync' })

  function payload() {
    const draft = clone(form)
    const hysteria = draft.hysteria2
    for (const [key, label] of [['up_mbps', '上传'], ['down_mbps', '下载']]) {
      if (missingRate(hysteria, key)) throw new Error(`请填写服务器${label}带宽；不限速请填 0。`)
      hysteria[key] = Number(hysteria[key] ?? 0)
    }
    draft.reality.port = Number(draft.reality.port ?? 0)
    hysteria.port = Number(hysteria.port ?? 0)
    if (!hysteria.obfs_type) hysteria.obfs_password = ''
    return draft
  }

  function reset() {
    replaceForm(clone(DEFAULT_FORM))
    baseline = nodeConfigFingerprint(null)
    lastIncoming = baseline
    dirty.value = false
    conflict.value = false
  }

  watch(form, () => {
    if (!applying) {
      dirty.value = JSON.stringify(form) !== baseline
      conflict.value = dirty.value && lastIncoming !== baseline
    }
  }, { deep: true })

  return {
    form,
    dirty,
    conflict,
    accept,
    reconcile,
    payload,
    reset
  }
}
