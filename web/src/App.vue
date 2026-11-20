<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import {
  AlertTriangle,
  ExternalLink,
  LogOut,
  ShieldCheck,
  Wrench
} from '@lucide/vue'
import { toast } from 'vue-sonner'
import DashboardView from './components/DashboardView.vue'
import NodesView from './components/NodesView.vue'
import MaintenanceView from './components/MaintenanceView.vue'
import SettingsView from './components/SettingsView.vue'
import OverviewView from './components/OverviewView.vue'
import ThemeToggle from './components/ThemeToggle.vue'
import GithubIcon from './components/GithubIcon.vue'
import LoginPage from './pages/LoginPage.vue'
import { Alert, AlertDescription, AlertTitle } from './components/ui/alert'
import { Spinner } from './components/ui/spinner'
import { NAV_ITEMS, DEFAULT_SECTION, findNavItem, hashForSection, sectionFromHash } from './app/navigation'
import { createApiClient, isCanceledRequest } from './api/client'
import { createPanelApi } from './api/panel'
import { useClipboardFeedback } from './composables/useClipboard'
import { useMaintenanceLock } from './composables/useMaintenanceLock'
import { useNodeDraft } from './composables/useNodeDraft'
import { usePolling } from './composables/usePolling'
import { useSessionReset } from './composables/useSessionReset'
import { useTrafficHistory } from './composables/useTrafficHistory'

const view = ref('loading')
const activeTab = ref(DEFAULT_SECTION)
const password = ref('')
const error = ref('')
const notice = ref('')
const loginBusy = ref(false)
const state = ref(null)
const settingsRef = ref(null)
const singBoxVersions = ref(null)
const singBoxLoading = ref(false)
const diagnosticReport = ref(null)
const logEntries = ref([])
const logUnit = ref('sing-box')
const busy = reactive({ nodes: false, racknerd: false, bbr: false, speedtest: false, diagnostics: false, logs: false, recovery: false, updateCheck: false, updating: false, singBox: false })

const currentNavItem = computed(() => findNavItem(activeTab.value))
const panelAddress = computed(() => {
  const domain = state.value?.config?.panel?.domain
  if (domain) return `https://${domain}:8443`
  if (typeof window !== 'undefined') return window.location.origin
  return ''
})
const serviceHealthy = computed(() => state.value?.status?.sing_box === 'active')
const overviewStatus = computed(() => state.value?.status || {})
const overviewRackNerd = computed(() => state.value?.racknerd || { configured: false })
const maintenanceLabel = computed(() => maintenanceBusy.value ? '维护任务运行中' : '无维护任务')
const routeAnnouncement = computed(() => `当前页面：${currentNavItem.value.title}`)

const nodeDraft = useNodeDraft()
const { form, conflict: nodeDraftConflict } = nodeDraft
const clipboard = useClipboardFeedback()
const { copied } = clipboard
const polling = usePolling()
const session = useSessionReset()
let noticeTimer
let stateRefreshing = false
let systemRefreshing = false
let singBoxRefreshing = false
let authStopped = false
let resettingSession = false

const apiClient = createApiClient({ onUnauthorized: handleUnauthorized })
const api = createPanelApi(apiClient)
const traffic = useTrafficHistory({
  api,
  canRequest: () => !authStopped && view.value === 'app',
  reportError: message => { error.value = message }
})
const {
  networkSamples,
  trafficLive,
  trafficHistory,
  trafficHistoryRange,
  trafficHistoryLoading,
  refreshTrafficLive,
  loadTrafficHistory,
  setTrafficHistoryRange
} = traffic
const maintenance = useMaintenanceLock({ busy, state, singBoxVersions })
const maintenanceBusy = maintenance.locked

function clearBusy() {
  for (const key of Object.keys(busy)) busy[key] = false
}

function resetSensitiveState() {
  state.value = null
  singBoxVersions.value = null
  diagnosticReport.value = null
  logEntries.value = []
  logUnit.value = 'sing-box'
  password.value = ''
  settingsRef.value?.clearCredentials()
  nodeDraft.reset()
  traffic.reset()
  clipboard.reset()
  singBoxLoading.value = false
  clearBusy()
}

