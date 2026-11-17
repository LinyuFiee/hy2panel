<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import {
  Activity,
  AlertTriangle,
  ArrowDownToLine,
  ArrowUpFromLine,
  Cpu,
  Database,
  ExternalLink,
  HardDrive,
  HeartPulse,
  LogOut,
  MemoryStick,
  Menu,
  Network,
  ShieldCheck,
  Wrench
} from '@lucide/vue'
import { toast } from 'vue-sonner'
import DashboardView from './components/DashboardView.vue'
import NodesView from './components/NodesView.vue'
import MaintenanceView from './components/MaintenanceView.vue'
import SettingsView from './components/SettingsView.vue'
import ThemeToggle from './components/ThemeToggle.vue'
import HairlineFigure from './components/HairlineFigure.vue'
import GithubIcon from './components/GithubIcon.vue'
import LoginPage from './pages/LoginPage.vue'
import { Alert, AlertDescription, AlertTitle } from './components/ui/alert'
import { Button } from './components/ui/button'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger
} from './components/ui/sheet'
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
import { formatBytes, formatRate, percent, sumBytes } from './utils'

const view = ref('loading')
const activeTab = ref(DEFAULT_SECTION)
const password = ref('')
const error = ref('')
const notice = ref('')
const loginBusy = ref(false)
const mobileNavOpen = ref(false)
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
const overviewMemoryPercent = computed(() => percent(overviewStatus.value.memory_used_mb, overviewStatus.value.memory_total_mb))
const overviewDiskPercent = computed(() => percent(overviewStatus.value.disk_used_bytes, overviewStatus.value.disk_total_bytes))
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
  mobileNavOpen.value = false
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
  await Promise.all([refreshSystemStatus(), refreshTrafficLive(), loadTrafficHistory(trafficHistoryRange.value)])
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
    mobileNavOpen.value = false
    activeTab.value = DEFAULT_SECTION
    view.value = 'login'
  }
}

function switchTab(tab, { updateHash = true } = {}) {
  const section = findNavItem(tab).id
  activeTab.value = section
  window.scrollTo({ top: 0, left: 0, behavior: 'auto' })
  mobileNavOpen.value = false
  error.value = ''
  notice.value = ''
  if (updateHash) ensureSectionHash(section)
  if (section === 'overview' || section === 'traffic') loadTrafficHistory(trafficHistoryRange.value)
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
      when: () => view.value === 'app' && !document.hidden,
      run: () => {
        refreshSystemStatus()
        refreshTrafficLive()
      }
    },
    { id: 'history', interval: 60000, when: () => view.value === 'app' && ['overview', 'traffic'].includes(activeTab.value) && !document.hidden, run: () => loadTrafficHistory(trafficHistoryRange.value) },
    { id: 'sing-box', interval: 3000, when: () => view.value === 'app' && activeTab.value === 'settings' && !document.hidden, run: () => loadSingBoxVersions() }
  ])
}

