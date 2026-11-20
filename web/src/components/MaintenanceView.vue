<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import {
  Activity,
  AlertCircle,
  CheckCircle2,
  ChevronDown,
  CircleAlert,
  ClipboardCheck,
  Info,
  LoaderCircle,
  RefreshCw,
  RotateCcw,
  ShieldAlert,
  ShieldCheck
} from '@lucide/vue'
import {
  Badge,
  Button,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Switch,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger
} from '@/components/ui'

const props = defineProps({
  state: { type: Object, required: true },
  report: { type: Object, default: null },
  entries: { type: Array, default: () => [] },
  logUnit: { type: String, default: 'sing-box' },
  busy: { type: Object, required: true }
})

const emit = defineEmits(['diagnostics', 'logs', 'recovery'])

const logBox = ref(null)
const selectedLogUnit = ref(props.logUnit)
const pendingLogUnit = ref(null)
const recoveryValue = ref(Boolean(props.state.config?.recovery?.auto_port))
const pendingRecovery = ref(null)
const recoveryRequestObserved = ref(false)
const recoveryFeedback = ref('')
const openChecks = ref(new Set())
let recoveryFeedbackTimer

const logUnits = [
  { value: 'sing-box', label: 'sing-box', short: '核心' },
  { value: 'panel', label: '面板', short: '面板' },
  { value: 'watchdog', label: '看门狗', short: '守护' },
  { value: 'apply', label: '配置应用', short: '应用' }
]

const statusIcons = { pass: CheckCircle2, warn: CircleAlert, fail: AlertCircle, info: Info }
const statusText = { pass: '正常', warn: '提醒', fail: '异常', info: '说明' }
const badgeVariant = { pass: 'success', warn: 'warning', fail: 'destructive', info: 'outline' }
const reportTotal = computed(() => props.report?.checks?.length || 0)
const infoCount = computed(() => Math.max(0, reportTotal.value - Number(props.report?.pass || 0) - Number(props.report?.warn || 0) - Number(props.report?.fail || 0)))
const lastPortChange = computed(() => {
  const timestamp = props.state.config?.recovery?.last_port_change || 0
  return timestamp > 0 ? new Date(timestamp * 1000).toLocaleString() : '尚未自动切换'
})
const diagnosticTitle = computed(() => {
  if (!props.report) return '等待首次检查'
  if (props.report.fail > 0) return `${props.report.fail} 项需要立即处理`
  if (props.report.warn > 0) return `${props.report.warn} 项建议复核`
  return '关键检查均已通过'
})
const diagnosticTone = computed(() => {
  if (!props.report) return 'idle'
  if (props.report.fail > 0) return 'danger'
  if (props.report.warn > 0) return 'warning'
  return 'success'
})
const diagnosticIcon = computed(() => ({
  idle: ClipboardCheck,
  danger: ShieldAlert,
  warning: CircleAlert,
  success: CheckCircle2
})[diagnosticTone.value])
const recoveryStatus = computed(() => {
  if (props.busy.recovery || pendingRecovery.value !== null) return { tone: 'pending', text: '正在同步策略', icon: LoaderCircle }
  if (recoveryFeedback.value === 'reverted') return { tone: 'danger', text: '保存失败，显示已回滚', icon: RotateCcw }
  if (recoveryFeedback.value === 'saved') return { tone: 'success', text: '策略已同步', icon: CheckCircle2 }
  return recoveryValue.value
    ? { tone: 'success', text: '自动换端口已启用', icon: ShieldCheck }
    : { tone: 'neutral', text: '保守恢复模式', icon: ShieldCheck }
})

function setRecoveryFeedback(value) {
  window.clearTimeout(recoveryFeedbackTimer)
  recoveryFeedback.value = value
  recoveryFeedbackTimer = window.setTimeout(() => { recoveryFeedback.value = '' }, 5000)
}

function requestRecovery(value) {
  if (props.busy.recovery) return
  const enabled = value === true
  recoveryValue.value = enabled
  pendingRecovery.value = enabled
  recoveryRequestObserved.value = false
  recoveryFeedback.value = ''
  emit('recovery', enabled)
}

function requestLogs(value) {
  if (!value || props.busy.logs || value === props.logUnit) return
  selectedLogUnit.value = value
  pendingLogUnit.value = value
  emit('logs', value)
}

function refreshLogs() {
  if (!props.busy.logs) emit('logs', props.logUnit)
}