session.register(() => {
  polling.stop()
  apiClient.resetSession()
  stateRefreshing = false
  systemRefreshing = false
  singBoxRefreshing = false
  traffic.stop()
})
session.register(resetSensitiveState)

function resetSession(reason) {
  if (resettingSession) return
  resettingSession = true
  authStopped = true
  session.reset(reason)
  resettingSession = false
}

function ensureSectionHash(section = activeTab.value, { replace = false } = {}) {
  const target = hashForSection(section)
  if (window.location.hash === target) return
  if (replace) window.history.replaceState(null, '', target)
  else window.location.hash = target
}

function syncSectionFromHash({ replaceInvalid = false } = {}) {
  const requested = sectionFromHash(window.location.hash)
  const normalizedHash = hashForSection(requested)
  activeTab.value = requested
  if (replaceInvalid && window.location.hash !== normalizedHash) {
    window.history.replaceState(null, '', normalizedHash)
  }
}

function handleHashChange() {
  if (view.value !== 'app') return
  const section = sectionFromHash(window.location.hash)
  switchTab(section, { updateHash: false })
  if (window.location.hash !== hashForSection(section)) {
    window.history.replaceState(null, '', hashForSection(section))
  }
}

function handleUnauthorized() {
  resetSession('unauthorized')
  error.value = ''
  notice.value = ''
  activeTab.value = DEFAULT_SECTION
  view.value = 'login'
}

function applyState(payload, { syncDraft = false } = {}) {
  const previousStatus = state.value?.status
  state.value = payload
  if (syncDraft) nodeDraft.accept(payload.config)
  else nodeDraft.reconcile(payload.config)
  if (!previousStatus && payload.status) traffic.sample(payload.status)
}

async function refresh(sync = false, showError = false) {
  if (stateRefreshing || authStopped) return false
  stateRefreshing = true
  try {
    applyState(await api.state(), { syncDraft: sync })
    view.value = 'app'
    return true
  } catch (failure) {
    if (showError && failure.status !== 401 && !isCanceledRequest(failure)) error.value = failure.message
    if (!isCanceledRequest(failure) && view.value !== 'login' && !state.value) view.value = 'login'
    return false
  } finally {
    stateRefreshing = false
  }
}

async function refreshSystemStatus(showError = false) {
  if (systemRefreshing || authStopped || view.value !== 'app') return false
  systemRefreshing = true
  try {
    const payload = await api.systemStatus()
    if (!state.value) return false
    state.value = { ...state.value, status: payload.status, speedtest: payload.speedtest }
    traffic.sample(payload.status)
    return true
  } catch (failure) {
    if (showError && failure.status !== 401 && !isCanceledRequest(failure)) error.value = failure.message
    return false
  } finally {
    systemRefreshing = false
  }
}

async function loadSingBoxVersions(showError = false) {
  if (singBoxRefreshing || authStopped || view.value !== 'app') return false
  singBoxRefreshing = true
  singBoxLoading.value = true
  try {
    singBoxVersions.value = await api.singBoxVersions()
    return true
  } catch (failure) {
    if (showError && failure.status !== 401 && !isCanceledRequest(failure)) error.value = failure.message
    return false
  } finally {
    singBoxRefreshing = false
    singBoxLoading.value = false
  }
}

async function initializeAuthenticatedSession() {
  const authenticated = await refresh(true, true)
  if (!authenticated || view.value !== 'app') return false
  syncSectionFromHash({ replaceInvalid: true })
  const initialRequests = [refreshSystemStatus()]
  if (activeTab.value === 'traffic') {
    initialRequests.push(refreshTrafficLive(), loadTrafficHistory(trafficHistoryRange.value))
  }
  await Promise.all(initialRequests)
  if (activeTab.value === 'maintenance' && !logEntries.value.length) loadLogs()
  if (activeTab.value === 'settings') loadSingBoxVersions()
  if (!authStopped && view.value === 'app') startPolling()
  return true
}

