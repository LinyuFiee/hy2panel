<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { Activity, LayoutDashboard, Leaf, LogOut, ServerCog, Settings2, ShieldCheck, Waypoints } from '@lucide/vue'
import DashboardView from './components/DashboardView.vue'
import NodesView from './components/NodesView.vue'
import SettingsView from './components/SettingsView.vue'
import HairlineFigure from './components/HairlineFigure.vue'

const view = ref('loading')
const activeTab = ref('dashboard')
const password = ref('')
const error = ref('')
const notice = ref('')
const loginBusy = ref(false)
const copied = ref('')
const state = ref(null)
const settingsRef = ref(null)
const networkSamples = ref([])
const busy = reactive({ nodes: false, racknerd: false, bbr: false, speedtest: false })
const form = reactive({
  public_host: '',
  reality: { enabled: true, port: 443, server_name: 'www.microsoft.com', uuid: '', private_key: '', public_key: '', short_id: '' },
  hysteria2: { enabled: false, port: 443, domain: '', password: '', up_mbps: 100, down_mbps: 100 }
})
const navItems = [
  { id: 'dashboard', label: '概览', icon: LayoutDashboard },
  { id: 'nodes', label: '节点管理', icon: ServerCog },
  { id: 'settings', label: '系统设置', icon: Settings2 }
]
const titles = {
  dashboard: ['节点概览', '实时掌握连接、流量与网络质量'],
  nodes: ['节点管理', '保留常用设置，把复杂参数收进高级选项'],
  settings: ['系统设置', 'RackNerd、BBR 与访问安全']
}
const currentTitle = computed(() => titles[activeTab.value])
const panelAddress = computed(() => state.value ? `https://${state.value.config.panel.domain}:8443` : '')
let pollTimer
let noticeTimer
let refreshing = false

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

function syncForms(config) {
  form.public_host = config.panel.public_host
  Object.assign(form.reality, config.reality)
  Object.assign(form.hysteria2, config.hysteria2)
}

function applyState(payload, sync = false) {
  state.value = payload
  if (sync) syncForms(payload.config)
  const status = payload.status
  networkSamples.value = [...networkSamples.value, { rx: status.rx_bytes_per_second, tx: status.tx_bytes_per_second }].slice(-30)
}

async function refresh(sync = false, showError = false) {
  if (refreshing) return
  refreshing = true
  try {
    applyState(await request('/api/state'), sync)
    view.value = 'app'
  } catch (failure) {
    if (showError && failure.status !== 401) error.value = failure.message
    if (view.value !== 'login' && !state.value) view.value = 'login'
  } finally {
    refreshing = false
  }
}

async function login() {
  loginBusy.value = true
  error.value = ''
  try {
    await request('/api/login', { method: 'POST', body: JSON.stringify({ password: password.value }) })
    password.value = ''
    activeTab.value = 'dashboard'
    await refresh(true, true)
  } catch (failure) {
    error.value = failure.message
  } finally {
    loginBusy.value = false
  }
}

async function logout() {
  try {
    await request('/api/logout', { method: 'POST', body: '{}' })
  } catch {
  } finally {
    state.value = null
    view.value = 'login'
  }
}

function switchTab(tab) {
  activeTab.value = tab
  error.value = ''
  notice.value = ''
}

function showNotice(message) {
  notice.value = message
  window.clearTimeout(noticeTimer)
  noticeTimer = window.setTimeout(() => { notice.value = '' }, 4000)
}

async function saveNodes() {
  busy.nodes = true
  error.value = ''
  try {
    applyState(await request('/api/config', {
      method: 'POST',
      body: JSON.stringify({ public_host: form.public_host, reality: form.reality, hysteria2: form.hysteria2 })
    }), true)
    showNotice('节点配置已校验并应用')
  } catch (failure) {
    error.value = failure.message
  } finally {
    busy.nodes = false
  }
}

async function saveRackNerd({ apiKey, apiHash }) {
  busy.racknerd = true
  error.value = ''
  try {
    applyState(await request('/api/settings/racknerd', {
      method: 'POST',
      body: JSON.stringify({ api_key: apiKey, api_hash: apiHash, clear: false })
    }))
    settingsRef.value?.clearCredentials()
    showNotice('RackNerd 配置已保存')
  } catch (failure) {
    error.value = failure.message
  } finally {
    busy.racknerd = false
  }
}

async function clearRackNerd() {
  busy.racknerd = true
  error.value = ''
  try {
    applyState(await request('/api/settings/racknerd', {
      method: 'POST', body: JSON.stringify({ api_key: '', api_hash: '', clear: true })
    }))
    settingsRef.value?.clearCredentials()
    showNotice('RackNerd 凭据已清除')
  } catch (failure) {
    error.value = failure.message
  } finally {
    busy.racknerd = false
  }
}