function setLogBox(element, unit) {
  if (unit === selectedLogUnit.value) logBox.value = element
}

function checkOpen(id) {
  return openChecks.value.has(id)
}

function setCheckOpen(id, open) {
  const next = new Set(openChecks.value)
  if (open) next.add(id)
  else next.delete(id)
  openChecks.value = next
}

watch(() => props.state.config?.recovery?.auto_port, value => {
  if (pendingRecovery.value === null || value === pendingRecovery.value) recoveryValue.value = Boolean(value)
})

watch(() => props.busy.recovery, async (busy, wasBusy) => {
  if (busy && pendingRecovery.value !== null) recoveryRequestObserved.value = true
  if (!busy && wasBusy && recoveryRequestObserved.value && pendingRecovery.value !== null) {
    await nextTick()
    const persisted = Boolean(props.state.config?.recovery?.auto_port)
    const saved = persisted === pendingRecovery.value
    recoveryValue.value = persisted
    pendingRecovery.value = null
    recoveryRequestObserved.value = false
    setRecoveryFeedback(saved ? 'saved' : 'reverted')
  }
})

watch(() => props.logUnit, async value => {
  selectedLogUnit.value = value
  pendingLogUnit.value = null
  await nextTick()
  if (logBox.value) logBox.value.scrollTop = logBox.value.scrollHeight
})

watch(() => props.busy.logs, (busy, wasBusy) => {
  if (!busy && wasBusy && pendingLogUnit.value && props.logUnit !== pendingLogUnit.value) {
    selectedLogUnit.value = props.logUnit
    pendingLogUnit.value = null
  }
})

watch(() => props.entries, async () => {
  await nextTick()
  if (logBox.value) logBox.value.scrollTop = logBox.value.scrollHeight
}, { deep: true })

watch(() => props.report, report => {
  openChecks.value = new Set((report?.checks || []).filter(check => check.status === 'fail' || check.status === 'warn').map(check => check.id))
}, { immediate: true })

onBeforeUnmount(() => window.clearTimeout(recoveryFeedbackTimer))
</script>

