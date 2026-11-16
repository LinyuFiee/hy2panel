<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { Activity, LayoutDashboard, Leaf, LogOut, ScrollText, ServerCog, Settings2, ShieldCheck } from '@lucide/vue'
import DashboardView from './components/DashboardView.vue'
import NodesView from './components/NodesView.vue'
import MaintenanceView from './components/MaintenanceView.vue'
import SettingsView from './components/SettingsView.vue'
import HairlineFigure from './components/HairlineFigure.vue'
import GithubIcon from './components/GithubIcon.vue'

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
const trafficLive = ref(null)
const trafficHistory = ref(null)
const trafficHistoryRange = ref('1h')
const trafficHistoryLoading = ref(false)
const singBoxVersions = ref(null)
const singBoxLoading = ref(false)
const diagnosticReport = ref(null)
const logEntries = ref([])
const logUnit = ref('sing-box')
const busy = reactive({ nodes: false, racknerd: false, bbr: false, speedtest: false, diagnostics: false, logs: false, recovery: false, updateCheck: false, updating: false, singBox: false })
const form = reactive({
  public_host: '',
  reality: { enabled: true, port: 443, server_name: 'www.microsoft.com', uuid: '', private_key: '', public_key: '', short_id: '' },
  hysteria2: { enabled: false, port: 443, domain: '', password: '', up_mbps: 100, down_mbps: 100 }
})
const navItems = [
  { id: 'dashboard', label: '概览', icon: LayoutDashboard },
  { id: 'nodes', label: '节点管理', icon: ServerCog },
  { id: 'maintenance', label: '日志与自检', icon: ScrollText },
  { id: 'settings', label: '系统设置', icon: Settings2 }
]
const titles = {
  dashboard: ['节点概览', '实时掌握连接、流量与网络质量'],
  nodes: ['节点管理', '保留常用设置，把复杂参数收进高级选项'],
  maintenance: ['日志与自检', '快速定位配置、监听、证书和服务异常'],
  settings: ['系统设置', '核心版本、网络优化与访问安全']
}
const maintenanceBusy = computed(() => busy.updating || busy.singBox || state.value?.update?.updating || singBoxVersions.value?.operation?.status === 'running')
const currentTitle = computed(() => titles[activeTab.value])
const panelAddress = computed(() => state.value?.config?.panel?.domain ? `https://${state.value.config.panel.domain}:8443` : '')
let statePollTimer
let livePollTimer
let historyPollTimer
let singBoxPollTimer
let noticeTimer
let stateRefreshing = false
let systemRefreshing = false
let trafficRefreshing = false
let historyRefreshing = false
let singBoxRefreshing = false
let authStopped = false