async function applyBBR() {
  busy.bbr = true
  error.value = ''
  try {
    applyState(await request('/api/bbr', { method: 'POST', body: '{}' }))
    showNotice('BBR + fq 已应用')
  } catch (failure) {
    error.value = failure.message
  } finally {
    busy.bbr = false
  }
}

async function startSpeedTest() {
  busy.speedtest = true
  error.value = ''
  try {
    state.value.speedtest = await request('/api/speedtest', { method: 'POST', body: '{}' })
    showNotice('测速已开始，通常需要半分钟左右')
  } catch (failure) {
    error.value = failure.message
  } finally {
    busy.speedtest = false
  }
}

async function copyLink(name) {
  const link = state.value?.links[name]
  if (!link) return
  await navigator.clipboard.writeText(link)
  copied.value = name
  window.setTimeout(() => { if (copied.value === name) copied.value = '' }, 1600)
}

onMounted(async () => {
  await refresh(true, true)
  pollTimer = window.setInterval(() => {
    if (view.value === 'app' && !document.hidden) refresh(false, false)
  }, 3000)
})

onBeforeUnmount(() => {
  window.clearInterval(pollTimer)
  window.clearTimeout(noticeTimer)
})
</script>

<template>
  <main v-if="view === 'loading'" class="loading-screen"><div class="logo-mark large"><Waypoints :size="28" /></div><span>正在整理节点数据…</span></main>

  <main v-else-if="view === 'login'" class="login-page">
    <section class="login-hero">
      <div class="login-brand"><div class="logo-mark"><Waypoints :size="23" /></div><span>hy2panel</span></div>
      <div class="login-hero-copy">
        <span class="eyebrow"><Leaf :size="14" /> A LIGHTER WAY TO CONNECT</span>
        <h1>小清新，<br />也能管好你的节点。</h1>
        <p>Reality、Hysteria2、流量与网络质量。<br />少一点复杂，多一点一目了然。</p>
      </div>
      <HairlineFigure />
      <small>移动指针，和这台小路由器打个招呼。</small>
    </section>
    <section class="login-form-wrap">
      <form class="login-form" @submit.prevent="login">
        <div class="login-lock"><ShieldCheck :size="24" /></div>
        <span class="section-kicker">WELCOME BACK</span>
        <h2>欢迎回来</h2>
        <p>输入管理员密码，继续前往节点控制台。</p>
        <div v-if="error" class="alert error-alert">{{ error }}</div>
        <label>管理员密码<input v-model="password" type="password" autocomplete="current-password" placeholder="输入密码" required autofocus /></label>
        <button class="primary-button full-button" type="submit" :disabled="loginBusy">{{ loginBusy ? '正在登录…' : '进入控制台' }}</button>
        <div class="login-security"><ShieldCheck :size="14" />HTTPS · IP 白名单 · 安全会话</div>
      </form>
    </section>
  </main>

  <div v-else class="app-shell">
    <aside class="sidebar">
      <div class="sidebar-brand"><div class="logo-mark"><Waypoints :size="22" /></div><div><strong>hy2panel</strong><span>轻盈节点管理</span></div></div>
      <nav class="sidebar-nav">
        <button v-for="item in navItems" :key="item.id" :class="{ active: activeTab === item.id }" @click="switchTab(item.id)">
          <component :is="item.icon" :size="19" /><span>{{ item.label }}</span>
        </button>
      </nav>
      <div class="sidebar-bottom">
        <div class="sidebar-status"><Activity :size="16" /><div><strong :class="state.status.sing_box === 'active' ? 'text-green' : 'text-red'">{{ state.status.sing_box === 'active' ? '服务运行正常' : '服务需要检查' }}</strong><span>{{ panelAddress }}</span></div></div>
        <button class="logout-button" @click="logout"><LogOut :size="17" />退出登录</button>
      </div>
    </aside>

    <main class="main-content">
      <header class="page-header">
        <div><h1>{{ currentTitle[0] }}</h1><p>{{ currentTitle[1] }}</p></div>
        <span class="status-pill" :class="state.status.sing_box === 'active' ? 'ok' : 'warn'"><i></i>{{ state.status.sing_box === 'active' ? '节点在线' : '节点异常' }}</span>
      </header>
      <div v-if="error" class="alert error-alert">{{ error }}</div>
      <div v-if="notice" class="alert success-alert">{{ notice }}</div>
      <DashboardView v-if="activeTab === 'dashboard'" :state="state" :samples="networkSamples" :starting="busy.speedtest" @speedtest="startSpeedTest" @settings="switchTab('settings')" />
      <NodesView v-else-if="activeTab === 'nodes'" :form="form" :links="state.links" :saving="busy.nodes" :copied="copied" @save="saveNodes" @copy="copyLink" />
      <SettingsView v-else ref="settingsRef" :state="state" :busy="busy" @save-racknerd="saveRackNerd" @clear-racknerd="clearRackNerd" @apply-bbr="applyBBR" />
    </main>
  </div>
</template>