<template>
  <div class="maint">
    <div class="grid-2">
      <article class="card">
        <header class="card-head">
          <div class="card-title">
            <h3>恢复策略</h3>
            <small>进程崩溃由 systemd 自动重启；持续监听异常按低频策略处理。</small>
          </div>
          <Badge variant="outline" class="state-badge" :data-tone="recoveryStatus.tone" role="status" aria-live="polite">
            <component :is="recoveryStatus.icon" :size="13" :class="{ spin: recoveryStatus.tone === 'pending' }" />
            {{ recoveryStatus.text }}
          </Badge>
        </header>
        <div class="card-body rows">
          <div class="policy-row">
            <div class="policy-copy">
              <div class="policy-title"><strong>进程崩溃自动重启</strong><Badge variant="success">始终启用</Badge></div>
              <p>systemd 立即拉起服务，看门狗每分钟复核实际监听。</p>
            </div>
          </div>
          <div class="policy-row is-switch">
            <div class="policy-copy">
              <strong>连续异常后自动换端口</strong>
              <p>连续 3 次监听失败才触发，24 小时内最多切换一次。换端口后需重新导入客户端链接。</p>
            </div>
            <Switch
              id="auto-port-recovery"
              class="policy-toggle"
              :model-value="recoveryValue"
              :disabled="busy.recovery"
              aria-label="连续异常后自动换端口"
              aria-describedby="recovery-boundary"
              @update:model-value="requestRecovery"
            />
          </div>
          <div id="recovery-boundary" class="boundary-note">
            <Info :size="15" />
            <p>本机只能可靠发现服务和监听异常；运营商或网络封堵需客户端或外部探测复核。</p>
          </div>
          <div class="foot-line"><span>上次自动切换</span><strong>{{ lastPortChange }}</strong></div>
        </div>
      </article>

      <article class="card diagnostic-card" :data-tone="diagnosticTone">
        <header class="card-head">
          <div class="card-title">
            <h3>服务端自检</h3>
            <small>只读检查配置、服务、监听、证书、DNS、BBR 与资源。</small>
          </div>
          <Badge v-if="report" variant="outline" class="tnum">{{ new Date(report.checked_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }}</Badge>
        </header>
        <div class="card-body diag-body">
          <div class="diag-hero">
            <div class="diag-icon"><component :is="diagnosticIcon" :size="22" /></div>
            <div class="diag-copy">
              <span>{{ report ? `${reportTotal} 项检查已完成` : '尚未运行自检' }}</span>
              <strong>{{ diagnosticTitle }}</strong>
            </div>
          </div>
          <div v-if="report" class="summary-counts" aria-label="诊断结果摘要">
            <div class="is-fail"><span>异常</span><strong class="tnum">{{ report.fail || 0 }}</strong></div>
            <div class="is-warn"><span>提醒</span><strong class="tnum">{{ report.warn || 0 }}</strong></div>
            <div class="is-pass"><span>正常</span><strong class="tnum">{{ report.pass || 0 }}</strong></div>
            <div class="is-info"><span>说明</span><strong class="tnum">{{ infoCount }}</strong></div>
          </div>
          <p v-else class="read-only-note"><Info :size="14" />只执行预设只读检查，不会改动节点配置。</p>
          <Button class="run-button" :disabled="busy.diagnostics" @click="emit('diagnostics')">
            <LoaderCircle v-if="busy.diagnostics" :size="15" class="spin" /><Activity v-else :size="15" />
            {{ busy.diagnostics ? '正在逐项检查…' : report ? '重新运行自检' : '开始自检' }}
          </Button>
        </div>
      </article>
    </div>

    <article v-if="report" class="card">
      <header class="card-head">
        <div class="card-title">
          <h3>检查结果</h3>
          <small>{{ new Date(report.checked_at).toLocaleString() }} · 点击每项展开详情</small>
        </div>
        <Badge variant="outline" class="tnum">{{ reportTotal }} 项</Badge>
      </header>
      <div class="card-body">
        <div class="diagnostic-list">
          <Collapsible
            v-for="check in report.checks"
            :key="check.id"
            :open="checkOpen(check.id)"
            class="diagnostic-item"
            :data-status="check.status"
            @update:open="setCheckOpen(check.id, $event)"
          >
            <CollapsibleTrigger class="diagnostic-trigger">
              <span class="check-icon"><component :is="statusIcons[check.status] || Info" :size="16" /></span>
              <span class="check-summary">
                <span class="check-title"><strong>{{ check.name }}</strong><Badge :variant="badgeVariant[check.status] || 'outline'">{{ statusText[check.status] || check.status }}</Badge></span>
                <span class="check-message">{{ check.message }}</span>
              </span>
              <ChevronDown :size="16" class="chevron" />
            </CollapsibleTrigger>
            <CollapsibleContent class="diagnostic-details">
              <div><span>检查结论</span><p>{{ check.message }}</p></div>
              <div><span>建议操作</span><p>{{ check.suggestion || '无需处理，保持当前配置并继续观察。' }}</p></div>
            </CollapsibleContent>
          </Collapsible>
        </div>
      </div>
    </article>

    <article class="card">
      <header class="card-head">
        <div class="card-title">
          <h3>运行日志</h3>
          <small>仅读取固定 systemd 单元，返回前已脱敏凭据。</small>
        </div>
        <Badge variant="outline">最多 300 行</Badge>
      </header>
      <div class="card-body">
        <Tabs :model-value="selectedLogUnit" activation-mode="manual" class="log-tabs-root" @update:model-value="requestLogs">
          <div class="log-toolbar">
            <div class="log-tab-strip">
              <TabsList class="log-tabs" aria-label="日志单元">
                <TabsTrigger v-for="unit in logUnits" :key="unit.value" :value="unit.value" :disabled="busy.logs" class="log-tab">
                  <span class="tab-full">{{ unit.label }}</span><span class="tab-short">{{ unit.short }}</span>
                </TabsTrigger>
              </TabsList>
            </div>
            <Button variant="outline" size="sm" class="refresh-button" :disabled="busy.logs" @click="refreshLogs">
              <RefreshCw :size="14" :class="{ spin: busy.logs }" /><span>{{ busy.logs ? '读取中' : '刷新' }}</span>
            </Button>
          </div>
          <TabsContent v-for="unit in logUnits" :key="unit.value" :value="unit.value" class="log-panel">
            <div class="terminal-shell">
              <div class="terminal-bar" aria-hidden="true">
                <span>journalctl · {{ unit.label }}</span><em>{{ unit.value === logUnit ? '已加载' : '待加载' }}</em>
              </div>
              <pre
                :ref="element => setLogBox(element, unit.value)"
                class="log-viewer"
                tabindex="0"
                role="log"
                aria-live="polite"
                :aria-busy="busy.logs"
                :aria-label="`${unit.label} 运行日志，可聚焦后滚动查看`"
              ><template v-if="busy.logs"><span class="log-empty"><LoaderCircle :size="14" class="spin" />正在读取 {{ unit.label }} 日志…</span></template><template v-else-if="unit.value !== logUnit"><span class="log-empty">按 Enter 或空格加载此单元日志。</span></template><template v-else-if="!entries.length"><span class="log-empty">当前单元暂无日志。</span></template><template v-else><code><span v-for="(entry, index) in entries" :key="`${logUnit}-${index}`" class="log-line"><span class="line-number" aria-hidden="true">{{ String(index + 1).padStart(3, '0') }}</span>{{ entry }}<br /></span></code></template></pre>
            </div>
          </TabsContent>
        </Tabs>
      </div>
    </article>
  </div>
</template>

<style scoped>
.maint {
  display: grid;
  gap: 0.85rem;
  width: 100%;
  min-width: 0;
}

.grid-2 {
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(18rem, 0.85fr);
  gap: 0.85rem;
  align-items: start;
}

.grid-2 > * {
  min-width: 0;
}

.card {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 0.75rem;
  background: var(--card);
  box-shadow: 0 1px 2px color-mix(in srgb, var(--foreground) 4%, transparent);
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.8rem;
  padding: 0.78rem 1rem;
  border-bottom: 1px solid var(--border);
}

.card-title {
  min-width: 0;
  display: grid;
  gap: 0.15rem;
}

.card-title h3 {
  margin: 0;
  font-size: 0.9rem;
  font-weight: 650;
  letter-spacing: -0.015em;
}

.card-title small {
  color: var(--muted-foreground);
  font-size: 0.68rem;
  line-height: 1.45;
}

.state-badge[data-tone='success'] {
  border-color: color-mix(in srgb, var(--success) 30%, transparent);
  background: color-mix(in srgb, var(--success) 11%, transparent);
  color: var(--success);
}

.state-badge[data-tone='danger'] {
  border-color: color-mix(in srgb, var(--destructive) 30%, transparent);
  background: color-mix(in srgb, var(--destructive) 11%, transparent);
  color: var(--destructive);
}

.state-badge[data-tone='pending'] {
  border-color: color-mix(in srgb, var(--warning) 30%, transparent);
  background: color-mix(in srgb, var(--warning) 13%, transparent);
  color: var(--warning);
}

.card-body {
  padding: 0.8rem 1rem 0.9rem;
}

/* ---------- recovery ---------- */

.rows {
  display: grid;
  gap: 0;
}

.policy-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.8rem;
  padding: 0.65rem 0;
  border-bottom: 1px solid var(--border);
}