async function login() {
  if (loginBusy.value) return
  loginBusy.value = true
  error.value = ''
  try {
    await api.login(password.value)
    apiClient.resetSession()
    authStopped = false
    password.value = ''
    activeTab.value = sectionFromHash(window.location.hash)
    const authenticated = await initializeAuthenticatedSession()
    if (authenticated) toast.success('已进入控制平面')
  } catch (failure) {
    if (!isCanceledRequest(failure)) error.value = failure.message
  } finally {
    loginBusy.value = false
  }
}

async function logout() {
  polling.stop()
  apiClient.abortInFlight('logout')
  try {
    await api.logout()
  } catch {
  } finally {
    resetSession('logout')
    error.value = ''
    notice.value = ''
    activeTab.value = DEFAULT_SECTION
    view.value = 'login'
  }
}

function switchTab(tab, { updateHash = true } = {}) {
  const section = findNavItem(tab).id
  activeTab.value = section
  window.scrollTo({ top: 0, left: 0, behavior: 'auto' })
  error.value = ''
  notice.value = ''
  if (updateHash) ensureSectionHash(section)
  if (section === 'traffic') {
    refreshSystemStatus()
    refreshTrafficLive()
    loadTrafficHistory(trafficHistoryRange.value)
  }
  if (section === 'maintenance' && !logEntries.value.length) loadLogs()
  if (section === 'settings') loadSingBoxVersions()
}

function showNotice(message) {
  notice.value = message
  toast.success(message)
  window.clearTimeout(noticeTimer)
  noticeTimer = window.setTimeout(() => { notice.value = '' }, 4000)
}

async function saveNodes() {
  const lockMessage = maintenance.guard('核心或面板正在维护，请等待完成后再保存节点配置')
  if (lockMessage) {
    error.value = lockMessage
    return
  }
  if (nodeDraftConflict.value) {
    error.value = '节点配置已在其他会话中更新，请重新登录或刷新页面后再修改'
    return
  }
  busy.nodes = true
  error.value = ''
  try {
    applyState(await api.saveNodes(nodeDraft.payload()), { syncDraft: true })
    showNotice('节点配置已校验并应用')
  } catch (failure) {
    if (!isCanceledRequest(failure)) error.value = failure.message
  } finally {
    busy.nodes = false
  }
}

async function saveRackNerd({ apiKey, apiHash }) {
  busy.racknerd = true
  error.value = ''
  try {
    applyState(await api.saveRackNerd({ apiKey, apiHash }))
    settingsRef.value?.clearCredentials()
    showNotice('RackNerd 配置已保存')
  } catch (failure) {
    if (!isCanceledRequest(failure)) error.value = failure.message
  } finally {
    busy.racknerd = false
  }
}

async function clearRackNerd() {
  busy.racknerd = true
  error.value = ''
  try {
    applyState(await api.clearRackNerd())
    settingsRef.value?.clearCredentials()
    showNotice('RackNerd 凭据已清除')
  } catch (failure) {
    if (!isCanceledRequest(failure)) error.value = failure.message
  } finally {
    busy.racknerd = false
  }
}

async function applyBBR() {
  const lockMessage = maintenance.guard('维护任务正在运行，请稍后再应用 BBR')
  if (lockMessage) {
    error.value = lockMessage
    return
  }
  busy.bbr = true
  error.value = ''
  try {
    applyState(await api.applyBBR())
    showNotice('BBR + fq 已应用')
  } catch (failure) {
    if (!isCanceledRequest(failure)) error.value = failure.message
  } finally {
    busy.bbr = false
  }
}

async function startSpeedTest() {
  busy.speedtest = true
  error.value = ''
  try {
    const speedtest = await api.startSpeedTest()
    if (state.value) state.value = { ...state.value, speedtest }
    showNotice('测速已开始，通常需要半分钟左右')
  } catch (failure) {
    if (!isCanceledRequest(failure)) error.value = failure.message
  } finally {
    busy.speedtest = false
  }
}