function handleVisibilityChange() {
  if (document.hidden || authStopped || view.value !== 'app') return
  refresh(false, false)
  refreshSystemStatus()
  refreshTrafficLive()
  if (['overview', 'traffic'].includes(activeTab.value)) loadTrafficHistory(trafficHistoryRange.value)
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

  <main v-if="view === 'loading'" id="main-content" class="control-loading" tabindex="-1" aria-live="polite">
    <div class="control-loading-mark"><img src="/logo.png" alt="" /></div>
    <Spinner label="正在连接控制平面" class="control-loading-spinner" />
    <div><strong>正在连接控制平面</strong><span>读取服务状态与节点配置</span></div>
  </main>

  <LoginPage
    v-else-if="view === 'login'"
    v-model:password="password"
    :error="error"
    :busy="loginBusy"
    @submit="login"
  />

  <div v-else class="control-shell">
    <aside class="control-sidebar" aria-label="主导航">
      <a class="control-brand" :href="hashForSection('overview')" @click.prevent="switchTab('overview')">
        <img src="/logo.png" alt="" />
        <span><strong>hy2panel</strong><small>CONTROL PLANE</small></span>
      </a>

      <nav class="control-nav" aria-label="控制台分区">
        <a
          v-for="item in NAV_ITEMS"
          :key="item.id"
          :href="hashForSection(item.id)"
          :class="{ active: activeTab === item.id }"
          :aria-current="activeTab === item.id ? 'page' : undefined"
          @click.prevent="switchTab(item.id)"
        >
          <component :is="item.icon" :size="17" />
          <span>{{ item.label }}</span>
          <i aria-hidden="true"></i>
        </a>
      </nav>

      <div class="control-sidebar-footer">
        <div class="sidebar-health">
          <span class="health-orbit" :class="{ warn: !serviceHealthy }"><i></i></span>
          <div><strong>{{ serviceHealthy ? '系统正常' : '需要检查' }}</strong><span>sing-box {{ serviceHealthy ? 'active' : 'inactive' }}</span></div>
        </div>
        <a class="sidebar-external" href="https://github.com/LinyuFiee/hy2panel" target="_blank" rel="noopener noreferrer">
          <GithubIcon /><span>GitHub</span><ExternalLink :size="12" />
        </a>
        <button class="sidebar-logout" type="button" @click="logout"><LogOut :size="15" /><span>退出登录</span></button>
      </div>
    </aside>

    <div class="control-workspace">
      <header class="control-topbar">
        <div class="mobile-brand-group">
          <Sheet v-model:open="mobileNavOpen">
            <SheetTrigger as-child>
              <Button variant="outline" size="icon" class="mobile-menu-button" aria-label="打开导航菜单"><Menu :size="18" /></Button>
            </SheetTrigger>
            <SheetContent side="left" class="mobile-nav-sheet" :show-close-button="true">
              <SheetHeader class="mobile-sheet-header">
                <SheetTitle class="mobile-sheet-title"><img src="/logo.png" alt="" />hy2panel</SheetTitle>
                <SheetDescription>私有节点控制平面</SheetDescription>
              </SheetHeader>
              <nav class="mobile-sheet-nav" aria-label="移动端控制台分区">
                <SheetClose v-for="item in NAV_ITEMS" :key="item.id" as-child>
                  <a
                    :href="hashForSection(item.id)"
                    :class="{ active: activeTab === item.id }"
                    :aria-current="activeTab === item.id ? 'page' : undefined"
                    @click="switchTab(item.id, { updateHash: false })"
                  >
                    <component :is="item.icon" :size="18" />
                    <span><strong>{{ item.label }}</strong><small>{{ item.description }}</small></span>
                  </a>
                </SheetClose>
              </nav>
              <div class="mobile-sheet-footer">
                <div class="mobile-panel-address"><span>面板地址</span><strong>{{ panelAddress }}</strong></div>
                <ThemeToggle label="界面主题" />
                <a href="https://github.com/LinyuFiee/hy2panel" target="_blank" rel="noopener noreferrer"><GithubIcon />GitHub<ExternalLink :size="13" /></a>
                <button type="button" @click="logout"><LogOut :size="16" />退出登录</button>
              </div>
            </SheetContent>
          </Sheet>
          <a class="mobile-control-brand" :href="hashForSection('overview')" @click.prevent="switchTab('overview')"><img src="/logo.png" alt="" /><strong>hy2panel</strong></a>
        </div>

        <div class="control-breadcrumb" aria-label="当前位置">
          <span>控制台</span><i>/</i><strong>{{ currentNavItem.shortLabel }}</strong>
        </div>

        <div class="control-topbar-actions">
          <div class="topbar-status" :class="{ warn: !serviceHealthy }" :title="serviceHealthy ? 'sing-box 服务运行正常' : 'sing-box 服务需要检查'">
            <HeartPulse :size="14" /><span>{{ serviceHealthy ? '健康' : '异常' }}</span>
          </div>
          <div class="topbar-status maintenance" :class="{ active: maintenanceBusy }" :title="maintenanceLabel">
            <Wrench :size="14" /><span>{{ maintenanceBusy ? '维护中' : '空闲' }}</span>
          </div>
          <ThemeToggle compact />
        </div>
      </header>

      <main id="main-content" class="control-main" tabindex="-1">
        <div class="route-announcement sr-only" aria-live="polite">{{ routeAnnouncement }}</div>
        <header class="control-page-heading">
          <div>
            <span>{{ currentNavItem.eyebrow }}</span>
            <h1>{{ currentNavItem.title }}</h1>
            <p>{{ currentNavItem.description }}</p>
          </div>
          <div class="control-page-meta">
            <span :class="serviceHealthy ? 'ok' : 'warn'"><i></i>{{ serviceHealthy ? 'sing-box active' : 'sing-box inactive' }}</span>
            <small>{{ panelAddress }}</small>
          </div>
        </header>

        <Alert v-if="error" variant="destructive" class="control-alert" aria-live="assertive">
          <AlertTriangle />
          <AlertTitle>操作未完成</AlertTitle>
          <AlertDescription>{{ error }}</AlertDescription>
        </Alert>
        <Alert v-if="notice" variant="success" class="control-alert" aria-live="polite">
          <ShieldCheck />
          <AlertTitle>操作已完成</AlertTitle>
          <AlertDescription>{{ notice }}</AlertDescription>
        </Alert>

        <div class="control-view" :data-section="activeTab">
          <div v-if="activeTab === 'overview'" class="overview-shell-grid">
            <section class="overview-stat-grid" aria-label="主机关键指标">
              <article class="overview-stat-card">
                <div class="overview-stat-icon mint"><Cpu :size="18" /></div>
                <span>CPU 使用率</span>
                <strong>{{ Number(overviewStatus.cpu_percent || 0).toFixed(1) }}%</strong>
                <div class="overview-stat-track"><i :style="{ width: `${Math.min(Number(overviewStatus.cpu_percent || 0), 100)}%` }"></i></div>
                <small>负载 {{ overviewStatus.load_average || '—' }}</small>
              </article>
              <article class="overview-stat-card">
                <div class="overview-stat-icon violet"><MemoryStick :size="18" /></div>
                <span>内存占用</span>
                <strong>{{ overviewStatus.memory_used_mb || 0 }} MB</strong>
                <div class="overview-stat-track violet"><i :style="{ width: `${overviewMemoryPercent}%` }"></i></div>
                <small>共 {{ overviewStatus.memory_total_mb || 0 }} MB · {{ overviewMemoryPercent.toFixed(0) }}%</small>
              </article>
              <article class="overview-stat-card">
                <div class="overview-stat-icon amber"><HardDrive :size="18" /></div>
                <span>系统磁盘</span>
                <strong>{{ formatBytes(overviewStatus.disk_used_bytes) }}</strong>
                <div class="overview-stat-track amber"><i :style="{ width: `${overviewDiskPercent}%` }"></i></div>
                <small>共 {{ formatBytes(overviewStatus.disk_total_bytes) }} · {{ overviewDiskPercent.toFixed(0) }}%</small>
              </article>
              <article class="overview-stat-card">
                <div class="overview-stat-icon blue"><Network :size="18" /></div>
                <span>主机实时速率</span>
                <strong>{{ formatRate(sumBytes(overviewStatus.rx_bytes_per_second, overviewStatus.tx_bytes_per_second)) }}</strong>
                <div class="overview-rate-pair"><small><ArrowDownToLine :size="12" />{{ formatRate(overviewStatus.rx_bytes_per_second) }}</small><small><ArrowUpFromLine :size="12" />{{ formatRate(overviewStatus.tx_bytes_per_second) }}</small></div>
              </article>
            </section>
            <section class="overview-visual-card">
              <div class="overview-visual-copy">
                <span>DATA PATH</span>
                <h2>服务器路径</h2>
                <p>入口、配线端口、服务器刀片与出口构成单一数据链路。</p>
              </div>
              <HairlineFigure :play="false" :intensity="0.42" theme="auto" density="compact" label="服务器机架实时路径：入口、配线端口、服务器刀片与出口" />
            </section>
            <section class="overview-quick-grid">
              <article class="overview-quick-card">
                <div><span>PROTOCOL STATUS</span><h2>节点协议</h2></div>
                <div class="overview-protocol-list">
                  <div><i :class="{ active: state.config.reality?.enabled }"></i><span><strong>Reality</strong><small>TCP · {{ state.config.reality?.port || '—' }}</small></span><em>{{ state.config.reality?.enabled ? '在线' : '停用' }}</em></div>
                  <div><i :class="{ active: state.config.hysteria2?.enabled }"></i><span><strong>Hysteria2</strong><small>UDP · {{ state.config.hysteria2?.port || '—' }}</small></span><em>{{ state.config.hysteria2?.enabled ? '在线' : '停用' }}</em></div>
                </div>
                <button type="button" @click="switchTab('nodes')">管理节点 <ExternalLink :size="14" /></button>
              </article>
              <article class="overview-quick-card">
                <div><span>TRAFFIC SNAPSHOT</span><h2>流量速览</h2></div>
                <div class="overview-traffic-total"><Database :size="20" /><span><strong>{{ overviewRackNerd.info ? formatBytes(overviewRackNerd.info.bandwidth_used) : '未连接' }}</strong><small>{{ overviewRackNerd.info ? `本月已用 · ${Number(overviewRackNerd.info.percent_used || 0).toFixed(1)}%` : '配置 RackNerd 后显示套餐流量' }}</small></span></div>
                <button type="button" @click="switchTab('traffic')">查看流量分析 <ExternalLink :size="14" /></button>
              </article>
              <article class="overview-quick-card health-card">
                <div><span>SYSTEM READINESS</span><h2>运行状态</h2></div>
                <div class="overview-health-readout"><Activity :size="21" /><span><strong>{{ serviceHealthy ? '服务就绪' : '服务异常' }}</strong><small>{{ maintenanceBusy ? '维护任务正在运行' : '当前没有维护任务' }}</small></span></div>
                <button type="button" @click="switchTab('maintenance')">打开维护中心 <ExternalLink :size="14" /></button>
              </article>
            </section>
          </div>
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
  padding: 0.65rem 0.9rem;
  border-radius: 0.5rem;
  background: var(--foreground);
  color: var(--background);
  font-size: 0.8rem;
  font-weight: 700;
  text-decoration: none;
  transition: transform 0.15s ease;
}