.policy-row:first-child {
  padding-top: 0.15rem;
}

.policy-copy {
  min-width: 0;
  display: grid;
  gap: 0.25rem;
}

.policy-title {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.policy-copy strong {
  color: var(--foreground);
  font-size: 0.85rem;
  font-weight: 600;
}

.policy-copy p {
  margin: 0;
  color: var(--muted-foreground);
  font-size: 0.72rem;
  line-height: 1.55;
}

.policy-toggle {
  flex: 0 0 auto;
  justify-self: end;
}

.boundary-note {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  margin-top: 0.7rem;
  padding: 0.65rem 0.7rem;
  border: 1px solid var(--border);
  border-radius: 0.55rem;
  background: color-mix(in srgb, var(--muted) 58%, var(--card));
  color: var(--muted-foreground);
}

.boundary-note svg {
  flex: 0 0 auto;
  margin-top: 0.1rem;
  color: var(--warning);
}

.boundary-note p {
  margin: 0;
  font-size: 0.72rem;
  line-height: 1.55;
}

.foot-line {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.8rem;
  padding-top: 0.85rem;
  color: var(--muted-foreground);
  font-size: 0.74rem;
}

.foot-line strong {
  min-width: 0;
  color: var(--foreground);
  text-align: right;
  overflow-wrap: anywhere;
}

/* ---------- diagnostics ---------- */

.diag-body {
  display: grid;
  gap: 0.75rem;
}

.diag-hero {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.85rem;
  border-radius: 0.8rem;
  background: var(--muted);
}

.diag-icon {
  width: 2.6rem;
  height: 2.6rem;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  border-radius: 50%;
  background: color-mix(in srgb, var(--primary) 10%, transparent);
  color: var(--primary);
}

[data-tone='success'] .diag-icon {
  background: color-mix(in srgb, var(--success) 12%, transparent);
  color: var(--success);
}

[data-tone='warning'] .diag-icon {
  background: color-mix(in srgb, var(--warning) 14%, transparent);
  color: var(--warning);
}

[data-tone='danger'] .diag-icon {
  background: color-mix(in srgb, var(--destructive) 12%, transparent);
  color: var(--destructive);
}

.diag-copy {
  display: grid;
  gap: 0.2rem;
  min-width: 0;
}

.diag-copy span {
  color: var(--muted-foreground);
  font-size: 0.72rem;
}

.diag-copy strong {
  color: var(--foreground);
  font-size: 0.95rem;
  overflow-wrap: anywhere;
}

.summary-counts {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.45rem;
}

.summary-counts > div {
  min-width: 0;
  padding: 0.6rem 0.65rem;
  border-radius: 0.7rem;
  background: var(--muted);
}

.summary-counts span {
  display: block;
  color: var(--muted-foreground);
  font-size: 0.68rem;
}

.summary-counts strong {
  display: block;
  margin-top: 0.15rem;
  color: var(--foreground);
  font-size: 1.05rem;
}

.summary-counts .is-fail strong {
  color: var(--destructive);
}

.summary-counts .is-warn strong {
  color: var(--warning);
}

.summary-counts .is-pass strong {
  color: var(--success);
}

.read-only-note {
  display: flex;
  align-items: flex-start;
  gap: 0.4rem;
  color: var(--muted-foreground);
  font-size: 0.72rem;
  line-height: 1.5;
}

.read-only-note svg {
  flex: 0 0 auto;
  margin-top: 0.12rem;
}

.run-button {
  width: 100%;
}

/* ---------- report list ---------- */

.diagnostic-list {
  display: grid;
  gap: 0.25rem;
}

.diagnostic-item {
  min-width: 0;
  overflow: hidden;
  border: 0;
  border-radius: 0.6rem;
}

.diagnostic-trigger {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
  width: 100%;
  min-width: 0;
  padding: 0.75rem 0.85rem;
  border: 0;
  background: transparent;
  color: var(--foreground);
  text-align: left;
}

.diagnostic-trigger:hover {
  background: var(--muted);
}

.check-icon {
  width: 1.9rem;
  height: 1.9rem;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--muted);
  color: var(--muted-foreground);
}