async function runDiagnostics() {
  busy.diagnostics = true
  error.value = ''
  try {
    diagnosticReport.value = await api.diagnostics()
    showNotice('服务端自检已完成')
  } catch (failure) {
    if (!isCanceledRequest(failure)) error.value = failure.message
  } finally {
    busy.diagnostics = false
  }
}

async function loadLogs(unit = logUnit.value) {
  busy.logs = true
  error.value = ''
  try {
    const payload = await api.logs(unit)
    logUnit.value = unit
    logEntries.value = payload.entries
  } catch (failure) {
    if (!isCanceledRequest(failure)) error.value = failure.message
  } finally {
    busy.logs = false
  }
}

async function updateRecovery(autoPort) {
  busy.recovery = true
  error.value = ''
  try {
    applyState(await api.updateRecovery(autoPort))
    showNotice(autoPort ? '已启用保守自动换端口' : '已关闭自动换端口，自动重启仍然有效')
  } catch (failure) {
    if (!isCanceledRequest(failure)) error.value = failure.message
  } finally {
    busy.recovery = false
  }
}

async function switchSingBox(version) {
  if (!version || maintenanceBusy.value) return
  busy.singBox = true
  error.value = ''
  try {
    const operation = await api.switchSingBox(version)
    if (singBoxVersions.value) singBoxVersions.value = { ...singBoxVersions.value, operation }
    showNotice(`sing-box v${version} 切换任务已提交`)
  } catch (failure) {
    if (!isCanceledRequest(failure)) error.value = failure.message
    if (failure.status === 409 && !authStopped) await loadSingBoxVersions()
  } finally {
    busy.singBox = false
  }
}

async function rollbackSingBox() {
  if (maintenanceBusy.value || !singBoxVersions.value?.can_rollback) return
  busy.singBox = true
  error.value = ''
  try {
    const operation = await api.rollbackSingBox()
    singBoxVersions.value = { ...singBoxVersions.value, operation }
    showNotice('sing-box 回滚任务已提交')
  } catch (failure) {
    if (!isCanceledRequest(failure)) error.value = failure.message
    if (failure.status === 409 && !authStopped) await loadSingBoxVersions()
  } finally {
    busy.singBox = false
  }
}

async function checkUpdates() {
  busy.updateCheck = true
  error.value = ''
  try {
    const update = await api.checkUpdates()
    if (state.value) state.value = { ...state.value, update }
    showNotice(update.available ? `发现新版本 v${update.latest_version}` : '当前已经是最新版本')
  } catch (failure) {
    if (!isCanceledRequest(failure)) error.value = failure.message
  } finally {
    busy.updateCheck = false
  }
}

async function applyUpdate() {
  const lockMessage = maintenance.guard('sing-box 或面板维护任务正在运行')
  if (lockMessage) {
    error.value = lockMessage
    return
  }
  busy.updating = true
  error.value = ''
  try {
    const update = await api.applyUpdate()
    if (state.value) state.value = { ...state.value, update }
    showNotice('更新已开始，完成后请刷新页面')
  } catch (failure) {
    if (!isCanceledRequest(failure)) error.value = failure.message
  } finally {
    busy.updating = false
  }
}

async function copyLink(name) {
  await clipboard.copy(name, state.value?.links?.[name])
}

function startPolling() {
  authStopped = false
  polling.start([
    { id: 'state', interval: 3000, when: () => view.value === 'app' && !document.hidden, run: () => refresh(false, false) },
    {
      id: 'live',
      interval: 2500,
      when: () => view.value === 'app' && activeTab.value === 'traffic' && !document.hidden,
      run: () => {
        refreshSystemStatus()
        refreshTrafficLive()
      }
    },
    { id: 'history', interval: 60000, when: () => view.value === 'app' && activeTab.value === 'traffic' && !document.hidden, run: () => loadTrafficHistory(trafficHistoryRange.value) },
    { id: 'sing-box', interval: 3000, when: () => view.value === 'app' && activeTab.value === 'settings' && !document.hidden, run: () => loadSingBoxVersions() }
  ])
}