.skip-link:focus {
  transform: translateY(0);
}

.control-loading {
  min-height: 100svh;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.9rem;
  background:
    radial-gradient(circle at 50% 45%, color-mix(in oklab, var(--primary) 12%, transparent), transparent 22rem),
    var(--background);
  color: var(--foreground);
}

.control-loading:focus,
.control-main:focus {
  outline: none;
}

.control-loading-mark {
  width: 2.75rem;
  height: 2.75rem;
  display: grid;
  place-items: center;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 0.75rem;
  background: var(--card);
}

.control-loading-mark img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.control-loading-spinner {
  color: var(--primary);
}

.control-loading strong,
.control-loading span {
  display: block;
}

.control-loading strong {
  font-size: 0.82rem;
}

.control-loading span {
  margin-top: 0.18rem;
  color: var(--muted-foreground);
  font-size: 0.68rem;
}

.control-shell {
  min-height: 100svh;
  display: grid;
  grid-template-columns: 12.5rem minmax(0, 1fr);
  background: var(--background);
  color: var(--foreground);
}

.control-sidebar {
  position: sticky;
  z-index: 30;
  top: 0;
  height: 100svh;
  display: flex;
  flex-direction: column;
  padding: 1.1rem 0.8rem 0.8rem;
  border-right: 1px solid var(--border);
  background: color-mix(in oklab, var(--card) 91%, var(--background));
}