[data-status='pass'] .check-icon {
  background: color-mix(in srgb, var(--success) 12%, transparent);
  color: var(--success);
}

[data-status='warn'] .check-icon {
  background: color-mix(in srgb, var(--warning) 14%, transparent);
  color: var(--warning);
}

[data-status='fail'] .check-icon {
  background: color-mix(in srgb, var(--destructive) 12%, transparent);
  color: var(--destructive);
}

.check-summary {
  display: grid;
  gap: 0.25rem;
  min-width: 0;
}

.check-title {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.check-title strong {
  font-size: 0.82rem;
  font-weight: 600;
}

.check-message {
  min-width: 0;
  color: var(--muted-foreground);
  font-size: 0.72rem;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.chevron {
  color: var(--muted-foreground);
  transition: transform 160ms ease;
}

.diagnostic-trigger[data-state='open'] .chevron {
  transform: rotate(180deg);
}

.diagnostic-details {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
  margin-top: 0.25rem;
  padding: 0.8rem 0.85rem 0.85rem 3.5rem;
  border-radius: 0.7rem;
  background: var(--muted);
}

/* reka hides collapsed content with [hidden]; keep our display rule from
   overriding it and leaking empty padded boxes. */
.diagnostic-details[hidden] {
  display: none;
}

.diagnostic-details > div {
  min-width: 0;
}

.diagnostic-details span {
  display: block;
  margin-bottom: 0.25rem;
  color: var(--muted-foreground);
  font-size: 0.66rem;
  font-weight: 600;
}

.diagnostic-details p {
  margin: 0;
  color: var(--foreground);
  font-size: 0.72rem;
  line-height: 1.6;
  overflow-wrap: anywhere;
}

/* ---------- logs ---------- */

.log-tabs-root {
  min-width: 0;
}

.log-toolbar {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  min-width: 0;
  margin-bottom: 0.6rem;
}

.log-tab-strip {
  min-width: 0;
  flex: 1 1 auto;
  overflow-x: auto;
  overscroll-behavior-inline: contain;
  scrollbar-width: thin;
}

.log-tabs {
  display: flex;
  width: max-content;
  min-width: max-content;
  padding: 0;
  gap: 2px;
  border-radius: 0.5rem;
  background: transparent;
}

.refresh-button {
  flex: 0 0 auto;
}

.log-tab {
  min-width: 0;
  border: 0 !important;
  border-radius: 0.5rem !important;
  background: transparent;
  color: var(--muted-foreground);
  font-size: 0.74rem;
}

.log-tab[data-state='active'] {
  background: var(--secondary);
  color: var(--foreground);
  font-weight: 600;
}

.tab-short {
  display: none;
}

.log-panel {
  min-width: 0;
}

.terminal-shell {
  min-width: 0;
  overflow: hidden;
  border-radius: 0.85rem;
  background: #1b1b1d;
}

.terminal-bar {
  display: flex;
  align-items: center;
  gap: 0.38rem;
  min-width: 0;
  height: 2.3rem;
  padding: 0 0.85rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  color: #98989d;
  font: 0.68rem/1 ui-monospace, SFMono-Regular, Consolas, monospace;
}

.terminal-bar span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.terminal-bar em {
  margin-left: auto;
  color: #30d158;
  font-style: normal;
  white-space: nowrap;
}

.log-viewer {
  display: block;
  width: 100%;
  height: 20rem;
  max-width: 100%;
  margin: 0;
  overflow: auto;
  padding: 0.85rem 0;
  background: transparent;
  color: #d1d1d6;
  font: 0.68rem/1.75 ui-monospace, SFMono-Regular, Consolas, monospace;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  scrollbar-color: #48484a #1b1b1d;
}

.log-viewer:focus-visible {
  outline: 3px solid rgba(10, 132, 255, 0.5);
  outline-offset: -3px;
}

.log-line {
  display: block;
  min-width: 100%;
  padding: 0 0.9rem;
}

.log-line:hover {
  background: rgba(255, 255, 255, 0.04);
}

.line-number {
  display: inline-block;
  width: 2.7rem;
  margin-right: 0.7rem;
  color: #636366;
  user-select: none;
}

.log-empty {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0 0.9rem;
  color: #98989d;
}

.spin {
  animation: maintenance-spin 0.9s linear infinite;
}

@keyframes maintenance-spin {
  to { transform: rotate(360deg); }
}

/* ---------- responsive ---------- */

@media (max-width: 1050px) {
  .grid-2 {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .card-head {
    gap: 0.55rem;
  }

  .card-head > [data-slot='badge'] {
    flex: 0 0 auto;
  }

  .state-badge {
    align-self: flex-start;
  }

  .policy-row.is-switch {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .policy-toggle {
    justify-self: end;
  }

  .foot-line {
    align-items: flex-start;
    flex-direction: column;
  }

  .foot-line strong {
    text-align: left;
  }

  .summary-counts {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .diagnostic-details {
    grid-template-columns: 1fr;
    padding-left: 3.1rem;
  }

  .log-toolbar {
    align-items: center;
  }

  .log-tabs {
    width: max-content;
  }

  .log-tab {
    padding-inline: 0.4rem;
  }

  .tab-full {
    display: none;
  }

  .tab-short {
    display: inline;
  }

  .refresh-button {
    width: 2.15rem;
    padding-inline: 0;
  }

  .refresh-button span {
    display: none;
  }

  .log-viewer {
    height: 17rem;
    font-size: 0.63rem;
  }

  .line-number {
    width: 2.2rem;
    margin-right: 0.45rem;
  }
}

@media (max-width: 380px) {
  .card-head,
  .card-body {
    padding-inline: 0.75rem;
  }

  .card-title small {
    font-size: 0.64rem;
  }

  .summary-counts > div {
    padding: 0.5rem;
  }

  .diagnostic-trigger {
    gap: 0.55rem;
    padding-inline: 0.55rem;
  }

  .diagnostic-details {
    padding-left: 0.7rem;
  }

  .line-number {
    width: 1.8rem;
    margin-right: 0.3rem;
  }
}

@media (prefers-reduced-motion: reduce) {
  .spin {
    animation: none;
  }

  .chevron {
    transition: none;
  }
}
</style>
