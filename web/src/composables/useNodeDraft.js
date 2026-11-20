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

export function useNodeDraft() {
  const form = reactive(clone(DEFAULT_FORM))
  const dirty = ref(false)
  const conflict = ref(false)
  let baseline = nodeConfigFingerprint(null)
  let lastIncoming = baseline
  let applying = false

  function replaceForm(next) {
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
      accept(config)
      return 'synced'
    }
    conflict.value = incoming !== baseline
    return conflict.value ? 'conflict' : 'preserved'
  }

function toNumber(value) {
  const text = String(value ?? '').trim()
  return text === '' ? 0 : Number(text)
}

function payload() {
  const draft = clone(form)
  draft.reality.port = toNumber(draft.reality.port)
  draft.hysteria2.port = toNumber(draft.hysteria2.port)
  draft.hysteria2.up_mbps = toNumber(draft.hysteria2.up_mbps)
  draft.hysteria2.down_mbps = toNumber(draft.hysteria2.down_mbps)
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