function handleVisibilityChange() {
  if (document.hidden || authStopped || view.value !== 'app') return
  refresh(false, false)
  refreshSystemStatus()
  if (activeTab.value === 'traffic') {
    refreshTrafficLive()
    loadTrafficHistory(trafficHistoryRange.value)
  }
  if (activeTab.value === 'settings') loadSingBoxVersions()
}

watch(error, message => {
  if (message && view.value === 'app') toast.error(message)
})

onMounted(async () => {
  authStopped = false
  window.addEventListener('hashchange', handleHashChange)
  await initializeAuthenticatedSession()
  document.addEventListener('visibilitychange', handleVisibilityChange)
})

onBeforeUnmount(() => {
  resetSession('unmount')
  window.clearTimeout(noticeTimer)
  window.removeEventListener('hashchange', handleHashChange)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
})
</script>

<template>
  <a class="skip-link" href="#main-content">跳到主要内容</a>

  <main v-if="view === 'loading'" id="main-content" class="boot" tabindex="-1" aria-live="polite">
    <img class="boot-logo" src="/logo.png" alt="" />
    <Spinner label="正在连接控制平面" class="boot-spinner" />
    <strong>正在连接控制平面</strong>
  </main>

  <LoginPage
    v-else-if="view === 'login'"
    v-model:password="password"
    :error="error"
    :busy="loginBusy"
    @submit="login"
  />

  <div v-else class="shell">
    <aside class="sidebar">
      <a class="brand" :href="hashForSection('overview')" @click.prevent="switchTab('overview')">
        <img src="/logo.png" alt="" /><span>hy2panel</span>
      </a>

      <nav class="side-nav" aria-label="控制台分区">
        <a
          v-for="item in NAV_ITEMS"
          :key="item.id"
          :href="hashForSection(item.id)"
          :class="{ active: activeTab === item.id }"
          :aria-current="activeTab === item.id ? 'page' : undefined"
          @click.prevent="switchTab(item.id)"
        >
          <component :is="item.icon" :size="18" />
          <span>{{ item.label }}</span>
        </a>
      </nav>

      <div class="side-foot">
        <a class="side-link" href="https://github.com/LinyuFiee/hy2panel" target="_blank" rel="noopener noreferrer">
          <GithubIcon /><span>GitHub</span><ExternalLink :size="12" />
        </a>
      </div>
    </aside>

    <div class="workspace">
      <header class="topbar">
        <div class="topbar-context">
          <img src="/logo.png" alt="" />
          <strong class="topbar-title">{{ currentNavItem.label }}</strong>
        </div>
        <div class="topbar-actions">
          <span class="chip" :class="serviceHealthy ? 'chip-ok' : 'chip-bad'" :title="serviceHealthy ? 'sing-box 服务运行正常' : 'sing-box 服务需要检查'">
            <i></i><span>{{ serviceHealthy ? '在线' : '异常' }}</span>
          </span>
          <span class="chip" :class="{ 'chip-warn': maintenanceBusy }" :title="maintenanceLabel">
            <Wrench :size="12" /><span>{{ maintenanceBusy ? '维护中' : '空闲' }}</span>
          </span>
          <ThemeToggle compact />
          <button class="icon-btn" type="button" aria-label="退出登录" title="退出登录" @click="logout">
            <LogOut :size="16" />
          </button>
        </div>
      </header>

      <main id="main-content" class="content" tabindex="-1">
        <div class="route-announcement sr-only" aria-live="polite">{{ routeAnnouncement }}</div>

        <header class="page-head">
          <div class="page-heading">
            <p class="page-eyebrow">{{ currentNavItem.eyebrow }}</p>
            <h1>{{ currentNavItem.title }}</h1>
            <p class="page-description">{{ currentNavItem.description }}</p>
          </div>
          <div class="page-meta">
            <span>控制平面</span>
            <small class="tnum">{{ panelAddress }}</small>
          </div>
        </header>

        <Alert v-if="error" variant="destructive" class="page-alert" aria-live="assertive">
          <AlertTriangle />
          <AlertTitle>操作未完成</AlertTitle>
          <AlertDescription>{{ error }}</AlertDescription>
        </Alert>
        <Alert v-if="notice" variant="success" class="page-alert" aria-live="polite">
          <ShieldCheck />
          <AlertTitle>操作已完成</AlertTitle>
          <AlertDescription>{{ notice }}</AlertDescription>
        </Alert>

        <div class="view" :data-section="activeTab">
          <OverviewView
            v-if="activeTab === 'overview'"
            :config="state.config"
            :status="overviewStatus"
            :racknerd="overviewRackNerd"
            :healthy="serviceHealthy"
            :maintenance-busy="maintenanceBusy"
            @goto="switchTab"
          />
          <DashboardView
            v-else-if="activeTab === 'traffic'"
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
          <NodesView
            v-else-if="activeTab === 'nodes'"
            :form="form"
            :links="state.links"
            :meta="state.node_meta"
            :saving="busy.nodes"
            :dirty="nodeDraft.dirty.value"
            :conflict="nodeDraftConflict.value"
            :copied="copied"
            @save="saveNodes"
            @copy="copyLink"
          />
          <MaintenanceView
            v-else-if="activeTab === 'maintenance'"
            :state="state"
            :report="diagnosticReport"
            :entries="logEntries"
            :log-unit="logUnit"
            :busy="busy"
            @diagnostics="runDiagnostics"
            @logs="loadLogs"
            @recovery="updateRecovery"
          />
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
        </div>
      </main>

      <nav class="tabbar" aria-label="底部导航">
        <a
          v-for="item in NAV_ITEMS"
          :key="item.id"
          :href="hashForSection(item.id)"
          :class="{ active: activeTab === item.id }"
          :aria-current="activeTab === item.id ? 'page' : undefined"
          @click.prevent="switchTab(item.id)"
        >
          <component :is="item.icon" :size="20" />
          <span>{{ item.label }}</span>
        </a>
      </nav>
    </div>
  </div>
