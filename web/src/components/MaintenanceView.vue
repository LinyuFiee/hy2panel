<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import {
  Activity,
  AlertCircle,
  CheckCircle2,
  ChevronDown,
  CircleAlert,
  ClipboardCheck,
  FileClock,
  Info,
  LoaderCircle,
  RefreshCw,
  RotateCcw,
  ScrollText,
  ShieldAlert,
  ShieldCheck
} from '@lucide/vue'
import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
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
const activeIssueCount = computed(() => Number(props.report?.fail || 0) + Number(props.report?.warn || 0))
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

function requestRecovery(event) {
  if (props.busy.recovery) return
  const value = Boolean(event.target.checked)
  recoveryValue.value = value
  pendingRecovery.value = value
  recoveryRequestObserved.value = false
  recoveryFeedback.value = ''
  emit('recovery', value)
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
  <div class="maintenance-view">
    <section class="maintenance-overview" aria-labelledby="maintenance-overview-title">
      <div class="overview-copy">
        <span class="overview-kicker"><ShieldCheck :size="14" /> MAINTENANCE CONTROL</span>
        <h2 id="maintenance-overview-title">恢复、诊断与日志，一处完成。</h2>
        <p>先确认本机健康，再决定是否变更恢复策略。日志仅从固定单元读取，并在返回前脱敏。</p>
      </div>
      <div class="overview-status" aria-label="维护摘要">
        <div><Activity :size="17" /><span>诊断</span><strong>{{ report ? (activeIssueCount ? `${activeIssueCount} 项待复核` : '全部通过') : '尚未运行' }}</strong></div>
        <div><RefreshCw :size="17" /><span>自动换端口</span><strong>{{ state.config?.recovery?.auto_port ? '已启用' : '未启用' }}</strong></div>
        <div><ScrollText :size="17" /><span>当前日志</span><strong>{{ logUnit }}</strong></div>
      </div>
    </section>

    <section class="maintenance-top-grid">
      <Card class="feature-card recovery-card">
        <CardHeader class="card-header recovery-header">
          <div class="heading-group">
            <div class="heading-icon is-primary"><RotateCcw :size="19" /></div>
            <div>
              <span class="feature-kicker">RECOVERY POLICY</span>
              <CardTitle>恢复策略</CardTitle>
              <CardDescription>先恢复进程，再以低频策略处理持续监听异常。</CardDescription>
            </div>
          </div>
          <Badge variant="outline" class="state-badge" :data-tone="recoveryStatus.tone" role="status" aria-live="polite">
            <component :is="recoveryStatus.icon" :size="13" :class="{ spin: recoveryStatus.tone === 'pending' }" />
            {{ recoveryStatus.text }}
          </Badge>
        </CardHeader>
        <CardContent class="card-content">
          <div class="policy-list">
            <div class="recovery-row policy-row">
              <div class="policy-icon is-success"><ShieldCheck :size="19" /></div>
              <div class="policy-copy">
                <div class="policy-title"><strong>进程崩溃自动重启</strong><Badge variant="success">始终启用</Badge></div>
                <p>systemd 立即拉起服务，看门狗每分钟复核实际监听。</p>
              </div>
            </div>
            <div class="recovery-row policy-row is-switch">
              <div class="policy-icon is-violet"><ShieldAlert :size="19" /></div>
              <div class="policy-copy">
                <strong>连续异常后自动换端口</strong>
                <p>连续 3 次监听失败才触发，24 小时内最多切换一次。</p>
              </div>
              <label class="policy-toggle" for="auto-port-recovery">
                <input
                  id="auto-port-recovery"
                  type="checkbox"
                  :checked="recoveryValue"
                  :disabled="busy.recovery"
                  aria-describedby="recovery-boundary"
                  @change="requestRecovery"
                />
                <span aria-hidden="true"></span>
                <span class="sr-label">连续异常后自动换端口</span>
              </label>
            </div>
          </div>
          <div id="recovery-boundary" class="boundary-note">
            <Info :size="16" />
            <p>本机只能可靠发现服务和监听异常。运营商或网络封堵仍需客户端或外部探测复核；换端口后必须重新导入客户端链接。</p>
          </div>
          <div class="policy-footer"><span>上次自动切换</span><strong>{{ lastPortChange }}</strong></div>
        </CardContent>
      </Card>

      <Card class="feature-card diagnostic-card" :data-tone="diagnosticTone">
        <CardHeader class="card-header diagnostic-header">
          <div>
            <span class="feature-kicker">GUIDED DIAGNOSTICS</span>
            <CardTitle>服务端问题自检</CardTitle>
            <CardDescription>按固定清单检查配置、服务、监听、证书、DNS、BBR 与资源。</CardDescription>
          </div>
          <Badge v-if="report" variant="outline" class="time-badge">{{ new Date(report.checked_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }}</Badge>
        </CardHeader>
        <CardContent class="card-content diagnostic-content">
          <div class="diagnostic-hero">
            <div class="diagnostic-icon"><component :is="diagnosticIcon" :size="24" /></div>
            <div><span>{{ report ? `${reportTotal} 项检查已完成` : '尚未运行自检' }}</span><strong>{{ diagnosticTitle }}</strong></div>
          </div>
          <div v-if="report" class="summary-counts" aria-label="诊断结果摘要">
            <div class="is-fail"><span>异常</span><strong>{{ report.fail || 0 }}</strong></div>
            <div class="is-warn"><span>提醒</span><strong>{{ report.warn || 0 }}</strong></div>
            <div class="is-pass"><span>正常</span><strong>{{ report.pass || 0 }}</strong></div>
            <div class="is-info"><span>说明</span><strong>{{ infoCount }}</strong></div>
          </div>
          <div v-else class="read-only-note"><Info :size="15" />只执行预设、只读检查，不会自动改动节点配置。</div>
          <Button class="run-button" :disabled="busy.diagnostics" @click="emit('diagnostics')">
            <LoaderCircle v-if="busy.diagnostics" :size="16" class="spin" /><Activity v-else :size="16" />
            {{ busy.diagnostics ? '正在逐项检查…' : report ? '重新运行自检' : '开始自检' }}
          </Button>
        </CardContent>
      </Card>
    </section>

    <Card v-if="report" class="feature-card report-card">
      <CardHeader class="card-header report-header">
        <div class="heading-group">
          <div class="heading-icon is-info"><ClipboardCheck :size="19" /></div>
          <div>
            <span class="feature-kicker">DIAGNOSTIC REPORT</span>
            <CardTitle>检查结果与修复建议</CardTitle>
            <CardDescription>{{ new Date(report.checked_at).toLocaleString() }} · 点击每项展开检查详情</CardDescription>
          </div>
        </div>
        <Badge variant="outline">{{ reportTotal }} 项</Badge>
      </CardHeader>
      <CardContent class="card-content report-content">
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
              <span class="check-icon"><component :is="statusIcons[check.status] || Info" :size="18" /></span>
              <span class="check-summary">
                <span class="check-title"><strong>{{ check.name }}</strong><Badge :variant="badgeVariant[check.status] || 'outline'">{{ statusText[check.status] || check.status }}</Badge></span>
                <span class="check-message">{{ check.message }}</span>
              </span>
              <ChevronDown :size="17" class="chevron" />
            </CollapsibleTrigger>
            <CollapsibleContent class="diagnostic-details">
              <div><span>检查结论</span><p>{{ check.message }}</p></div>
              <div><span>建议操作</span><p>{{ check.suggestion || '无需处理，保持当前配置并继续观察。' }}</p></div>
            </CollapsibleContent>
          </Collapsible>
        </div>
      </CardContent>
    </Card>

    <Card class="feature-card log-card">
      <CardHeader class="card-header log-header">
        <div class="heading-group">
          <div class="heading-icon is-violet"><ScrollText :size="19" /></div>
          <div>
            <span class="feature-kicker">REDACTED JOURNAL</span>
            <CardTitle>运行日志</CardTitle>
            <CardDescription>标签使用手动激活；凭据在服务端返回前完成脱敏。</CardDescription>
          </div>
        </div>
        <Badge variant="outline"><FileClock :size="13" />最多 300 行</Badge>
      </CardHeader>
      <CardContent class="card-content log-content">
        <Tabs :model-value="selectedLogUnit" activation-mode="manual" class="log-tabs-root" @update:model-value="requestLogs">
          <div class="log-toolbar">
            <TabsList class="log-tabs" aria-label="日志单元">
              <TabsTrigger v-for="unit in logUnits" :key="unit.value" :value="unit.value" :disabled="busy.logs" class="log-tab">
                <span class="tab-full">{{ unit.label }}</span><span class="tab-short">{{ unit.short }}</span>
              </TabsTrigger>
            </TabsList>
            <Button variant="outline" size="sm" class="refresh-button" :disabled="busy.logs" @click="refreshLogs">
              <RefreshCw :size="14" :class="{ spin: busy.logs }" /><span>{{ busy.logs ? '读取中' : '刷新' }}</span>
            </Button>
          </div>
          <TabsContent v-for="unit in logUnits" :key="unit.value" :value="unit.value" class="log-panel">
            <div class="terminal-shell">
              <div class="terminal-bar" aria-hidden="true">
                <i class="is-red"></i><i class="is-amber"></i><i class="is-green"></i>
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
        <p class="security-note"><ShieldCheck :size="14" />仅允许读取固定 systemd 单元；节点与 API 凭据不会显示。</p>
      </CardContent>
    </Card>
  </div>
</template>

<style scoped>
.maintenance-view {
  --m-card: var(--card, #fff);
  --m-text: var(--foreground, #263a33);
  --m-text-soft: color-mix(in srgb, var(--foreground, #263a33) 82%, transparent);
  --m-muted: var(--muted-foreground, #71827a);
  --m-border: var(--border, #e1e9e4);
  --m-border-soft: color-mix(in srgb, var(--border, #e1e9e4) 72%, transparent);
  --m-surface: color-mix(in srgb, var(--muted, #f1f5f2) 72%, var(--card, #fff));
  --m-primary: #4f8f79;
  --m-primary-strong: #39745f;
  --m-primary-soft: color-mix(in srgb, #69a58e 15%, var(--card, #fff));
  --m-success: #478266;
  --m-success-soft: color-mix(in srgb, #62a17e 14%, var(--card, #fff));
  --m-warning: #a0793c;
  --m-warning-soft: color-mix(in srgb, #d8b66f 15%, var(--card, #fff));
  --m-danger: #b85f62;
  --m-danger-soft: color-mix(in srgb, #cc7477 13%, var(--card, #fff));
  --m-info: #688da1;
  --m-info-soft: color-mix(in srgb, #77a7bd 14%, var(--card, #fff));
  --m-violet: #7e76aa;
  --m-violet-soft: color-mix(in srgb, #9389c1 14%, var(--card, #fff));
  display: grid;
  gap: 1rem;
  width: 100%;
  min-width: 0;
}

.maintenance-overview {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(21rem, 0.8fr);
  align-items: center;
  gap: 1.5rem;
  min-width: 0;
  padding: 1.35rem 1.5rem;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--m-primary) 16%, var(--m-border));
  border-radius: 1rem;
  background: radial-gradient(circle at 96% 10%, color-mix(in srgb, var(--m-violet) 10%, transparent), transparent 15rem), linear-gradient(135deg, var(--m-primary-soft), var(--m-card) 58%);
}

.maintenance-overview::after {
  content: '';
  position: absolute;
  right: -2.5rem;
  bottom: -3rem;
  width: 11rem;
  height: 11rem;
  border: 1px solid color-mix(in srgb, var(--m-primary) 12%, transparent);
  border-radius: 50%;
  pointer-events: none;
}

.overview-copy, .overview-status { position: relative; z-index: 1; min-width: 0; }
.overview-kicker, .feature-kicker { color: var(--m-primary); font-size: 0.63rem; font-weight: 760; letter-spacing: 0.13em; }
.overview-kicker { display: inline-flex; align-items: center; gap: 0.4rem; }
.feature-kicker { display: block; margin-bottom: 0.4rem; }
.overview-copy h2 { max-width: 34rem; margin: 0.55rem 0 0.5rem; color: var(--m-text); font-size: clamp(1.25rem, 2.2vw, 1.65rem); line-height: 1.25; }
.overview-copy p { max-width: 39rem; margin: 0; color: var(--m-muted); font-size: 0.74rem; line-height: 1.65; }
.overview-status { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 0.5rem; }
.overview-status > div { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 0.25rem 0.4rem; min-width: 0; padding: 0.72rem; border: 1px solid var(--m-border-soft); border-radius: 0.72rem; background: color-mix(in srgb, var(--m-card) 84%, transparent); }
.overview-status svg { grid-row: 1 / 3; align-self: center; color: var(--m-primary); }
.overview-status span { color: var(--m-muted); font-size: 0.58rem; }
.overview-status strong { min-width: 0; overflow: hidden; color: var(--m-text); font-size: 0.67rem; text-overflow: ellipsis; white-space: nowrap; }

.maintenance-top-grid { display: grid; grid-template-columns: minmax(0, 1.18fr) minmax(18rem, 0.82fr); align-items: stretch; gap: 1rem; min-width: 0; }
.maintenance-top-grid > * { min-width: 0; }
.feature-card { min-width: 0; gap: 0; padding-block: 0; overflow: hidden; border-color: var(--m-border); background: var(--m-card); box-shadow: 0 0.45rem 1.7rem color-mix(in srgb, var(--m-text) 5%, transparent); }
.card-header { display: flex; grid-template-columns: none; align-items: flex-start; justify-content: space-between; gap: 1rem; padding: 1.3rem 1.4rem 1rem; }
.card-content { padding: 0 1.4rem 1.3rem; }
.heading-group { display: flex; align-items: flex-start; gap: 0.78rem; min-width: 0; }
.heading-icon, .policy-icon, .diagnostic-icon, .check-icon { display: grid; place-items: center; flex: 0 0 auto; }
.heading-icon { width: 2.3rem; height: 2.3rem; border-radius: 0.72rem; }
.heading-icon.is-primary { background: var(--m-primary-soft); color: var(--m-primary); }
.heading-icon.is-info { background: var(--m-info-soft); color: var(--m-info); }
.heading-icon.is-violet { background: var(--m-violet-soft); color: var(--m-violet); }
:deep([data-slot='card-title']) { margin: 0; color: var(--m-text); font-size: 1rem; line-height: 1.3; }
:deep([data-slot='card-description']) { margin: 0.35rem 0 0; color: var(--m-muted); font-size: 0.7rem; line-height: 1.55; }
.card-header > :deep([data-slot='badge']) { flex: 0 0 auto; border-color: var(--m-border); color: var(--m-muted); font-size: 0.61rem; }

.state-badge[data-tone='success'] { border-color: color-mix(in srgb, var(--m-success) 28%, transparent); background: var(--m-success-soft); color: var(--m-success); }
.state-badge[data-tone='danger'] { border-color: color-mix(in srgb, var(--m-danger) 28%, transparent); background: var(--m-danger-soft); color: var(--m-danger); }
.state-badge[data-tone='pending'] { border-color: color-mix(in srgb, var(--m-warning) 28%, transparent); background: var(--m-warning-soft); color: var(--m-warning); }
.policy-list { display: grid; }
.policy-row { display: grid; grid-template-columns: auto minmax(0, 1fr); align-items: center; gap: 0.8rem; min-width: 0; padding: 1rem 0; border-bottom: 1px solid var(--m-border-soft); }
.policy-row.is-switch { grid-template-columns: auto minmax(0, 1fr) auto; }
.policy-icon { width: 2.25rem; height: 2.25rem; border-radius: 0.7rem; }
.policy-icon.is-success { background: var(--m-success-soft); color: var(--m-success); }
.policy-icon.is-violet { background: var(--m-violet-soft); color: var(--m-violet); }
.policy-copy { min-width: 0; }
.policy-title { display: flex; align-items: center; flex-wrap: wrap; gap: 0.5rem; }
.policy-copy strong { color: var(--m-text); font-size: 0.77rem; }
.policy-copy p { margin: 0.28rem 0 0; color: var(--m-muted); font-size: 0.67rem; line-height: 1.6; }
.policy-title :deep([data-slot='badge']) { font-size: 0.57rem; }
.policy-toggle { position: relative; display: block; width: 2.4rem; height: 1.4rem; flex: 0 0 auto; margin: 0; }
.policy-toggle input { position: absolute; width: 1px; height: 1px; margin: -1px; overflow: hidden; clip: rect(0 0 0 0); }
.policy-toggle > span:not(.sr-label) { position: absolute; inset: 0; border-radius: 2rem; background: color-mix(in srgb, var(--m-muted) 24%, var(--m-card)); transition: background 160ms ease; }
.policy-toggle > span:not(.sr-label)::after { content: ''; position: absolute; top: 0.2rem; left: 0.2rem; width: 1rem; height: 1rem; border-radius: 50%; background: white; box-shadow: 0 1px 4px #0002; transition: transform 160ms ease; }
.policy-toggle input:checked + span { background: var(--m-primary); }
.policy-toggle input:checked + span::after { transform: translateX(1rem); }
.policy-toggle input:focus-visible + span { outline: 3px solid color-mix(in srgb, var(--m-primary) 28%, transparent); outline-offset: 2px; }
.policy-toggle input:disabled + span { opacity: 0.58; cursor: wait; }
.sr-label { position: absolute; width: 1px; height: 1px; margin: -1px; overflow: hidden; clip: rect(0 0 0 0); }
.boundary-note { display: grid; grid-template-columns: auto minmax(0, 1fr); align-items: start; gap: 0.55rem; margin-top: 1rem; padding: 0.8rem; border: 1px solid color-mix(in srgb, var(--m-warning) 20%, transparent); border-radius: 0.75rem; background: var(--m-warning-soft); color: var(--m-warning); }
.boundary-note svg { margin-top: 0.1rem; }
.boundary-note p { margin: 0; color: var(--m-text-soft); font-size: 0.64rem; line-height: 1.6; }
.policy-footer { display: flex; align-items: center; justify-content: space-between; gap: 0.8rem; margin-top: 0.85rem; color: var(--m-muted); font-size: 0.62rem; }
.policy-footer strong { min-width: 0; color: var(--m-text-soft); text-align: right; overflow-wrap: anywhere; }

.diagnostic-card::before { content: ''; display: block; height: 0.2rem; background: var(--m-primary); }
.diagnostic-card[data-tone='success']::before { background: var(--m-success); }
.diagnostic-card[data-tone='warning']::before { background: var(--m-warning); }
.diagnostic-card[data-tone='danger']::before { background: var(--m-danger); }
.diagnostic-content { display: grid; gap: 0.8rem; }
.diagnostic-hero { display: flex; align-items: center; gap: 0.8rem; min-height: 4.4rem; padding: 0.85rem; border: 1px solid var(--m-border-soft); border-radius: 0.8rem; background: var(--m-surface); }
.diagnostic-icon { width: 2.8rem; height: 2.8rem; border-radius: 0.85rem; background: var(--m-primary-soft); color: var(--m-primary); }
[data-tone='success'] .diagnostic-icon { background: var(--m-success-soft); color: var(--m-success); }
[data-tone='warning'] .diagnostic-icon { background: var(--m-warning-soft); color: var(--m-warning); }
[data-tone='danger'] .diagnostic-icon { background: var(--m-danger-soft); color: var(--m-danger); }
.diagnostic-hero > div:last-child { display: grid; gap: 0.2rem; min-width: 0; }
.diagnostic-hero span { color: var(--m-muted); font-size: 0.64rem; }
.diagnostic-hero strong { color: var(--m-text); font-size: 0.88rem; overflow-wrap: anywhere; }
.summary-counts { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 0.42rem; }
.summary-counts > div { min-width: 0; padding: 0.62rem; border: 1px solid var(--m-border-soft); border-radius: 0.65rem; background: var(--m-card); }
.summary-counts span { display: block; color: var(--m-muted); font-size: 0.57rem; }
.summary-counts strong { display: block; margin-top: 0.15rem; color: var(--m-text); font-size: 0.95rem; }
.summary-counts .is-fail strong { color: var(--m-danger); }
.summary-counts .is-warn strong { color: var(--m-warning); }
.summary-counts .is-pass strong { color: var(--m-success); }
.summary-counts .is-info strong { color: var(--m-info); }
.read-only-note { display: flex; align-items: flex-start; gap: 0.4rem; color: var(--m-muted); font-size: 0.64rem; line-height: 1.5; }
.run-button { width: 100%; min-height: 2.5rem; border-radius: 0.7rem; background: var(--m-primary); color: white; }
.run-button:hover { background: var(--m-primary-strong); }

.report-content { padding-top: 0; }
.diagnostic-list { display: grid; gap: 0.55rem; }
.diagnostic-item { min-width: 0; overflow: hidden; border: 1px solid var(--m-border-soft); border-radius: 0.75rem; background: var(--m-card); }
.diagnostic-trigger { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 0.75rem; width: 100%; min-width: 0; padding: 0.85rem 0.9rem; border: 0; background: transparent; color: var(--m-text); text-align: left; }
.diagnostic-trigger:hover { background: var(--m-surface); }
.diagnostic-trigger:focus-visible { outline: 3px solid color-mix(in srgb, var(--m-primary) 24%, transparent); outline-offset: -3px; }
.check-icon { width: 2rem; height: 2rem; border-radius: 50%; background: var(--m-info-soft); color: var(--m-info); }
[data-status='pass'] .check-icon { background: var(--m-success-soft); color: var(--m-success); }
[data-status='warn'] .check-icon { background: var(--m-warning-soft); color: var(--m-warning); }
[data-status='fail'] .check-icon { background: var(--m-danger-soft); color: var(--m-danger); }
.check-summary { display: grid; gap: 0.25rem; min-width: 0; }
.check-title { display: flex; align-items: center; flex-wrap: wrap; gap: 0.5rem; }
.check-title strong { color: var(--m-text); font-size: 0.75rem; }
.check-title :deep([data-slot='badge']) { font-size: 0.56rem; }
.check-message { min-width: 0; color: var(--m-muted); font-size: 0.67rem; line-height: 1.5; overflow-wrap: anywhere; }
.chevron { color: var(--m-muted); transition: transform 160ms ease; }
.diagnostic-trigger[data-state='open'] .chevron { transform: rotate(180deg); }
.diagnostic-details { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.75rem; padding: 0.85rem 0.9rem 0.9rem 3.65rem; border-top: 1px solid var(--m-border-soft); background: var(--m-surface); }
.diagnostic-details > div { min-width: 0; }
.diagnostic-details span { display: block; margin-bottom: 0.25rem; color: var(--m-muted); font-size: 0.56rem; font-weight: 700; letter-spacing: 0.06em; }
.diagnostic-details p { margin: 0; color: var(--m-text-soft); font-size: 0.66rem; line-height: 1.6; overflow-wrap: anywhere; }

.log-content { min-width: 0; }
.log-tabs-root { min-width: 0; }
.log-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 0.75rem; min-width: 0; margin-bottom: 0.65rem; }
.log-tabs { width: fit-content; max-width: 100%; min-width: 0; background: var(--m-surface); }
.log-tab { min-width: 0; color: var(--m-muted); font-size: 0.66rem; }
.log-tab[data-state='active'] { border-color: var(--m-border); background: var(--m-card); color: var(--m-text); }
.tab-short { display: none; }
.refresh-button { flex: 0 0 auto; border-color: var(--m-border); color: var(--m-text-soft); }
.log-panel { min-width: 0; }
.terminal-shell { min-width: 0; overflow: hidden; border: 1px solid #ffffff18; border-radius: 0.85rem; background: #17201d; }
.terminal-bar { display: flex; align-items: center; gap: 0.38rem; min-width: 0; height: 2.35rem; padding: 0 0.8rem; border-bottom: 1px solid #ffffff12; background: #202b27; color: #8fa29a; font: 0.61rem/1 ui-monospace, SFMono-Regular, Consolas, monospace; }
.terminal-bar i { width: 0.48rem; height: 0.48rem; flex: 0 0 auto; border-radius: 50%; }
.terminal-bar i.is-red { background: #ce7474; }
.terminal-bar i.is-amber { background: #d0a95f; }
.terminal-bar i.is-green { background: #6fac91; }
.terminal-bar span { min-width: 0; margin-left: 0.3rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.terminal-bar em { margin-left: auto; color: #8fc8ae; font-style: normal; white-space: nowrap; }
.log-viewer { display: block; width: 100%; height: 21rem; max-width: 100%; margin: 0; overflow: auto; padding: 0.85rem 0; background: transparent; color: #c8d9d1; font: 0.68rem/1.78 ui-monospace, SFMono-Regular, Consolas, monospace; white-space: pre-wrap; overflow-wrap: anywhere; scrollbar-color: #4b6258 #17201d; }
.log-viewer:focus-visible { outline: 3px solid #8fc8ae78; outline-offset: -3px; }
.log-line { display: block; min-width: 100%; padding: 0 0.9rem; }
.log-line:hover { background: #ffffff08; }
.line-number { display: inline-block; width: 2.7rem; margin-right: 0.7rem; color: #61756c; user-select: none; }
.log-empty { display: inline-flex; align-items: center; gap: 0.5rem; padding: 0 0.9rem; color: #7f938a; }
.security-note { display: flex; align-items: flex-start; gap: 0.42rem; margin: 0.75rem 0 0; color: var(--m-muted); font-size: 0.62rem; line-height: 1.5; }
.security-note svg { flex: 0 0 auto; margin-top: 0.05rem; color: var(--m-success); }
.spin { animation: maintenance-spin 0.9s linear infinite; }
@keyframes maintenance-spin { to { transform: rotate(360deg); } }

@media (max-width: 1050px) {
  .maintenance-overview, .maintenance-top-grid { grid-template-columns: 1fr; }
}

@media (max-width: 640px) {
  .maintenance-view { gap: 0.8rem; }
  .maintenance-overview { gap: 1rem; padding: 1.15rem 1rem; }
  .overview-status { grid-template-columns: 1fr; }
  .overview-status > div { grid-template-columns: auto minmax(0, 1fr) auto; }
  .overview-status svg { grid-row: auto; }
  .overview-status strong { text-align: right; }
  .card-header { padding: 1.05rem 1rem 0.85rem; }
  .card-content { padding: 0 1rem 1rem; }
  .recovery-header, .diagnostic-header { flex-direction: column; }
  .state-badge { margin-left: 3.05rem; }
  .policy-row.is-switch { grid-template-columns: auto minmax(0, 1fr); }
  .policy-toggle { grid-column: 2; justify-self: start; }
  .policy-footer { align-items: flex-start; flex-direction: column; }
  .policy-footer strong { text-align: left; }
  .summary-counts { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .report-header > :deep([data-slot='badge']), .log-header > :deep([data-slot='badge']) { display: none; }
  .diagnostic-trigger { gap: 0.6rem; padding-inline: 0.7rem; }
  .diagnostic-details { grid-template-columns: 1fr; padding-left: 3.25rem; }
  .log-toolbar { align-items: stretch; }
  .log-tabs { width: 100%; }
  .log-tab { padding-inline: 0.4rem; }
  .tab-full { display: none; }
  .tab-short { display: inline; }
  .refresh-button { width: 2.15rem; padding-inline: 0; }
  .refresh-button span { display: none; }
  .log-viewer { height: 18rem; font-size: 0.63rem; }
  .line-number { width: 2.2rem; margin-right: 0.45rem; }
}

@media (max-width: 420px) {
  .check-icon { width: 1.8rem; height: 1.8rem; }
  .diagnostic-details { padding-left: 0.75rem; }
}

@media (prefers-reduced-motion: reduce) {
  .spin { animation: none; }
  .policy-toggle > span, .policy-toggle > span::after, .chevron { transition: none; }
}
</style>