.control-brand {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  min-width: 0;
  padding: 0.25rem 0.45rem;
  color: var(--foreground);
  text-decoration: none;
}

.control-brand img,
.mobile-control-brand img,
.mobile-sheet-title img {
  width: 2rem;
  height: 2rem;
  flex: 0 0 auto;
  border: 1px solid var(--border);
  border-radius: 0.58rem;
  background: var(--background);
  object-fit: contain;
}

.control-brand span,
.control-brand strong,
.control-brand small {
  display: block;
  min-width: 0;
}

.control-brand strong {
  font-size: 0.85rem;
  letter-spacing: -0.02em;
}

.control-brand small {
  margin-top: 0.1rem;
  color: var(--muted-foreground);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.5rem;
  letter-spacing: 0.12em;
}

.control-nav {
  display: grid;
  gap: 0.2rem;
  margin-top: 2rem;
}

.control-nav a {
  position: relative;
  display: grid;
  grid-template-columns: 1.25rem 1fr auto;
  align-items: center;
  gap: 0.55rem;
  min-height: 2.45rem;
  padding: 0.55rem 0.7rem;
  border: 1px solid transparent;
  border-radius: 0.55rem;
  color: var(--muted-foreground);
  font-size: 0.76rem;
  font-weight: 570;
  text-decoration: none;
  transition: background 0.14s ease, border-color 0.14s ease, color 0.14s ease;
}

.control-nav a:hover {
  background: var(--muted);
  color: var(--foreground);
}