</template>

<style scoped>
.skip-link {
  position: fixed;
  z-index: 1000;
  top: 0.75rem;
  left: 0.75rem;
  transform: translateY(-180%);
  padding: 0.6rem 0.9rem;
  border-radius: 0.6rem;
  background: var(--primary);
  color: var(--primary-foreground);
  font-size: 0.8rem;
  font-weight: 600;
  text-decoration: none;
  transition: transform 0.15s ease;
}

.skip-link:focus {
  transform: translateY(0);
}

.boot {
  min-height: 100svh;
  display: grid;
  place-items: center;
  align-content: center;
  gap: 0.8rem;
  background: var(--background);
  color: var(--foreground);
}

.boot:focus {
  outline: none;
}

.boot-logo {
  width: 3rem;
  height: 3rem;
  border-radius: 0.85rem;
}

.boot-spinner {
  color: var(--primary);
}

.boot strong {
  color: var(--muted-foreground);
  font-size: 0.85rem;
  font-weight: 500;
}

.shell {
  min-height: 100svh;
  display: grid;
  grid-template-columns: 14rem minmax(0, 1fr);
  background: var(--background);
  color: var(--foreground);
}

/* ---------- sidebar ---------- */

.sidebar {
  position: sticky;
  z-index: 30;
  top: 0;
  height: 100svh;
  display: flex;
  flex-direction: column;
  padding: 0.8rem;
  border-right: 1px solid var(--border);
  background: color-mix(in srgb, var(--card) 92%, var(--background));
}

.brand {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  min-height: 2.4rem;
  padding: 0.2rem 0.55rem;
  color: var(--foreground);
  text-decoration: none;
}

.brand img {
  width: 1.65rem;
  height: 1.65rem;
  border-radius: 0.45rem;
}

