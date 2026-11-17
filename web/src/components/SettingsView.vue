<script setup>
import { computed, ref, watch } from 'vue'
import {
  Check,
  Circle,
  CircleAlert,
  CloudCog,
  Database,
  Download,
  ExternalLink,
  Gauge,
  KeyRound,
  LoaderCircle,
  LockKeyhole,
  Palette,
  RefreshCw,
  RotateCcw,
  ServerCog,
  ShieldCheck,
  SlidersHorizontal,
  Trash2,
  XCircle,
  Zap
} from '@lucide/vue'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Field,
  FieldDescription,
  FieldLabel,
  Input,
  NativeSelect,
  NativeSelectOption,
  Progress
} from '@/components/ui'
import ThemeToggle from './ThemeToggle.vue'
import { formatBytes } from '../utils'

const props = defineProps({
  state: { type: Object, required: true },
  busy: { type: Object, required: true },
  singBox: { type: Object, default: null },
  singBoxLoading: { type: Boolean, default: false }
})

const emit = defineEmits([
  'save-racknerd',
  'clear-racknerd',
  'apply-bbr',
  'check-update',
  'apply-update',
  'switch-sing-box',
  'rollback-sing-box'
])

const apiKey = ref('')
const apiHash = ref('')
const selectedVersion = ref('')
const versions = computed(() => props.singBox?.versions || [])
const operation = computed(() => props.singBox?.operation || {})
const operationStatus = computed(() => operation.value.status || 'idle')
const operationRunning = computed(() => operationStatus.value === 'running')
const selectedRelease = computed(() => versions.value.find(entry => entry.version === selectedVersion.value) || null)
const coreControlsDisabled = computed(() => props.singBoxLoading || props.busy.singBox || operationRunning.value)

const switchDisabledReason = computed(() => {
  if (props.singBoxLoading) return '正在读取兼容版本，请稍候。'
  if (props.busy.singBox) return '请求正在提交，请勿重复操作。'
  if (operationRunning.value) return '已有核心版本任务运行中。'
  if (!versions.value.length) return '兼容版本清单暂不可用。'
  if (!selectedVersion.value) return '请先选择目标版本。'
  if (selectedVersion.value === props.singBox?.current_version) return '所选版本已经在运行。'
  return ''
})
const rollbackDisabledReason = computed(() => {
  if (props.singBoxLoading) return '正在读取核心状态，请稍候。'
  if (props.busy.singBox) return '请求正在提交，请勿重复操作。'
  if (operationRunning.value) return '已有核心版本任务运行中。'
  if (!props.singBox?.can_rollback || !props.singBox?.previous_version) return '尚无可恢复的上一版本。'
  return ''
})
const bbrDisabledReason = computed(() => {
  if (props.busy.bbr) return ''
  if (operationRunning.value || props.busy.singBox) return 'sing-box 核心版本任务正在运行。'
  if (props.busy.updating || props.state.update?.updating) return '面板更新正在运行。'
  return ''
})
const updateDisabledReason = computed(() => {
  if (operationRunning.value || props.busy.singBox) return 'sing-box 核心版本任务正在运行。'
  return ''
})
const maintenanceLocked = computed(() => Boolean(
  operationRunning.value ||
  props.busy.singBox ||
  props.busy.updating ||
  props.state.update?.updating
))

const coreSteps = [
  { id: 'requested', label: '提交任务' },
  { id: 'downloading', label: '下载校验' },
  { id: 'preflight', label: '兼容检查' },
  { id: 'switching', label: '切换重启' },
  { id: 'committed', label: '健康确认' }
]
const phaseAliases = {
  verifying: 'downloading',
  installing: 'downloading',
  restarting: 'switching',
  restarting_core: 'switching',
  healthcheck: 'committed',
  health_check: 'committed',
  completed: 'committed',
  rollback: 'rolling_back'
}
const normalizedPhase = computed(() => phaseAliases[operation.value.phase] || operation.value.phase || 'requested')
const operationSucceeded = computed(() => ['succeeded', 'completed', 'success'].includes(operationStatus.value))
const rollbackSucceeded = computed(() => operationStatus.value === 'failed_rolled_back' || normalizedPhase.value === 'rolled_back' || operation.value.rollback?.succeeded === true)
const rollbackFailed = computed(() => operationStatus.value === 'rollback_failed' || normalizedPhase.value === 'rollback_failed')
const operationFailed = computed(() => !operationRunning.value && !operationSucceeded.value && operationStatus.value !== 'idle' && !rollbackSucceeded.value)
const hasOperation = computed(() => Boolean(operation.value.id || operation.value.status || operation.value.phase || operation.value.message))
const timelineSteps = computed(() => {
  const steps = coreSteps.map(step => ({ ...step }))
  if (rollbackSucceeded.value || rollbackFailed.value || ['rolling_back', 'rolled_back', 'rollback_failed'].includes(normalizedPhase.value)) {
    steps.push({ id: 'rollback', label: '自动回滚' })
  }
  return steps
})
const operationActiveIndex = computed(() => {
  if (['rolling_back', 'rolled_back', 'rollback_failed'].includes(normalizedPhase.value)) return timelineSteps.value.length - 1
  const index = coreSteps.findIndex(step => step.id === normalizedPhase.value)
  return Math.max(0, index)
})
const operationProgress = computed(() => {
  if (operationSucceeded.value || operationFailed.value || rollbackSucceeded.value || rollbackFailed.value) return 100
  return ((operationActiveIndex.value + 0.55) / Math.max(1, timelineSteps.value.length)) * 100
})
const operationVisual = computed(() => {
  if (rollbackFailed.value) return { tone: 'danger', label: '回滚失败', icon: XCircle }
  if (rollbackSucceeded.value) return { tone: 'rollback', label: '失败后已恢复', icon: RotateCcw }
  if (operationFailed.value) return { tone: 'danger', label: '任务失败', icon: XCircle }
  if (operationSucceeded.value) return { tone: 'success', label: '任务完成', icon: ShieldCheck }
  if (operationRunning.value) return { tone: 'running', label: '任务运行中', icon: LoaderCircle }
  return { tone: 'idle', label: '暂无任务', icon: Circle }
})