async function request(path, options = {}) {
  const headers = { 'X-Requested-With': 'fetch', ...(options.headers || {}) }
  if (options.body) headers['Content-Type'] = 'application/json'
  const response = await fetch(path, { credentials: 'same-origin', ...options, headers })
  const payload = await response.json().catch(() => ({}))
  if (response.status === 401 && path !== '/api/login') {
    stopPollingForAuth()
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
  if (!config) return
  form.public_host = config.panel?.public_host || ''
  Object.assign(form.reality, config.reality || {})
  Object.assign(form.hysteria2, config.hysteria2 || {})
}

function applyState(payload, sync = false) {
  const previousStatus = state.value?.status
  state.value = payload
  if (sync) syncForms(payload.config)
  if (!previousStatus && payload.status) appendNetworkSample(payload.status)
}

function appendNetworkSample(status) {
  if (!status) return
  networkSamples.value = [...networkSamples.value, {
    rx: Number(status.rx_bytes_per_second || 0),
    tx: Number(status.tx_bytes_per_second || 0)
  }].slice(-30)
}

async function refresh(sync = false, showError = false) {
  if (stateRefreshing || authStopped) return false
  stateRefreshing = true
  try {
    applyState(await request('/api/state'), sync)
    view.value = 'app'
    return true
  } catch (failure) {
    if (showError && failure.status !== 401) error.value = failure.message
    if (view.value !== 'login' && !state.value) view.value = 'login'
    return false
  } finally {
    stateRefreshing = false
  }
}

async function refreshSystemStatus(showError = false) {
  if (systemRefreshing || authStopped || view.value !== 'app') return
  systemRefreshing = true
  try {
    const payload = await request('/api/system/status')
    if (!state.value) return
    state.value = { ...state.value, status: payload.status, speedtest: payload.speedtest }
    appendNetworkSample(payload.status)
  } catch (failure) {
    if (showError && failure.status !== 401) error.value = failure.message
  } finally {
    systemRefreshing = false
  }
}

async function refreshTrafficLive(showError = false) {
  if (trafficRefreshing || authStopped || view.value !== 'app') return
  trafficRefreshing = true
  try {
    trafficLive.value = await request('/api/traffic/live')
  } catch (failure) {
    if (showError && failure.status !== 401) error.value = failure.message
  } finally {
    trafficRefreshing = false
  }
}

async function loadTrafficHistory(range = trafficHistoryRange.value, showError = false) {
  if (!['1h', '6h', '24h', '7d'].includes(range) || historyRefreshing || authStopped || view.value !== 'app') return
  historyRefreshing = true
  trafficHistoryLoading.value = true
  try {
    const payload = await request(`/api/traffic/history?range=${encodeURIComponent(range)}`)
    trafficHistoryRange.value = range
    trafficHistory.value = payload
  } catch (failure) {
    if (showError && failure.status !== 401) error.value = failure.message
  } finally {
    historyRefreshing = false
    trafficHistoryLoading.value = false
  }
}

async function loadSingBoxVersions(showError = false) {
  if (singBoxRefreshing || authStopped || view.value !== 'app') return
  singBoxRefreshing = true
  singBoxLoading.value = true
  try {
    singBoxVersions.value = await request('/api/sing-box/versions')
  } catch (failure) {
    if (showError && failure.status !== 401) error.value = failure.message
  } finally {
    singBoxRefreshing = false
    singBoxLoading.value = false
  }
}

async function login() {
  loginBusy.value = true
  error.value = ''
  try {
    await request('/api/login', { method: 'POST', body: JSON.stringify({ password: password.value }) })
    authStopped = false
    password.value = ''
    activeTab.value = 'dashboard'
    const authenticated = await refresh(true, true)
    if (authenticated && view.value === 'app') {
      await Promise.all([refreshSystemStatus(), refreshTrafficLive(), loadTrafficHistory(trafficHistoryRange.value)])
      startPolling()
    }
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
    stopPollingForAuth()
    state.value = null
    trafficLive.value = null
    trafficHistory.value = null
    singBoxVersions.value = null
    view.value = 'login'
  }
}

function switchTab(tab) {
  activeTab.value = tab
  error.value = ''
  notice.value = ''
  if (tab === 'dashboard') {
    loadTrafficHistory(trafficHistoryRange.value)
  }
  if (tab === 'maintenance' && !logEntries.value.length) loadLogs()
  if (tab === 'settings') loadSingBoxVersions()
}

function showNotice(message) {
  notice.value = message
  window.clearTimeout(noticeTimer)
  noticeTimer = window.setTimeout(() => { notice.value = '' }, 4000)
}

async function saveNodes() {
  if (maintenanceBusy.value) {
    error.value = '核心或面板正在维护，请等待完成后再保存节点配置'
    return
  }
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
  if (maintenanceBusy.value) {
    error.value = '维护任务正在运行，请稍后再应用 BBR'
    return
  }
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

async function runDiagnostics() {
  busy.diagnostics = true
  error.value = ''
  try {
    diagnosticReport.value = await request('/api/diagnostics', { method: 'POST', body: '{}' })
    showNotice('服务端自检已完成')
  } catch (failure) {
    error.value = failure.message
  } finally {
    busy.diagnostics = false
  }
}

async function loadLogs(unit = logUnit.value) {
  busy.logs = true
  error.value = ''
  try {
    const payload = await request(`/api/logs?unit=${encodeURIComponent(unit)}&lines=150`)
    logUnit.value = unit
    logEntries.value = payload.entries
  } catch (failure) {
    error.value = failure.message
  } finally {
    busy.logs = false
  }
}

async function updateRecovery(autoPort) {
  busy.recovery = true
  error.value = ''
  try {
    applyState(await request('/api/settings/recovery', {
      method: 'POST', body: JSON.stringify({ auto_port: autoPort })
    }))
    showNotice(autoPort ? '已启用保守自动换端口' : '已关闭自动换端口，自动重启仍然有效')
  } catch (failure) {
    error.value = failure.message
  } finally {
    busy.recovery = false
  }
}

async function switchSingBox(version) {
  if (!version || maintenanceBusy.value) return
  if (!window.confirm(`切换 sing-box 到 v${version} 会短暂中断代理连接。确定继续吗？`)) return
  busy.singBox = true
  error.value = ''
  try {
    const operation = await request('/api/sing-box/switch', {
      method: 'POST', body: JSON.stringify({ version })
    })
    if (singBoxVersions.value) singBoxVersions.value = { ...singBoxVersions.value, operation }
    showNotice(`sing-box v${version} 切换任务已提交`)
  } catch (failure) {
    error.value = failure.message
    if (failure.status === 409) await loadSingBoxVersions()
  } finally {
    busy.singBox = false
  }
}

async function rollbackSingBox() {
  if (maintenanceBusy.value || !singBoxVersions.value?.can_rollback) return
  const previous = singBoxVersions.value.previous_version
  if (!window.confirm(`回滚到上一版本${previous ? ` v${previous}` : ''} 会短暂中断代理连接。确定继续吗？`)) return
  busy.singBox = true
  error.value = ''
  try {
    const operation = await request('/api/sing-box/rollback', { method: 'POST', body: '{}' })
    singBoxVersions.value = { ...singBoxVersions.value, operation }
    showNotice('sing-box 回滚任务已提交')
  } catch (failure) {
    error.value = failure.message
    if (failure.status === 409) await loadSingBoxVersions()
  } finally {
    busy.singBox = false
  }
}

function setTrafficHistoryRange(range) {
  if (range === trafficHistoryRange.value && trafficHistory.value) return
  loadTrafficHistory(range, true)
}

async function checkUpdates() {
  busy.updateCheck = true
  error.value = ''
  try {
    state.value.update = await request('/api/update/check', { method: 'POST', body: '{}' })
    showNotice(state.value.update.available ? `发现新版本 v${state.value.update.latest_version}` : '当前已经是最新版本')
  } catch (failure) {
    error.value = failure.message
  } finally {
    busy.updateCheck = false
  }
}

async function applyUpdate() {
  if (maintenanceBusy.value) {
    error.value = 'sing-box 或面板维护任务正在运行'
    return
  }
  if (!window.confirm('更新会短暂重启面板，但会保留节点配置。确定继续吗？')) return
  busy.updating = true
  error.value = ''
  try {
    state.value.update = await request('/api/update/apply', { method: 'POST', body: '{}' })
    showNotice('更新已开始，完成后请刷新页面')
  } catch (failure) {
    error.value = failure.message
  } finally {
    busy.updating = false
  }
}

async function copyLink(name) {
  const link = state.value?.links[name]
  if (!link) return
  try {
    await navigator.clipboard.writeText(link)
  } catch {
    const textarea = document.createElement('textarea')
    textarea.value = link
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
  copied.value = name
  window.setTimeout(() => { if (copied.value === name) copied.value = '' }, 1600)
}

function stopPollingForAuth() {
  authStopped = true
  stateRefreshing = false
  systemRefreshing = false
  trafficRefreshing = false
  historyRefreshing = false
  singBoxRefreshing = false
  trafficHistoryLoading.value = false
  singBoxLoading.value = false
  window.clearInterval(statePollTimer)
  window.clearInterval(livePollTimer)
  window.clearInterval(historyPollTimer)
  window.clearInterval(singBoxPollTimer)
  statePollTimer = undefined
  livePollTimer = undefined
  historyPollTimer = undefined
  singBoxPollTimer = undefined
}

function startPolling() {
  authStopped = false
  window.clearInterval(statePollTimer)
  window.clearInterval(livePollTimer)
  window.clearInterval(historyPollTimer)
  window.clearInterval(singBoxPollTimer)
  statePollTimer = window.setInterval(() => {
    if (view.value === 'app' && !document.hidden) refresh(false, false)
  }, 3000)
  livePollTimer = window.setInterval(() => {
    if (view.value === 'app' && !document.hidden) {
      refreshSystemStatus()
      refreshTrafficLive()
    }
  }, 2500)
  historyPollTimer = window.setInterval(() => {
    if (view.value === 'app' && activeTab.value === 'dashboard' && !document.hidden) loadTrafficHistory(trafficHistoryRange.value)
  }, 60000)
  singBoxPollTimer = window.setInterval(() => {
    if (view.value === 'app' && activeTab.value === 'settings' && !document.hidden) loadSingBoxVersions()
  }, 3000)
}

function handleVisibilityChange() {
  if (document.hidden || authStopped || view.value !== 'app') return
  refresh(false, false)
  refreshSystemStatus()
  refreshTrafficLive()
  if (activeTab.value === 'dashboard') loadTrafficHistory(trafficHistoryRange.value)
  if (activeTab.value === 'settings') loadSingBoxVersions()
}

onMounted(async () => {
  authStopped = false
  const authenticated = await refresh(true, true)
  if (authenticated && view.value === 'app') {
    await Promise.all([refreshSystemStatus(), refreshTrafficLive(), loadTrafficHistory(trafficHistoryRange.value)])
    startPolling()
  }
  document.addEventListener('visibilitychange', handleVisibilityChange)
})

onBeforeUnmount(() => {
  stopPollingForAuth()
  window.clearTimeout(noticeTimer)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
})
</script>

<template>
  <main v-if="view === 'loading'" class="loading-screen"><img class="logo-mark logo-image large" src="/logo.png" alt="hy2panel 白发男孩与白猫 Logo" /><span>正在整理节点数据…</span></main>

  <main v-else-if="view === 'login'" class="login-page">
    <section class="login-hero">
      <div class="login-brand"><img class="logo-mark logo-image" src="/logo.png" alt="hy2panel 白发男孩与白猫 Logo" /><span>hy2panel</span></div>
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
        <div class="login-security"><ShieldCheck :size="14" />公网 HTTPS · 登录限速 · 安全会话</div>
      </form>
    </section>
  </main>

  <div v-else class="app-shell">
    <aside class="sidebar">
      <div class="sidebar-brand"><img class="logo-mark logo-image" src="/logo.png" alt="hy2panel 白发男孩与白猫 Logo" /><div><strong>hy2panel</strong><span>轻盈节点管理</span></div></div>
      <nav class="sidebar-nav">
        <button v-for="item in navItems" :key="item.id" :class="{ active: activeTab === item.id }" @click="switchTab(item.id)">
          <component :is="item.icon" :size="19" /><span>{{ item.label }}</span>
        </button>
      </nav>
      <div class="sidebar-bottom">
        <div class="sidebar-status"><Activity :size="16" /><div><strong :class="state.status.sing_box === 'active' ? 'text-green' : 'text-red'">{{ state.status.sing_box === 'active' ? '服务运行正常' : '服务需要检查' }}</strong><span>{{ panelAddress }}</span></div></div>
        <a class="github-link" href="https://github.com/LinyuFiee/hy2panel" target="_blank" rel="noopener noreferrer"><GithubIcon :size="16" />GitHub</a>
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
      <DashboardView
        v-if="activeTab === 'dashboard'"
        :state="state"
        :samples="networkSamples"
        :starting="busy.speedtest"
        :traffic="trafficLive"
        :history="trafficHistory"
        :history-range="trafficHistoryRange"
        :history-loading="trafficHistoryLoading"
        @speedtest="startSpeedTest"
        @settings="switchTab('settings')"
        @history-range="setTrafficHistoryRange"
      />
      <NodesView v-else-if="activeTab === 'nodes'" :form="form" :links="state.links" :meta="state.node_meta" :saving="busy.nodes" :copied="copied" @save="saveNodes" @copy="copyLink" />
      <MaintenanceView v-else-if="activeTab === 'maintenance'" :state="state" :report="diagnosticReport" :entries="logEntries" :log-unit="logUnit" :busy="busy" @diagnostics="runDiagnostics" @logs="loadLogs" @recovery="updateRecovery" />
      <SettingsView
        v-else
        ref="settingsRef"
        :state="state"
        :busy="busy"
        :sing-box="singBoxVersions"
        :sing-box-loading="singBoxLoading"
        @save-racknerd="saveRackNerd"
        @clear-racknerd="clearRackNerd"
        @apply-bbr="applyBBR"
        @check-update="checkUpdates"
        @apply-update="applyUpdate"
        @switch-sing-box="switchSingBox"
        @rollback-sing-box="rollbackSingBox"
      />
    </main>
  </div>
</template>