.brand span {
  font-size: 0.9rem;
  font-weight: 650;
  letter-spacing: -0.02em;
}

.side-nav {
  display: grid;
  gap: 0.2rem;
  margin-top: 1.7rem;
}

.side-nav a {
  position: relative;
  display: flex;
  align-items: center;
  gap: 0.65rem;
  min-height: 2.4rem;
  padding: 0.45rem 0.7rem;
  border-radius: 0.55rem;
  color: var(--muted-foreground);
  font-size: 0.8rem;
  font-weight: 520;
  text-decoration: none;
  transition: color 150ms ease, background 150ms ease, transform 100ms ease;
}

.side-nav a:hover {
  background: var(--accent);
  color: var(--foreground);
}

.side-nav a:active {
  transform: scale(0.985);
}

.side-nav a.active {
  background: var(--secondary);
  color: var(--foreground);
  font-weight: 620;
}

.side-nav a.active::before {
  content: '';
  position: absolute;
  left: -0.8rem;
  top: 0.55rem;
  bottom: 0.55rem;
  width: 2px;
  border-radius: 999px;
  background: var(--foreground);
}

.side-nav a.active > svg {
  color: var(--foreground);
}

.side-foot {
  display: grid;
  gap: 0.1rem;
  margin-top: auto;
  padding-top: 0.65rem;
  border-top: 1px solid var(--border);
}

.side-link {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  min-height: 2rem;
  padding: 0.4rem 0.6rem;
  border-radius: 0.6rem;
  color: var(--muted-foreground);
  font-size: 0.74rem;
  text-decoration: none;
}

.side-link:hover {
  background: var(--accent);
  color: var(--foreground);
}

.side-link svg:last-child {
  margin-left: auto;
  color: var(--muted-foreground);
}

/* ---------- workspace / topbar ---------- */

.workspace {
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.topbar {
  position: sticky;
  z-index: 30;
  top: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  min-height: 3.4rem;
  padding: 0.45rem 1.4rem;
  border-bottom: 1px solid var(--border);
  background: color-mix(in srgb, var(--background) 88%, transparent);
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
}

.topbar-context {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 0.55rem;
}

.topbar-context img {
  display: none;
  width: 1.55rem;
  height: 1.55rem;
  border-radius: 0.4rem;
}

.topbar-title {
  color: var(--muted-foreground);
  font-size: 0.76rem;
  font-weight: 620;
}

.topbar-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.chip {
  height: 1.7rem;
  display: inline-flex;
  align-items: center;
  gap: 0.32rem;
  padding: 0 0.58rem;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: transparent;
  color: var(--muted-foreground);
  font-size: 0.66rem;
  font-weight: 620;
  white-space: nowrap;
}

.chip i {
  width: 0.4rem;
  height: 0.4rem;
  border-radius: 50%;
  background: currentColor;
}

.chip-ok {
  border-color: color-mix(in srgb, var(--success) 24%, transparent);
  color: var(--success);
  background: color-mix(in srgb, var(--success) 7%, transparent);
}

.chip-bad {
  border-color: color-mix(in srgb, var(--destructive) 24%, transparent);
  color: var(--destructive);
  background: color-mix(in srgb, var(--destructive) 7%, transparent);
}

.chip-warn {
  border-color: color-mix(in srgb, var(--warning) 25%, transparent);
  color: var(--warning);
  background: color-mix(in srgb, var(--warning) 8%, transparent);
}

.icon-btn {
  width: 2rem;
  height: 2rem;
  display: grid;
  place-items: center;
  border: 1px solid transparent;
  border-radius: 0.5rem;
  background: transparent;
  color: var(--muted-foreground);
  transition: color 150ms ease, background 150ms ease, transform 100ms ease;
}

.icon-btn:hover {
  border-color: var(--border);
  background: var(--card);
  color: var(--foreground);
}

.icon-btn:active {
  transform: scale(0.95);
}

/* ---------- content ---------- */

.content {
  flex: 1;
  width: min(100%, 72rem);
  margin: 0 auto;
  padding: 2rem 1.6rem 3rem;
}

.page-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1.5rem;
  margin-bottom: 1.55rem;
}