const updateChecking = computed(() => Boolean(props.busy.updateCheck || props.state.update?.checking))
const updateRunning = computed(() => Boolean(props.busy.updating || props.state.update?.updating))
const updateStatusText = computed(() => {
  if (updateRunning.value) return '正在安装面板更新'
  if (updateChecking.value) return '正在检查 Release'
  if (props.state.update?.error) return '最近操作失败'
  if (props.state.update?.available) return `v${props.state.update.latest_version} 可用`
  if (props.state.update?.latest_version) return '当前已是最新版本'
  return '尚未检查更新'
})

watch(() => [props.singBox?.recommended_version, props.singBox?.current_version, versions.value.length], () => {
  if (!versions.value.some(entry => entry.version === selectedVersion.value)) {
    selectedVersion.value = props.singBox?.recommended_version || props.singBox?.current_version || versions.value[0]?.version || ''
  }
}, { immediate: true })

function timelineStepState(index, step) {
  if (!hasOperation.value) return 'idle'
  if (step.id === 'rollback') {
    if (rollbackFailed.value) return 'failed'
    if (rollbackSucceeded.value) return 'rollback-success'
    return 'active'
  }
  if (rollbackSucceeded.value || rollbackFailed.value) return 'done'
  if ((operationFailed.value || operationRunning.value) && index === operationActiveIndex.value) return operationFailed.value ? 'failed' : 'active'
  if (operationSucceeded.value || index < operationActiveIndex.value) return 'done'
  return 'pending'
}

function timelineStepIcon(value) {
  if (value === 'done' || value === 'rollback-success') return Check
  if (value === 'failed') return XCircle
  if (value === 'active') return LoaderCircle
  return Circle
}

function saveRackNerd() {
  emit('save-racknerd', { apiKey: apiKey.value, apiHash: apiHash.value })
}

function clearCredentials() {
  apiKey.value = ''
  apiHash.value = ''
}

defineExpose({ clearCredentials })
</script>