.control-nav a.active {
  border-color: color-mix(in oklab, var(--primary) 18%, var(--border));
  background: color-mix(in oklab, var(--primary) 9%, var(--card));
  color: var(--foreground);
  box-shadow: inset 2px 0 0 var(--primary);
}

.control-nav a.active > svg {
  color: var(--primary);
}

.control-nav a i {
  width: 0.3rem;
  height: 0.3rem;
  border-radius: 50%;
  background: transparent;
}

.control-nav a.active i {
  background: var(--primary);
}

.control-sidebar-footer {
  display: grid;
  gap: 0.3rem;
  margin-top: auto;
  padding-top: 1rem;
  border-top: 1px solid var(--border);
}

.sidebar-health {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.55rem 0.65rem 0.75rem;
}

.health-orbit {
  width: 1.75rem;
  height: 1.75rem;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  border: 1px solid color-mix(in oklab, var(--primary) 30%, var(--border));
  border-radius: 50%;
  background: color-mix(in oklab, var(--primary) 8%, transparent);
}

.health-orbit i {
  width: 0.42rem;
  height: 0.42rem;
  border-radius: 50%;
  background: var(--primary);
  box-shadow: 0 0 0 0.2rem color-mix(in oklab, var(--primary) 12%, transparent);
}

.health-orbit.warn {
  border-color: color-mix(in oklab, var(--destructive) 35%, var(--border));
  background: color-mix(in oklab, var(--destructive) 9%, transparent);
}

.health-orbit.warn i {
  background: var(--destructive);
  box-shadow: 0 0 0 0.2rem color-mix(in oklab, var(--destructive) 12%, transparent);
}

.sidebar-health strong,
.sidebar-health span {
  display: block;
}

.sidebar-health strong {
  font-size: 0.66rem;
}

.sidebar-health span {
  margin-top: 0.14rem;
  color: var(--muted-foreground);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.52rem;
}

.sidebar-external,
.sidebar-logout {
  width: 100%;
  display: grid;
  grid-template-columns: 1rem 1fr auto;
  align-items: center;
  gap: 0.5rem;
  min-height: 2.1rem;
  padding: 0.45rem 0.65rem;
  border: 0;
  border-radius: 0.45rem;
  background: transparent;
  color: var(--muted-foreground);
  font-size: 0.68rem;
  text-align: left;
  text-decoration: none;
}

.sidebar-external:hover,
.sidebar-logout:hover {
  background: var(--muted);
  color: var(--foreground);
}

.sidebar-logout:hover {
  color: var(--destructive);
}

.control-workspace {
  min-width: 0;
  overflow-x: clip;
}

.control-topbar {
  position: sticky;
  z-index: 20;
  top: 0;
  min-height: 3.5rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.6rem 1.5rem;
  border-bottom: 1px solid var(--border);
  background: color-mix(in oklab, var(--background) 88%, transparent);
  backdrop-filter: blur(16px);
}

.control-breadcrumb {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  color: var(--muted-foreground);
  font-size: 0.68rem;
}

.control-breadcrumb i {
  color: var(--border);
  font-style: normal;
}

.control-breadcrumb strong {
  color: var(--foreground);
  font-weight: 650;
}

.control-topbar-actions {
  display: flex;
  align-items: center;
  gap: 0.45rem;
}

.topbar-status {
  height: 1.95rem;
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0 0.65rem;
  border: 1px solid color-mix(in oklab, var(--primary) 20%, var(--border));
  border-radius: 0.55rem;
  background: color-mix(in oklab, var(--primary) 6%, transparent);
  color: var(--primary);
  font-size: 0.64rem;
  font-weight: 650;
}

.topbar-status.warn {
  border-color: color-mix(in oklab, var(--destructive) 26%, var(--border));
  background: color-mix(in oklab, var(--destructive) 7%, transparent);
  color: var(--destructive);
}

.topbar-status.maintenance {
  border-color: var(--border);
  background: var(--card);
  color: var(--muted-foreground);
}

