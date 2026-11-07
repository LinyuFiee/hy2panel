<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import QRCode from 'qrcode'

const view = ref('loading')
const password = ref('')
const error = ref('')
const notice = ref('')
const busy = ref(false)
const copied = ref('')
const state = ref(null)
const qr = reactive({ reality: '', hysteria2: '' })
const form = reactive({
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
    down_mbps: 100
  }
})

const panelAddress = computed(() => state.value ? `https://${state.value.config.panel.domain}:8443` : '')
const serviceClass = computed(() => state.value?.status.sing_box === 'active' ? 'online' : 'offline')

async function request(path, options = {}) {
  const headers = { 'X-Requested-With': 'fetch', ...(options.headers || {}) }
  if (options.body) headers['Content-Type'] = 'application/json'
  const response = await fetch(path, { credentials: 'same-origin', ...options, headers })
  const payload = await response.json().catch(() => ({}))
  if (response.status === 401 && path !== '/api/login') {
    view.value = 'login'
    state.value = null
  }
  if (!response.ok) {
    const failure = new Error(payload.error || `请求失败 (${response.status})`)
    failure.status = response.status
    throw failure
  }
  return payload
}

function applyState(payload) {
  state.value = payload
  form.public_host = payload.config.panel.public_host
  Object.assign(form.reality, payload.config.reality)
  Object.assign(form.hysteria2, payload.config.hysteria2)
  updateQR('reality', payload.links.reality)
  updateQR('hysteria2', payload.links.hysteria2)
}

async function updateQR(name, value) {
  qr[name] = value ? await QRCode.toDataURL(value, { width: 360, margin: 1 }) : ''
}

async function loadState() {
  error.value = ''
  try {
    applyState(await request('/api/state'))
    view.value = 'dashboard'
  } catch (failure) {
    if (failure.status !== 401) error.value = failure.message
    if (view.value !== 'login') view.value = 'login'
  }
}

async function login() {
  busy.value = true
  error.value = ''
  try {
    await request('/api/login', { method: 'POST', body: JSON.stringify({ password: password.value }) })
    password.value = ''
    await loadState()
  } catch (failure) {
    error.value = failure.message
  } finally {
    busy.value = false
  }
}

async function logout() {
  busy.value = true
  try {
    await request('/api/logout', { method: 'POST', body: '{}' })
  } catch {
  } finally {
    state.value = null
    view.value = 'login'
    busy.value = false
  }
}

async function save() {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    const payload = await request('/api/config', {
      method: 'POST',
      body: JSON.stringify({
        public_host: form.public_host,
        reality: form.reality,
        hysteria2: form.hysteria2
      })
    })
    applyState(payload)
    notice.value = '配置已通过校验并应用。'
  } catch (failure) {
    error.value = failure.message
  } finally {
    busy.value = false
  }
}

async function copyLink(name) {
  const value = state.value?.links[name]
  if (!value) return
  await navigator.clipboard.writeText(value)
  copied.value = name
  window.setTimeout(() => {
    if (copied.value === name) copied.value = ''
  }, 1600)
}

onMounted(loadState)
</script>