<template>
  <div class="settings-view">
    <section class="settings-overview" aria-labelledby="settings-overview-title">
      <div class="overview-copy">
        <span class="overview-kicker"><SlidersHorizontal :size="14" /> SYSTEM CONFIGURATION</span>
        <h2 id="settings-overview-title">版本、网络与访问边界。</h2>
        <p>高影响操作会先展示影响范围，再由服务器执行固定、可审计的维护流程。</p>
      </div>
      <div class="overview-map" aria-label="设置分区">
        <span><ServerCog :size="15" />核心</span>
        <span><KeyRound :size="15" />凭据</span>
        <span><Zap :size="15" />网络</span>
        <span><Download :size="15" />更新</span>
        <span><LockKeyhole :size="15" />安全与主题</span>
      </div>
    </section>

    <Card class="settings-card singbox-card core-card" data-testid="sing-box-version-card">
      <div class="core-accent"></div>
      <CardHeader class="card-header">
        <div class="heading-group">
          <div class="heading-icon is-info"><ServerCog :size="21" /></div>
          <div>
            <span class="settings-kicker">CORE RUNTIME</span>
            <CardTitle>sing-box 核心</CardTitle>
            <CardDescription>仅切换到兼容清单内、具有固定校验值的官方版本。</CardDescription>
          </div>
        </div>
        <Badge :variant="operationRunning ? 'warning' : 'success'"><i></i>{{ operationRunning ? '维护中' : '运行可用' }}</Badge>
      </CardHeader>

      <CardContent class="card-content core-content">
        <template v-if="singBox">
          <div class="version-overview">
            <div class="version-primary">
              <span>当前运行版本</span><strong>v{{ singBox.current_version || '未知' }}</strong><small><ShieldCheck :size="13" />受兼容清单保护</small>
            </div>
            <div><span>上一版本</span><strong>{{ singBox.previous_version ? `v${singBox.previous_version}` : '暂无' }}</strong><small>{{ singBox.can_rollback ? '可用于手动回滚' : '没有回滚快照' }}</small></div>
            <div class="is-recommended"><span>推荐版本</span><strong>v{{ singBox.recommended_version || '未知' }}</strong><small>维护清单建议</small></div>
          </div>

          <div class="version-workbench">
            <div class="version-field">
              <label for="sing-box-version">目标版本</label>
              <NativeSelect
                id="sing-box-version"
                v-model="selectedVersion"
                class="version-select"
                :disabled="coreControlsDisabled || !versions.length"
                aria-describedby="version-help"
              >
                <NativeSelectOption v-for="entry in versions" :key="entry.version" :value="entry.version">
                  v{{ entry.version }}{{ entry.status === 'recommended' ? ' · 推荐' : '' }}{{ entry.version === singBox.current_version ? ' · 当前' : '' }}
                </NativeSelectOption>
              </NativeSelect>
              <p id="version-help" class="control-reason">{{ switchDisabledReason || '切换会重启 sing-box；健康检查失败时服务器自动恢复上一版本。' }}</p>
            </div>
            <div class="release-meta">
              <div><span>官方资产</span><strong>{{ selectedRelease?.asset_name || '正在读取兼容版本…' }}</strong><small v-if="selectedRelease">{{ formatBytes(selectedRelease.size) }} · SHA-256 固定校验</small></div>
              <a v-if="selectedRelease?.release_url" :href="selectedRelease.release_url" target="_blank" rel="noopener noreferrer">Release <ExternalLink :size="13" /></a>
            </div>
          </div>

          <div class="core-actions">
            <AlertDialog>
              <AlertDialogTrigger as-child>
                <Button class="primary-action" :disabled="Boolean(switchDisabledReason)">
                  <RefreshCw :size="16" :class="{ spin: operationRunning && operation.action === 'switch' }" />
                  {{ operationRunning && operation.action === 'switch' ? '正在切换…' : '切换核心' }}
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>切换到 sing-box v{{ selectedVersion }}？</AlertDialogTitle>
                  <AlertDialogDescription>代理连接会短暂中断。服务器将下载并校验官方资产，执行兼容检查、重启与健康确认；失败时自动尝试恢复当前版本。</AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>取消</AlertDialogCancel>
                  <AlertDialogAction @click="emit('switch-sing-box', selectedVersion)">确认切换</AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>

            <AlertDialog>
              <AlertDialogTrigger as-child>
                <Button variant="outline" :disabled="Boolean(rollbackDisabledReason)">
                  <RotateCcw :size="16" :class="{ spin: operationRunning && operation.action === 'rollback' }" />
                  {{ operationRunning && operation.action === 'rollback' ? '正在回滚…' : '回滚上一版本' }}
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>回滚到 v{{ singBox.previous_version }}？</AlertDialogTitle>
                  <AlertDialogDescription>当前连接会短暂中断。回滚仍会经过服务重启与健康检查，完成后当前版本与上一版本记录会交换。</AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>取消</AlertDialogCancel>
                  <AlertDialogAction class="destructive-dialog-action" @click="emit('rollback-sing-box')">确认回滚</AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
          <p v-if="rollbackDisabledReason" class="disabled-reason">回滚不可用：{{ rollbackDisabledReason }}</p>

          <div v-if="hasOperation" class="operation-timeline" :data-tone="operationVisual.tone" role="status" aria-live="polite">
            <div class="timeline-header">
              <div class="timeline-state">
                <component :is="operationVisual.icon" :size="16" :class="{ spin: operationRunning }" />
                <div><span>核心版本任务</span><strong>{{ operationVisual.label }}</strong></div>
              </div>
              <Badge variant="outline">{{ operation.action === 'rollback' ? '手动回滚' : operation.target_version ? `目标 v${operation.target_version}` : '版本维护' }}</Badge>
            </div>
            <div class="timeline-track" aria-label="任务进度">
              <Progress :model-value="operationProgress" class="timeline-progress" />
              <ol :class="{ 'has-rollback': timelineSteps.length > 5 }">
                <li v-for="(step, index) in timelineSteps" :key="step.id" :data-state="timelineStepState(index, step)">
                  <span><component :is="timelineStepIcon(timelineStepState(index, step))" :size="12" :class="{ spin: timelineStepState(index, step) === 'active' }" /></span>
                  <em>{{ step.label }}</em>
                </li>
              </ol>
            </div>
            <div class="timeline-message">
              <CircleAlert v-if="operationVisual.tone === 'danger'" :size="15" />
              <RotateCcw v-else-if="operationVisual.tone === 'rollback'" :size="15" />
              <ShieldCheck v-else :size="15" />
              <p>{{ operation.message || '核心维护任务正在处理。' }}</p>
            </div>
          </div>
        </template>

        <div v-else class="core-empty" :aria-busy="singBoxLoading">
          <RefreshCw :size="22" :class="{ spin: singBoxLoading }" />
          <strong>{{ singBoxLoading ? '正在读取核心版本' : '核心版本信息暂不可用' }}</strong>
          <span>{{ singBoxLoading ? '正在同步兼容清单与最近任务。' : '稍后重新进入此页面以重试。' }}</span>
        </div>
      </CardContent>
    </Card>

    <div class="settings-pair-grid">
      <Card class="settings-card credential-card">
        <CardHeader class="card-header compact-header">
          <div class="heading-group">
            <div class="heading-icon is-primary"><CloudCog :size="20" /></div>
            <div><span class="settings-kicker">BANDWIDTH SOURCE</span><CardTitle>RackNerd 官方流量</CardTitle><CardDescription>凭据仅用于只读 info 请求，不提供关机或重启能力。</CardDescription></div>
          </div>
          <Badge v-if="state.racknerd?.configured" variant="success"><ShieldCheck :size="13" />已配置</Badge>
          <Badge v-else variant="outline">未配置</Badge>
        </CardHeader>
        <CardContent class="card-content section-content">
          <form class="credential-form" @submit.prevent="saveRackNerd">
            <div class="credential-fields">
              <Field>
                <FieldLabel for="racknerd-api-key">API Key</FieldLabel>
                <Input id="racknerd-api-key" v-model="apiKey" type="password" autocomplete="off" :disabled="busy.racknerd" :placeholder="state.racknerd?.configured ? '已保存；输入新 Key 可替换' : '输入该 VPS 的 API Key'" />
              </Field>
              <Field>
                <FieldLabel for="racknerd-api-hash">API Hash</FieldLabel>
                <Input id="racknerd-api-hash" v-model="apiHash" type="password" autocomplete="off" :disabled="busy.racknerd" :placeholder="state.racknerd?.configured ? '已保存；输入新 Hash 可替换' : '输入 API Hash'" />
              </Field>
            </div>
            <FieldDescription class="field-help">凭据仅保存在 VPS 的 600 权限配置中，状态 API 不会回显；已配置时留空字段会保留原值。</FieldDescription>
            <div class="action-row">
              <Button type="submit" class="primary-action" :disabled="busy.racknerd"><KeyRound :size="16" />{{ busy.racknerd ? '正在验证…' : '保存并获取流量' }}</Button>
              <AlertDialog v-if="state.racknerd?.configured">
                <AlertDialogTrigger as-child><Button type="button" variant="outline" class="danger-outline" :disabled="busy.racknerd"><Trash2 :size="16" />清除凭据</Button></AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader><AlertDialogTitle>清除 RackNerd 凭据？</AlertDialogTitle><AlertDialogDescription>清除后面板将停止拉取官方套餐流量；节点和代理服务不会受到影响。需要再次显示套餐流量时必须重新输入 Key 与 Hash。</AlertDialogDescription></AlertDialogHeader>
                  <AlertDialogFooter><AlertDialogCancel>取消</AlertDialogCancel><AlertDialogAction class="destructive-dialog-action" @click="emit('clear-racknerd')">确认清除</AlertDialogAction></AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </div>
          </form>
          <p v-if="state.racknerd?.error" class="error-message" role="status">{{ state.racknerd.error }}</p>
        </CardContent>
      </Card>

      <Card class="settings-card bbr-card">
        <CardHeader class="card-header compact-header">
          <div class="heading-group">
            <div class="heading-icon is-violet"><Zap :size="20" /></div>
            <div><span class="settings-kicker">NETWORK TUNING</span><CardTitle>BBR 网络优化</CardTitle><CardDescription>应用项目内置的 BBR + fq 保守模板，不接受浏览器任意命令。</CardDescription></div>
          </div>
          <Badge :variant="state.status.bbr_enabled ? 'success' : 'warning'">{{ state.status.bbr_enabled ? '已启用' : '待处理' }}</Badge>
        </CardHeader>
        <CardContent class="card-content section-content">
          <div class="readout-grid"><div><span>拥塞控制</span><strong>{{ state.status.congestion_control || '未知' }}</strong></div><div><span>队列规则</span><strong>{{ state.status.default_qdisc || '未知' }}</strong></div></div>
          <div class="network-note"><Gauge :size="16" /><p>BBR 主要优化 TCP；Hysteria2 使用 QUIC 自有拥塞控制，不会被 BBR 直接加速。</p></div>
          <AlertDialog>
            <AlertDialogTrigger as-child><Button variant="outline" class="fit-action" :disabled="busy.bbr || Boolean(bbrDisabledReason)"><RotateCcw :size="16" :class="{ spin: busy.bbr }" />{{ busy.bbr ? '正在应用…' : '应用 BBR + fq' }}</Button></AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader><AlertDialogTitle>应用 BBR + fq 模板？</AlertDialogTitle><AlertDialogDescription>系统将写入项目固定的内核网络参数并立即加载。此操作不会执行浏览器提供的命令，也不会修改节点协议配置。</AlertDialogDescription></AlertDialogHeader>
              <AlertDialogFooter><AlertDialogCancel>取消</AlertDialogCancel><AlertDialogAction @click="emit('apply-bbr')">确认应用</AlertDialogAction></AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
          <p v-if="bbrDisabledReason" class="disabled-reason">当前不可用：{{ bbrDisabledReason }}</p>
        </CardContent>
      </Card>
    </div>

    <Card class="settings-card update-card">
      <CardHeader class="card-header compact-header">
        <div class="heading-group">
          <div class="heading-icon is-success"><Download :size="20" /></div>
          <div><span class="settings-kicker">PANEL RELEASE</span><CardTitle>面板更新</CardTitle><CardDescription>从 hy2panel 官方 Release 检测，并在安装前强制校验 SHA-256。</CardDescription></div>
        </div>
        <Badge v-if="state.update?.available" variant="success">发现新版本</Badge><Badge v-else variant="outline">{{ updateChecking ? '检测中' : '稳定' }}</Badge>
      </CardHeader>
      <CardContent class="card-content section-content">
        <div class="update-summary">
          <div><span>当前版本</span><strong>v{{ state.update?.current_version || '未知' }}</strong></div>
          <div><span>最新版本</span><strong>{{ state.update?.latest_version ? `v${state.update.latest_version}` : '尚未检测' }}</strong></div>
          <div class="operation-readout"><span>操作状态</span><strong>{{ updateStatusText }}</strong></div>
        </div>
        <div class="action-row">
          <Button variant="outline" :disabled="updateChecking || updateRunning" @click="emit('check-update')"><RefreshCw :size="16" :class="{ spin: updateChecking }" />{{ updateChecking ? '正在检测…' : '检测更新' }}</Button>
          <AlertDialog v-if="state.update?.available">
            <AlertDialogTrigger as-child><Button class="primary-action" :disabled="updateRunning || Boolean(updateDisabledReason)"><Download :size="16" />{{ updateRunning ? '正在更新…' : '立即更新' }}</Button></AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader><AlertDialogTitle>更新面板到 v{{ state.update.latest_version }}？</AlertDialogTitle><AlertDialogDescription>面板会短暂重启，但节点配置会保留。服务器将在安装前校验升级包，失败时自动尝试回滚到当前版本。</AlertDialogDescription></AlertDialogHeader>
              <AlertDialogFooter><AlertDialogCancel>取消</AlertDialogCancel><AlertDialogAction @click="emit('apply-update')">确认更新</AlertDialogAction></AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
          <Button as="a" variant="outline" href="https://github.com/LinyuFiee/hy2panel" target="_blank" rel="noopener noreferrer">GitHub <ExternalLink :size="14" /></Button>
          <Button v-if="state.update?.release_url" as="a" variant="ghost" :href="state.update.release_url" target="_blank" rel="noopener noreferrer">Release <ExternalLink :size="14" /></Button>
        </div>
        <p v-if="updateDisabledReason && state.update?.available" class="disabled-reason">更新不可用：{{ updateDisabledReason }}</p>
        <p v-if="state.update?.message" class="success-message" role="status"><ShieldCheck :size="14" />{{ state.update.message }}</p>
        <p v-if="state.update?.error" class="error-message" role="status">{{ state.update.error }}</p>
        <p class="field-help">更新不会覆盖节点配置；切换期间控制面板可能短暂无法访问。</p>
      </CardContent>
    </Card>

    <Card class="settings-card security-card">
      <CardHeader class="card-header compact-header">
        <div class="heading-group">
          <div class="heading-icon is-warning"><LockKeyhole :size="20" /></div>
          <div><span class="settings-kicker">SECURITY & APPEARANCE</span><CardTitle>安全与主题</CardTitle><CardDescription>确认当前访问边界，并选择仅保存在本机浏览器的外观偏好。</CardDescription></div>
        </div>
      </CardHeader>
      <CardContent class="card-content section-content">
        <div class="safety-layout">
          <section class="security-section" aria-labelledby="security-title">
            <div class="subheading"><ShieldCheck :size="17" /><div><strong id="security-title">访问安全</strong><span>服务器强制执行</span></div></div>
            <ul class="security-list">
              <li><ShieldCheck :size="15" /><span>公网入口<strong>仅通过 HTTPS 访问</strong></span></li>
              <li><ShieldCheck :size="15" /><span>登录防护<strong>按来源限制登录尝试速率</strong></span></li>
              <li><ShieldCheck :size="15" /><span>安全会话<strong>HttpOnly · SameSite Strict</strong></span></li>
              <li><Database :size="15" /><span>敏感配置<strong>仅 root / hy2panel 可读取</strong></span></li>
            </ul>
          </section>
          <section class="theme-section" aria-labelledby="theme-title">
            <div class="subheading"><Palette :size="17" /><div><strong id="theme-title">外观主题</strong><span>仅影响当前浏览器</span></div></div>
            <div class="theme-panel"><ThemeToggle label="界面颜色" /><p>跟随系统会根据设备偏好自动选择浅色或深色，不会同步到服务器。</p></div>
          </section>
        </div>
      </CardContent>
    </Card>

    <p v-if="maintenanceLocked" class="maintenance-lock-note" role="status">当前有维护任务执行中；相互冲突的版本、网络或面板操作会保持禁用，直至任务完成。</p>
  </div>