.topbar-status.maintenance.active {
  border-color: color-mix(in oklab, #d49a35 30%, var(--border));
  background: color-mix(in oklab, #d49a35 8%, transparent);
  color: #9a6812;
}

:global(.dark) .topbar-status.maintenance.active {
  color: #f0bd61;
}

.mobile-brand-group {
  display: none;
}

.control-main {
  width: min(100%, 94rem);
  min-width: 0;
  overflow-x: clip;
  margin: 0 auto;
  padding: clamp(1.4rem, 2.8vw, 2.75rem) clamp(1rem, 2.8vw, 2.75rem) 4rem;
}

.control-page-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 2rem;
  margin-bottom: 1.6rem;
  padding-bottom: 1.35rem;
  border-bottom: 1px solid var(--border);
}

.control-page-heading > div:first-child > span {
  display: block;
  margin-bottom: 0.45rem;
  color: var(--primary);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.58rem;
  font-weight: 720;
  letter-spacing: 0.14em;
}

.control-page-heading h1 {
  margin: 0;
  color: var(--foreground);
  font-size: clamp(1.65rem, 2.7vw, 2.2rem);
  font-weight: 680;
  letter-spacing: -0.045em;
}

.control-page-heading p {
  margin: 0.45rem 0 0;
  color: var(--muted-foreground);
  font-size: 0.78rem;
  line-height: 1.6;
}

.control-page-meta {
  display: grid;
  justify-items: end;
  gap: 0.45rem;
  min-width: 0;
}

.control-page-meta > span {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  color: var(--primary);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.62rem;
  font-weight: 650;
}

.control-page-meta > span.warn {
  color: var(--destructive);
}

.control-page-meta i {
  width: 0.38rem;
  height: 0.38rem;
  border-radius: 50%;
  background: currentColor;
  box-shadow: 0 0 0 0.2rem color-mix(in oklab, currentColor 12%, transparent);
}

.control-page-meta small {
  max-width: 20rem;
  overflow: hidden;
  color: var(--muted-foreground);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.58rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.control-alert {
  margin-bottom: 1rem;
}

.control-alert :deep([data-slot='alert-description']) {
  color: inherit;
}

.control-view {
  min-width: 0;
}

.overview-shell-grid {
  display: grid;
  gap: 1rem;
  animation: control-view-enter 0.2s ease-out both;
}

.overview-stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.8rem;
}

.overview-stat-card,
.overview-quick-card {
  min-width: 0;
  border: 1px solid var(--border);
  border-radius: 0.85rem;
  background: var(--card);
  box-shadow: 0 0.4rem 1.5rem color-mix(in oklab, var(--foreground) 4%, transparent);
}

.overview-stat-card {
  display: grid;
  grid-template-columns: auto 1fr;
  align-items: center;
  gap: 0.35rem 0.65rem;
  padding: 1rem;
}

.overview-stat-icon {
  grid-row: 1 / 3;
  width: 2.2rem;
  height: 2.2rem;
  display: grid;
  place-items: center;
  border-radius: 0.6rem;
  background: color-mix(in oklab, var(--primary) 12%, var(--card));
  color: var(--primary);
}

.overview-stat-icon.violet {
  background: color-mix(in oklab, #8c84b8 13%, var(--card));
  color: #8c84b8;
}

.overview-stat-icon.amber {
  background: color-mix(in oklab, #d49578 13%, var(--card));
  color: #c57d5b;
}

.overview-stat-icon.blue {
  background: color-mix(in oklab, #6b9eb4 13%, var(--card));
  color: #5c90a8;
}

.overview-stat-card > span {
  color: var(--muted-foreground);
  font-size: 0.65rem;
}

.overview-stat-card > strong {
  color: var(--foreground);
  font-size: 1.05rem;
  letter-spacing: -0.025em;
}

.overview-stat-card > small,
.overview-rate-pair {
  grid-column: 1 / -1;
  color: var(--muted-foreground);
  font-size: 0.58rem;
}

.overview-stat-track {
  grid-column: 1 / -1;
  height: 0.22rem;
  overflow: hidden;
  margin-top: 0.45rem;
  border-radius: 2rem;
  background: var(--muted);
}

.overview-stat-track i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--primary);
}

.overview-stat-track.violet i { background: #8c84b8; }
.overview-stat-track.amber i { background: #d49578; }

.overview-rate-pair {
  display: flex;
  justify-content: space-between;
  gap: 0.5rem;
  margin-top: 0.5rem;
}

.overview-rate-pair small {
  display: inline-flex;
  align-items: center;
  gap: 0.2rem;
  color: var(--muted-foreground);
}

.overview-visual-card {
  min-height: 17rem;
  display: grid;
  grid-template-columns: minmax(13rem, .65fr) minmax(18rem, 1.35fr);
  align-items: center;
  overflow: hidden;
  padding: 1.25rem 1.4rem;
  border: 1px solid var(--border);
  border-radius: .9rem;
  background: linear-gradient(120deg, color-mix(in oklab, var(--primary) 8%, var(--card)), var(--card) 55%);
}

.overview-visual-copy span {
  color: var(--primary);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: .58rem;
  font-weight: 720;
  letter-spacing: .14em;
}

.overview-visual-copy h2 { margin: .42rem 0 0; font-size: 1.15rem; }
.overview-visual-copy p { max-width: 24rem; margin: .4rem 0 0; font-size: .72rem; line-height: 1.6; }
.overview-visual-card :deep(.hairline-figure) { min-height: 15rem; }

.overview-quick-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.8rem;
}

.overview-quick-card {
  display: flex;
  flex-direction: column;
  min-height: 15rem;
  padding: 1.15rem;
}

.overview-quick-card > div:first-child > span {
  color: var(--primary);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.54rem;
  font-weight: 700;
  letter-spacing: 0.12em;
}

.overview-quick-card h2 {
  margin: 0.35rem 0 0;
  color: var(--foreground);
  font-size: 1rem;
}

.overview-protocol-list {
  display: grid;
  gap: 0.55rem;
  margin: 1.1rem 0;
}

.overview-protocol-list > div {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 0.55rem;
  padding: 0.7rem;
  border: 1px solid var(--border);
  border-radius: 0.62rem;
  background: var(--background);
}

.overview-protocol-list i {
  width: 0.45rem;
  height: 0.45rem;
  border-radius: 50%;
  background: var(--muted-foreground);
}

.overview-protocol-list i.active {
  background: var(--primary);
  box-shadow: 0 0 0 0.2rem color-mix(in oklab, var(--primary) 12%, transparent);
}

.overview-protocol-list strong,
.overview-protocol-list small,
.overview-traffic-total strong,
.overview-traffic-total small,
.overview-health-readout strong,
.overview-health-readout small {
  display: block;
}

.overview-protocol-list strong {
  color: var(--foreground);
  font-size: 0.7rem;
}

.overview-protocol-list small,
.overview-traffic-total small,
.overview-health-readout small {
  margin-top: 0.18rem;
  color: var(--muted-foreground);
  font-size: 0.58rem;
}

.overview-protocol-list em {
  color: var(--muted-foreground);
  font-size: 0.58rem;
  font-style: normal;
}

.overview-traffic-total,
.overview-health-readout {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin: auto 0;
  padding: 1rem;
  border-radius: 0.7rem;
  background: var(--muted);
  color: var(--primary);
}

.overview-traffic-total strong,
.overview-health-readout strong {
  color: var(--foreground);
  font-size: 1.05rem;
}

.overview-quick-card > button {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: auto;
  padding: 0.65rem 0.75rem;
  border: 1px solid var(--border);
  border-radius: 0.55rem;
  background: transparent;
  color: var(--foreground);
  font-size: 0.66rem;
  font-weight: 650;
}

.overview-quick-card > button:hover {
  border-color: color-mix(in oklab, var(--primary) 35%, var(--border));
  background: color-mix(in oklab, var(--primary) 7%, transparent);
  color: var(--primary);
}

.control-view[data-section='traffic'] :deep(.welcome-card) {
  display: none;
}

.control-view :deep(.view-stack),
.control-view :deep(.settings-grid) {
  animation: control-view-enter 0.2s ease-out both;
}

.mobile-nav-sheet {
  width: min(22rem, 88vw) !important;
  gap: 0 !important;
  padding: 0 !important;
}

.mobile-sheet-header {
  padding: 1.2rem 1rem !important;
  border-bottom: 1px solid var(--border);
}

.mobile-sheet-title {
  display: flex;
  align-items: center;
  gap: 0.65rem;
}

.mobile-sheet-nav {
  display: grid;
  gap: 0.25rem;
  padding: 0.8rem;
}

.mobile-sheet-nav a {
  display: grid;
  grid-template-columns: 1.3rem 1fr;
  align-items: center;
  gap: 0.7rem;
  padding: 0.7rem;
  border: 1px solid transparent;
  border-radius: 0.65rem;
  color: var(--muted-foreground);
  text-decoration: none;
}

.mobile-sheet-nav a.active {
  border-color: color-mix(in oklab, var(--primary) 20%, var(--border));
  background: color-mix(in oklab, var(--primary) 8%, transparent);
  color: var(--foreground);
}

.mobile-sheet-nav a.active > svg {
  color: var(--primary);
}

.mobile-sheet-nav strong,
.mobile-sheet-nav small {
  display: block;
}

.mobile-sheet-nav strong {
  font-size: 0.78rem;
}

.mobile-sheet-nav small {
  margin-top: 0.2rem;
  color: var(--muted-foreground);
  font-size: 0.62rem;
  line-height: 1.35;
}

.mobile-sheet-footer {
  display: grid;
  gap: 0.65rem;
  margin-top: auto;
  padding: 1rem;
  border-top: 1px solid var(--border);
}

.mobile-panel-address {
  padding: 0.75rem;
  border: 1px solid var(--border);
  border-radius: 0.65rem;
  background: var(--muted);
}

.mobile-panel-address span,
.mobile-panel-address strong {
  display: block;
}

.mobile-panel-address span {
  color: var(--muted-foreground);
  font-size: 0.62rem;
}

.mobile-panel-address strong {
  margin-top: 0.25rem;
  overflow: hidden;
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.66rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mobile-sheet-footer > a,
.mobile-sheet-footer > button {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 0.55rem;
  padding: 0.7rem;
  border: 0;
  border-radius: 0.55rem;
  background: transparent;
  color: var(--muted-foreground);
  font-size: 0.72rem;
  text-align: left;
  text-decoration: none;
}

.mobile-sheet-footer > a svg:last-child {
  margin-left: auto;
}

.mobile-sheet-footer > a:hover,
.mobile-sheet-footer > button:hover {
  background: var(--muted);
  color: var(--foreground);
}

.mobile-sheet-footer > button:hover {
  color: var(--destructive);
}

@keyframes control-view-enter {
  from { opacity: 0; transform: translateY(0.25rem); }
  to { opacity: 1; transform: translateY(0); }
}

@media (max-width: 1100px) {
  .overview-stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .overview-visual-card {
    grid-template-columns: 1fr;
  }

  .overview-visual-copy {
    text-align: center;
  }

  .overview-visual-copy p {
    margin-inline: auto;
  }

  .overview-quick-grid {
    grid-template-columns: 1fr;
  }

  .overview-quick-card {
    min-height: 12rem;
  }
}

@media (max-width: 900px) {
  .control-shell {
    display: block;
  }

  .control-sidebar {
    display: none;
  }

  .control-topbar {
    padding-inline: 0.85rem;
  }

  .mobile-brand-group {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }

  .mobile-menu-button {
    width: 2rem;
    height: 2rem;
  }

  .mobile-control-brand {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    color: var(--foreground);
    font-size: 0.75rem;
    text-decoration: none;
  }

  .mobile-control-brand img {
    width: 1.75rem;
    height: 1.75rem;
  }

  .control-breadcrumb {
    display: none;
  }

  .control-main {
    padding-top: 1.5rem;
  }
}

@media (max-width: 600px) {
  .overview-stat-grid {
    grid-template-columns: 1fr;
  }

  .overview-visual-card {
    min-height: auto;
    display: grid;
    grid-template-columns: minmax(7rem, .65fr) minmax(0, 1.35fr);
    padding: .85rem;
  }

  .overview-visual-card :deep(.hairline-figure) {
    min-height: 11rem;
  }

  .control-topbar {
    min-height: 3.25rem;
    gap: 0.5rem;
  }

  .mobile-control-brand strong {
    display: none;
  }

  .topbar-status {
    width: 1.9rem;
    padding: 0;
    justify-content: center;
  }

  .topbar-status span {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
  }

  .control-topbar-actions {
    gap: 0.25rem;
  }

  .control-page-heading {
    align-items: flex-start;
    flex-direction: column;
    gap: 0.85rem;
  }

  .control-page-meta {
    width: 100%;
    align-items: center;
    grid-template-columns: auto minmax(0, 1fr);
    justify-items: start;
  }

  .control-page-meta small {
    width: 100%;
    max-width: 100%;
    overflow: hidden;
    text-align: right;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .overview-rate-pair {
    align-items: flex-start;
    flex-direction: column;
  }
}

@media (prefers-reduced-motion: reduce) {
  .control-view :deep(.view-stack),
  .control-view :deep(.settings-grid) {
    animation: none;
  }
}
</style>