<template>
  <main v-if="view === 'loading'" class="login-shell">
    <p class="muted">正在连接面板…</p>
  </main>

  <main v-else-if="view === 'login'" class="login-shell">
    <section class="login-card">
      <div class="brand">
        <div class="brand-mark">H2</div>
        <div>
          <h1>节点控制台</h1>
          <p class="muted">Reality 与 Hysteria2 管理</p>
        </div>
      </div>
      <form style="margin-top: 28px" @submit.prevent="login">
        <div v-if="error" class="alert error">{{ error }}</div>
        <div class="field">
          <label for="password">管理员密码</label>
          <input id="password" v-model="password" type="password" autocomplete="current-password" required autofocus />
        </div>
        <button class="primary full" type="submit" :disabled="busy">{{ busy ? '登录中…' : '安全登录' }}</button>
      </form>
      <p class="footer-note">仅允许白名单管理 IP 访问。会话 Cookie 使用 Secure、HttpOnly 和 SameSite=Strict。</p>
    </section>
  </main>

  <main v-else class="shell">
    <header class="topbar">
      <div>
        <h1>节点控制台</h1>
        <p class="muted">{{ panelAddress }}</p>
      </div>
      <button class="secondary" type="button" :disabled="busy" @click="logout">退出登录</button>
    </header>

    <div v-if="error" class="alert error">{{ error }}</div>
    <div v-if="notice" class="alert success">{{ notice }}</div>

    <section class="stats">
      <div class="stat">
        <div class="stat-label">sing-box</div>
        <div class="stat-value" :class="serviceClass">{{ state.status.sing_box }}</div>
      </div>
      <div class="stat">
        <div class="stat-label">系统负载</div>
        <div class="stat-value">{{ state.status.load_average }}</div>
      </div>
      <div class="stat">
        <div class="stat-label">内存使用</div>
        <div class="stat-value">{{ state.status.memory_used_mb }} MB</div>
      </div>
      <div class="stat">
        <div class="stat-label">内存总量</div>
        <div class="stat-value">{{ state.status.memory_total_mb }} MB</div>
      </div>
    </section>

    <form class="grid" @submit.prevent="save">
      <section class="card wide">
        <h2>节点地址</h2>
        <div class="field">
          <label for="public-host">Reality 使用的公网 IPv4、IPv6 或域名</label>
          <input id="public-host" v-model.trim="form.public_host" required />
        </div>
      </section>

      <section class="card">
        <div class="card-head">
          <div>
            <h2>VLESS Reality Vision</h2>
            <p class="muted">TCP 主力节点，不需要自己的节点证书。</p>
          </div>
          <label class="switch"><input v-model="form.reality.enabled" type="checkbox" />启用</label>
        </div>
        <div class="form-grid">
          <div class="field">
            <label for="reality-port">TCP 端口</label>
            <input id="reality-port" v-model.number="form.reality.port" type="number" min="1" max="65535" required />
          </div>
          <div class="field">
            <label for="reality-sni">Reality SNI / 握手目标</label>
            <input id="reality-sni" v-model.trim="form.reality.server_name" required />
          </div>
          <div class="field span">
            <label for="reality-uuid">UUID</label>
            <input id="reality-uuid" v-model.trim="form.reality.uuid" required />
          </div>
          <div class="field span">
            <label for="reality-private">Reality 私钥</label>
            <input id="reality-private" v-model.trim="form.reality.private_key" type="password" autocomplete="off" required />
          </div>
          <div class="field span">
            <label for="reality-public">Reality 公钥</label>
            <input id="reality-public" v-model.trim="form.reality.public_key" readonly required />
          </div>
          <div class="field">
            <label for="reality-short">Short ID</label>
            <input id="reality-short" v-model.trim="form.reality.short_id" required />
          </div>
        </div>
      </section>

      <section class="card">
        <div class="card-head">
          <div>
            <h2>Hysteria2</h2>
            <p class="muted">UDP 备选节点，带宽应根据实测逐步调整。</p>
          </div>
          <label class="switch"><input v-model="form.hysteria2.enabled" type="checkbox" />启用</label>
        </div>
        <div class="form-grid">
          <div class="field">
            <label for="hy2-port">UDP 端口</label>
            <input id="hy2-port" v-model.number="form.hysteria2.port" type="number" min="1" max="65535" required />
          </div>
          <div class="field">
            <label for="hy2-domain">证书域名</label>
            <input id="hy2-domain" v-model="form.hysteria2.domain" readonly />
          </div>
          <div class="field span">
            <label for="hy2-password">连接密码</label>
            <input id="hy2-password" v-model.trim="form.hysteria2.password" type="password" autocomplete="off" required />
          </div>
          <div class="field">
            <label for="hy2-up">服务器上传 Mbps</label>
            <input id="hy2-up" v-model.number="form.hysteria2.up_mbps" type="number" min="1" max="10000" required />
          </div>
          <div class="field">
            <label for="hy2-down">服务器下载 Mbps</label>
            <input id="hy2-down" v-model.number="form.hysteria2.down_mbps" type="number" min="1" max="10000" required />
          </div>
        </div>
      </section>

      <div class="card wide">
        <button class="primary" type="submit" :disabled="busy">{{ busy ? '校验并应用中…' : '校验并应用配置' }}</button>
        <p class="footer-note">应用前由 sing-box 校验候选配置；服务启动失败时恢复上一份可用配置。修改 Hysteria2 带宽不等于获得共享端口的保障速率。</p>
      </div>
    </form>

    <section v-if="Object.keys(state.links).length" class="card" style="margin-top: 18px">
      <h2>客户端链接与二维码</h2>
      <div class="links">
        <article v-for="(link, name) in state.links" :key="name" class="link-item">
          <div class="link-head">
            <h3>{{ name === 'reality' ? 'VLESS Reality Vision' : 'Hysteria2' }}</h3>
            <button class="secondary" type="button" @click="copyLink(name)">{{ copied === name ? '已复制' : '复制链接' }}</button>
          </div>
          <details>
            <summary class="muted">显示敏感链接与二维码</summary>
            <textarea class="secret-link" :value="link" readonly rows="4" @focus="$event.target.select()"></textarea>
            <img v-if="qr[name]" class="qr" :src="qr[name]" :alt="`${name} 客户端二维码`" />
          </details>
        </article>
      </div>
      <p class="footer-note">这些链接包含完整连接凭据，请勿发送到群聊、截图公开或提交到在线二维码服务。二维码在本地浏览器生成。</p>
    </section>
  </main>
</template>