</template>

<style scoped>
.settings-view {
  --s-card: var(--card, #fff);
  --s-text: var(--foreground, #263a33);
  --s-text-soft: color-mix(in srgb, var(--foreground, #263a33) 82%, transparent);
  --s-muted: var(--muted-foreground, #71827a);
  --s-border: var(--border, #e1e9e4);
  --s-border-soft: color-mix(in srgb, var(--border, #e1e9e4) 72%, transparent);
  --s-surface: color-mix(in srgb, var(--muted, #f1f5f2) 72%, var(--card, #fff));
  --s-primary: #4f8f79;
  --s-primary-strong: #39745f;
  --s-primary-soft: color-mix(in srgb, #69a58e 15%, var(--card, #fff));
  --s-success: #478266;
  --s-success-soft: color-mix(in srgb, #62a17e 14%, var(--card, #fff));
  --s-warning: #a0793c;
  --s-warning-soft: color-mix(in srgb, #d8b66f 15%, var(--card, #fff));
  --s-danger: #b85f62;
  --s-danger-soft: color-mix(in srgb, #cc7477 13%, var(--card, #fff));
  --s-info: #688da1;
  --s-info-soft: color-mix(in srgb, #77a7bd 14%, var(--card, #fff));
  --s-violet: #7e76aa;
  --s-violet-soft: color-mix(in srgb, #9389c1 14%, var(--card, #fff));
  display: grid;
  gap: 1rem;
  width: 100%;
  min-width: 0;
}

.settings-overview {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1.25fr) minmax(21rem, 0.75fr);
  align-items: center;
  gap: 1.5rem;
  min-width: 0;
  padding: 1.3rem 1.5rem;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--s-primary) 16%, var(--s-border));
  border-radius: 1rem;
  background: radial-gradient(circle at 96% 15%, color-mix(in srgb, var(--s-info) 11%, transparent), transparent 15rem), linear-gradient(135deg, var(--s-primary-soft), var(--s-card) 58%);
}
.settings-overview::after { content: ''; position: absolute; right: -3rem; bottom: -4rem; width: 12rem; height: 12rem; border: 1px solid color-mix(in srgb, var(--s-info) 13%, transparent); border-radius: 50%; pointer-events: none; }
.overview-copy, .overview-map { position: relative; z-index: 1; min-width: 0; }
.overview-kicker, .settings-kicker { color: var(--s-primary); font-size: 0.62rem; font-weight: 760; letter-spacing: 0.13em; }
.overview-kicker { display: inline-flex; align-items: center; gap: 0.4rem; }
.settings-kicker { display: block; margin-bottom: 0.38rem; }
.overview-copy h2 { margin: 0.55rem 0 0.5rem; color: var(--s-text); font-size: clamp(1.25rem, 2.2vw, 1.65rem); line-height: 1.25; }
.overview-copy p { max-width: 39rem; margin: 0; color: var(--s-muted); font-size: 0.74rem; line-height: 1.65; }
.overview-map { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 0.45rem; }
.overview-map span { display: inline-flex; align-items: center; gap: 0.35rem; padding: 0.5rem 0.65rem; border: 1px solid var(--s-border-soft); border-radius: 999px; background: color-mix(in srgb, var(--s-card) 84%, transparent); color: var(--s-text-soft); font-size: 0.61rem; font-weight: 650; white-space: nowrap; }
.overview-map svg { color: var(--s-primary); }

.settings-card { min-width: 0; gap: 0; padding-block: 0; overflow: hidden; border-color: var(--s-border); background: var(--s-card); box-shadow: 0 0.45rem 1.7rem color-mix(in srgb, var(--s-text) 5%, transparent); }
.card-header { display: flex; grid-template-columns: none; align-items: flex-start; justify-content: space-between; gap: 1rem; padding: 1.3rem 1.4rem 1rem; }
.card-content { padding: 0 1.4rem 1.3rem; }
.heading-group { display: flex; align-items: flex-start; gap: 0.78rem; min-width: 0; }
.heading-icon { display: grid; place-items: center; width: 2.45rem; height: 2.45rem; flex: 0 0 auto; border-radius: 0.76rem; }
.heading-icon.is-info { background: var(--s-info-soft); color: var(--s-info); }
.heading-icon.is-primary { background: var(--s-primary-soft); color: var(--s-primary); }
.heading-icon.is-violet { background: var(--s-violet-soft); color: var(--s-violet); }
.heading-icon.is-success { background: var(--s-success-soft); color: var(--s-success); }
.heading-icon.is-warning { background: var(--s-warning-soft); color: var(--s-warning); }
:deep([data-slot='card-title']) { margin: 0; color: var(--s-text); font-size: 1rem; line-height: 1.3; }
:deep([data-slot='card-description']) { margin: 0.35rem 0 0; color: var(--s-muted); font-size: 0.69rem; line-height: 1.55; }
.card-header > :deep([data-slot='badge']) { flex: 0 0 auto; font-size: 0.59rem; }
.card-header > :deep([data-slot='badge']) i { width: 0.38rem; height: 0.38rem; border-radius: 50%; background: currentColor; }
.core-accent { height: 0.22rem; background: linear-gradient(90deg, var(--s-info), var(--s-primary)); }
.core-content { display: grid; gap: 1rem; }

.version-overview { display: grid; grid-template-columns: minmax(0, 1.25fr) repeat(2, minmax(0, 0.8fr)); gap: 0.65rem; }
.version-overview > div { min-width: 0; padding: 0.9rem; border: 1px solid var(--s-border-soft); border-radius: 0.8rem; background: var(--s-surface); }
.version-overview .version-primary { background: linear-gradient(135deg, var(--s-info-soft), var(--s-card)); }
.version-overview .is-recommended { background: linear-gradient(135deg, var(--s-primary-soft), var(--s-card)); }
.version-overview span, .version-overview small { display: block; color: var(--s-muted); font-size: 0.59rem; }
.version-overview strong { display: block; margin: 0.32rem 0; color: var(--s-text); font-size: 1.05rem; overflow-wrap: anywhere; }
.version-primary strong { font-size: 1.35rem; }
.version-primary small { display: flex; align-items: center; gap: 0.3rem; color: var(--s-success); }
.version-workbench { display: grid; grid-template-columns: minmax(14rem, 0.9fr) minmax(0, 1.1fr); align-items: start; gap: 0.8rem; padding: 0.95rem; border: 1px solid var(--s-border-soft); border-radius: 0.85rem; }
.version-field { min-width: 0; }
.version-field label { display: block; margin: 0 0 0.42rem; color: var(--s-text-soft); font-size: 0.67rem; font-weight: 680; }
.version-field :deep([data-slot='native-select-wrapper']) { width: 100%; }
.version-select { min-height: 2.45rem; color: var(--s-text); }
.control-reason, .disabled-reason { margin: 0.42rem 0 0; color: var(--s-muted); font-size: 0.61rem; line-height: 1.5; }
.disabled-reason { color: var(--s-warning); }
.release-meta { display: flex; align-items: center; justify-content: space-between; gap: 0.75rem; min-width: 0; padding: 0.72rem 0.8rem; border-radius: 0.72rem; background: var(--s-surface); }
.release-meta > div { display: grid; gap: 0.18rem; min-width: 0; }
.release-meta span, .release-meta small { color: var(--s-muted); font-size: 0.57rem; }
.release-meta strong { min-width: 0; color: var(--s-text-soft); font-size: 0.67rem; overflow-wrap: anywhere; }
.release-meta a { display: inline-flex; align-items: center; gap: 0.3rem; flex: 0 0 auto; color: var(--s-primary); font-size: 0.63rem; font-weight: 680; text-decoration: none; }
.release-meta a:hover { text-decoration: underline; }
.core-actions, .action-row { display: flex; flex-wrap: wrap; gap: 0.55rem; }
.core-actions :deep(button), .action-row :deep(button), .action-row :deep(a) { min-height: 2.3rem; border-radius: 0.66rem; }
.primary-action { background: var(--s-primary); color: white; }
.primary-action:hover { background: var(--s-primary-strong); }
.danger-outline { color: var(--s-danger); }
.destructive-dialog-action { background: var(--destructive); color: var(--destructive-foreground); }

.operation-timeline { display: grid; gap: 0.85rem; padding: 1rem; border: 1px solid var(--s-border-soft); border-radius: 0.85rem; background: var(--s-surface); }
.operation-timeline[data-tone='success'] { border-color: color-mix(in srgb, var(--s-success) 24%, var(--s-border)); background: var(--s-success-soft); }
.operation-timeline[data-tone='rollback'] { border-color: color-mix(in srgb, var(--s-warning) 28%, var(--s-border)); background: var(--s-warning-soft); }
.operation-timeline[data-tone='danger'] { border-color: color-mix(in srgb, var(--s-danger) 28%, var(--s-border)); background: var(--s-danger-soft); }
.timeline-header, .timeline-state { display: flex; align-items: center; }
.timeline-header { justify-content: space-between; gap: 0.8rem; }
.timeline-state { gap: 0.55rem; min-width: 0; color: var(--s-primary); }
[data-tone='success'] .timeline-state { color: var(--s-success); }
[data-tone='rollback'] .timeline-state { color: var(--s-warning); }
[data-tone='danger'] .timeline-state { color: var(--s-danger); }
.timeline-state > div { display: grid; gap: 0.12rem; min-width: 0; }
.timeline-state span { color: var(--s-muted); font-size: 0.58rem; }
.timeline-state strong { color: var(--s-text); font-size: 0.72rem; }
.timeline-header :deep([data-slot='badge']) { flex: 0 0 auto; border-color: var(--s-border); color: var(--s-muted); font-size: 0.57rem; }
.timeline-track { position: relative; min-width: 0; padding-top: 0.15rem; }
.timeline-progress { height: 0.18rem; background: color-mix(in srgb, var(--s-border) 80%, transparent); }
.timeline-progress :deep(div) { background: var(--s-primary); }
[data-tone='success'] .timeline-progress :deep(div) { background: var(--s-success); }
[data-tone='rollback'] .timeline-progress :deep(div) { background: var(--s-warning); }
[data-tone='danger'] .timeline-progress :deep(div) { background: var(--s-danger); }
.timeline-track ol { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 0.4rem; margin: 0.5rem 0 0; padding: 0; list-style: none; }
.timeline-track ol.has-rollback { grid-template-columns: repeat(6, minmax(0, 1fr)); }
.timeline-track li { display: grid; justify-items: center; gap: 0.35rem; min-width: 0; color: var(--s-muted); font-size: 0.55rem; text-align: center; }
.timeline-track li > span { display: grid; place-items: center; width: 1.35rem; height: 1.35rem; border: 1px solid var(--s-border); border-radius: 50%; background: var(--s-card); color: var(--s-muted); }
.timeline-track li em { font-style: normal; }
.timeline-track li[data-state='done'] > span, .timeline-track li[data-state='rollback-success'] > span { border-color: var(--s-success); background: var(--s-success); color: white; }
.timeline-track li[data-state='active'] > span { border-color: var(--s-primary); background: var(--s-primary-soft); color: var(--s-primary); }
.timeline-track li[data-state='failed'] > span { border-color: var(--s-danger); background: var(--s-danger-soft); color: var(--s-danger); }
.timeline-track li[data-state='done'], .timeline-track li[data-state='rollback-success'] { color: var(--s-text-soft); }
.timeline-track li[data-state='active'] { color: var(--s-primary); font-weight: 700; }
.timeline-track li[data-state='failed'] { color: var(--s-danger); }
.timeline-message { display: flex; align-items: flex-start; gap: 0.45rem; color: var(--s-primary); }
[data-tone='success'] .timeline-message { color: var(--s-success); }
[data-tone='rollback'] .timeline-message { color: var(--s-warning); }
[data-tone='danger'] .timeline-message { color: var(--s-danger); }
.timeline-message svg { flex: 0 0 auto; margin-top: 0.05rem; }
.timeline-message p { margin: 0; color: var(--s-text-soft); font-size: 0.64rem; line-height: 1.55; overflow-wrap: anywhere; }
.core-empty { display: grid; justify-items: center; gap: 0.38rem; min-height: 10rem; place-content: center; color: var(--s-muted); text-align: center; }
.core-empty strong { color: var(--s-text); font-size: 0.78rem; }
.core-empty span { font-size: 0.65rem; }

.settings-pair-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: stretch; gap: 1rem; min-width: 0; }
.settings-pair-grid > * { min-width: 0; }
.compact-header { padding: 1.2rem 1.25rem 0.9rem; }
.section-content { display: grid; gap: 0.8rem; padding: 0 1.25rem 1.2rem; }
.credential-form { display: grid; gap: 0.8rem; }
.credential-fields { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.7rem; }
.credential-fields :deep([data-slot='field-label']) { color: var(--s-text-soft); font-size: 0.65rem; }
.credential-fields :deep(input) { min-height: 2.35rem; color: var(--s-text); }
.field-help { margin: 0; color: var(--s-muted); font-size: 0.61rem; line-height: 1.55; }
.readout-grid, .update-summary { display: grid; gap: 0.6rem; }
.readout-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.update-summary { grid-template-columns: minmax(0, 0.8fr) minmax(0, 0.8fr) minmax(0, 1.4fr); }
.readout-grid > div, .update-summary > div { min-width: 0; padding: 0.78rem; border: 1px solid var(--s-border-soft); border-radius: 0.7rem; background: var(--s-surface); }
.readout-grid span, .update-summary span { display: block; color: var(--s-muted); font-size: 0.57rem; }
.readout-grid strong, .update-summary strong { display: block; margin-top: 0.25rem; color: var(--s-text); font-size: 0.8rem; overflow-wrap: anywhere; }
.operation-readout strong { color: var(--s-primary); }
.network-note { display: flex; align-items: flex-start; gap: 0.48rem; padding: 0.7rem; border-radius: 0.68rem; background: var(--s-violet-soft); color: var(--s-violet); }
.network-note svg { flex: 0 0 auto; margin-top: 0.05rem; }
.network-note p { margin: 0; color: var(--s-text-soft); font-size: 0.61rem; line-height: 1.55; }
.fit-action { width: fit-content; min-height: 2.3rem; border-radius: 0.66rem; }
.error-message { margin: 0; padding: 0.7rem; border-radius: 0.65rem; background: var(--s-danger-soft); color: var(--s-danger); font-size: 0.62rem; line-height: 1.5; }
.success-message { display: flex; align-items: flex-start; gap: 0.4rem; margin: 0; color: var(--s-success); font-size: 0.61rem; line-height: 1.55; }
.success-message svg { flex: 0 0 auto; margin-top: 0.05rem; }

.safety-layout { display: grid; grid-template-columns: minmax(0, 1.35fr) minmax(16rem, 0.65fr); gap: 0.8rem; }
.security-section, .theme-section { min-width: 0; padding: 0.9rem; border: 1px solid var(--s-border-soft); border-radius: 0.8rem; background: var(--s-surface); }
.subheading { display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.75rem; color: var(--s-primary); }
.subheading > div { display: grid; gap: 0.12rem; min-width: 0; }
.subheading strong { color: var(--s-text); font-size: 0.71rem; }
.subheading span { color: var(--s-muted); font-size: 0.55rem; }
.security-list { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.5rem; margin: 0; padding: 0; list-style: none; }
.security-list li { display: flex; align-items: flex-start; gap: 0.45rem; min-width: 0; padding: 0.65rem; border-radius: 0.65rem; background: var(--s-card); color: var(--s-success); }
.security-list li > svg { flex: 0 0 auto; margin-top: 0.05rem; }
.security-list span { min-width: 0; color: var(--s-muted); font-size: 0.56rem; }
.security-list strong { display: block; margin-top: 0.18rem; color: var(--s-text-soft); font-size: 0.63rem; line-height: 1.45; overflow-wrap: anywhere; }
.theme-panel { display: grid; gap: 0.75rem; padding: 0.75rem; border-radius: 0.68rem; background: var(--s-card); }
.theme-panel :deep(.theme-toggle-label) { color: var(--s-text-soft); }
.theme-panel p { margin: 0; color: var(--s-muted); font-size: 0.59rem; line-height: 1.55; }
.maintenance-lock-note { margin: -0.2rem 0 0; padding: 0.75rem 0.9rem; border: 1px solid color-mix(in srgb, var(--s-warning) 24%, var(--s-border)); border-radius: 0.7rem; background: var(--s-warning-soft); color: var(--s-warning); font-size: 0.62rem; line-height: 1.5; }
.spin { animation: settings-spin 0.9s linear infinite; }
@keyframes settings-spin { to { transform: rotate(360deg); } }

@media (max-width: 980px) {
  .settings-overview { grid-template-columns: 1fr; }
  .overview-map { justify-content: flex-start; }
  .settings-pair-grid { grid-template-columns: 1fr; }
}

@media (max-width: 860px) {
  .safety-layout { grid-template-columns: 1fr; }
}

@media (max-width: 760px) {
  .version-overview { grid-template-columns: 1fr 1fr; }
  .version-primary { grid-column: 1 / -1; }
  .version-workbench { grid-template-columns: 1fr; }
}

@media (max-width: 640px) {
  .settings-view { gap: 0.8rem; }
  .settings-overview { gap: 1rem; padding: 1.15rem 1rem; }
  .overview-map { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .overview-map span { min-width: 0; justify-content: flex-start; overflow: hidden; text-overflow: ellipsis; }
  .card-header { padding: 1.05rem 1rem 0.85rem; }
  .card-content { padding: 0 1rem 1rem; }
  .card-header > :deep([data-slot='badge']) { display: none; }
  .version-overview { grid-template-columns: 1fr; }
  .version-primary { grid-column: auto; }
  .release-meta { align-items: flex-start; flex-direction: column; }
  .core-actions, .action-row { display: grid; grid-template-columns: 1fr; }
  .core-actions :deep(button), .action-row :deep(button), .action-row :deep(a) { width: 100%; }
  .timeline-header { align-items: flex-start; flex-direction: column; }
  .timeline-track { overflow-x: auto; padding-bottom: 0.35rem; }
  .timeline-track ol, .timeline-progress { min-width: 28rem; }
  .credential-fields, .readout-grid { grid-template-columns: 1fr; }
  .update-summary { grid-template-columns: 1fr 1fr; }
  .operation-readout { grid-column: 1 / -1; }
  .fit-action { width: 100%; }
  .security-list { grid-template-columns: 1fr; }
}

@media (prefers-reduced-motion: reduce) {
  .spin { animation: none; }
}
</style>