.page-heading {
  min-width: 0;
}

.page-eyebrow {
  margin: 0 0 0.35rem;
  color: var(--muted-foreground);
  font-size: 0.62rem;
  font-weight: 650;
  letter-spacing: 0.13em;
}

.page-head h1 {
  margin: 0;
  font-size: clamp(1.65rem, 3vw, 2rem);
  font-weight: 650;
  letter-spacing: -0.035em;
  line-height: 1.05;
}

.page-description {
  margin: 0.42rem 0 0;
  color: var(--muted-foreground);
  font-size: 0.76rem;
}

.page-meta {
  display: grid;
  justify-items: end;
  gap: 0.28rem;
  min-width: 0;
  color: var(--muted-foreground);
}

.page-meta > span {
  font-size: 0.62rem;
  font-weight: 620;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.page-meta small {
  max-width: 20rem;
  overflow: hidden;
  color: var(--muted-foreground);
  font-size: 0.66rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.page-alert {
  margin-bottom: 0.9rem;
}

.page-alert :deep([data-slot='alert-description']) {
  color: inherit;
}

.view {
  min-width: 0;
  animation: view-in 0.2s cubic-bezier(0.22, 1, 0.36, 1) both;
}

@keyframes view-in {
  from {
    opacity: 0;
    transform: translateY(4px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}

/* ---------- mobile bottom tab bar ---------- */

.tabbar {
  display: none;
}

@media (max-width: 900px) {
  .shell {
    display: block;
  }

  .sidebar {
    display: none;
  }

  .topbar {
    padding-inline: 0.9rem;
  }

  .topbar-context img {
    display: block;
  }

  .topbar-title {
    display: block;
    color: var(--foreground);
  }

  .content {
    padding: 1.5rem 0.9rem calc(5.2rem + env(safe-area-inset-bottom));
  }

  .page-head {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.5rem;
  }

  .page-meta {
    justify-items: start;
  }

  .page-meta small {
    max-width: 100%;
  }

  .tabbar {
    position: fixed;
    z-index: 40;
    left: 0;
    right: 0;
    bottom: 0;
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
    padding: 0.35rem 0.55rem calc(0.35rem + env(safe-area-inset-bottom));
    border-top: 1px solid var(--border);
    background: color-mix(in srgb, var(--card) 90%, transparent);
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
  }

  .tabbar a {
    display: grid;
    justify-items: center;
    gap: 0.14rem;
    min-width: 0;
    padding: 0.32rem 0;
    border-radius: 0.5rem;
    color: var(--muted-foreground);
    font-size: 0.6rem;
    font-weight: 520;
    text-decoration: none;
    transition: color 150ms ease, background 150ms ease, transform 100ms ease;
  }

  .tabbar a:active {
    transform: scale(0.96);
  }

  .tabbar a.active {
    background: var(--secondary);
    color: var(--foreground);
    font-weight: 620;
  }
}

@media (max-width: 600px) {
  .content {
    padding-inline: 0.8rem;
  }

  .page-head {
    margin-bottom: 1.25rem;
  }

  .page-description {
    max-width: 28rem;
  }

  .chip span {
    display: none;
  }

  .chip {
    padding: 0 0.55rem;
  }
}

@media (max-width: 390px) {
  .topbar {
    gap: 0.45rem;
    padding-inline: 0.65rem;
  }

  .topbar-context {
    gap: 0.4rem;
  }

  .topbar-title {
    font-size: 0.72rem;
  }

  .topbar-actions {
    gap: 0.25rem;
  }

  .chip {
    height: 1.65rem;
    padding-inline: 0.45rem;
  }

  .content {
    padding-inline: 0.65rem;
  }
}

@media (prefers-reduced-motion: reduce) {
  .view {
    animation: none;
  }
}
</style>
