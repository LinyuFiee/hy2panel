<script setup>
import { computed, ref, watch } from 'vue'
import {
  Check,
  Circle,
  CircleAlert,
  Database,
  Download,
  ExternalLink,
  Gauge,
  KeyRound,
  LoaderCircle,
  RefreshCw,
  RotateCcw,
  ShieldCheck,
  Trash2,
  XCircle
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
  <div class="settings">
    <article class="card" data-testid="sing-box-version-card">
      <header class="card-head">
        <div class="card-title">
          <h3>sing-box 核心</h3>
          <small>仅切换兼容清单内、具有固定校验值的官方版本。</small>
        </div>
        <Badge :variant="operationRunning ? 'warning' : 'success'"><i></i>{{ operationRunning ? '维护中' : '运行可用' }}</Badge>
      </header>

      <div class="card-body core-body">
        <template v-if="singBox">
          <div class="version-tiles">
            <div class="tile">
              <span>当前运行版本</span>
              <strong class="tnum">v{{ singBox.current_version || '未知' }}</strong>
              <small><ShieldCheck :size="12" />受兼容清单保护</small>
            </div>
            <div class="tile">
              <span>上一版本</span>
              <strong class="tnum">{{ singBox.previous_version ? `v${singBox.previous_version}` : '暂无' }}</strong>
              <small>{{ singBox.can_rollback ? '可用于手动回滚' : '没有回滚快照' }}</small>
            </div>
            <div class="tile">
              <span>推荐版本</span>
              <strong class="tnum">v{{ singBox.recommended_version || '未知' }}</strong>
              <small>维护清单建议</small>
            </div>
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
              <div>
                <span>官方资产</span>
                <strong>{{ selectedRelease?.asset_name || '正在读取兼容版本…' }}</strong>
                <small v-if="selectedRelease" class="tnum">{{ formatBytes(selectedRelease.size) }} · SHA-256 固定校验</small>
              </div>
              <a v-if="selectedRelease?.release_url" :href="selectedRelease.release_url" target="_blank" rel="noopener noreferrer">Release <ExternalLink :size="13" /></a>
            </div>
          </div>

          <div class="action-row">
            <AlertDialog>
              <AlertDialogTrigger as-child>
                <Button :disabled="Boolean(switchDisabledReason)">
                  <RefreshCw :size="15" :class="{ spin: operationRunning && operation.action === 'switch' }" />
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
                  <RotateCcw :size="15" :class="{ spin: operationRunning && operation.action === 'rollback' }" />
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
      </div>
    </article>

    <div class="pair-grid">
      <article class="card">
        <header class="card-head">
          <div class="card-title">
            <h3>RackNerd 官方流量</h3>
            <small>凭据仅用于只读 info 请求，不提供关机或重启能力。</small>
          </div>
          <Badge v-if="state.racknerd?.configured" variant="success"><ShieldCheck :size="13" />已配置</Badge>
          <Badge v-else variant="outline">未配置</Badge>
        </header>
        <div class="card-body stack">
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
              <Button type="submit" :disabled="busy.racknerd"><KeyRound :size="15" />{{ busy.racknerd ? '正在验证…' : '保存并获取流量' }}</Button>
              <AlertDialog v-if="state.racknerd?.configured">
                <AlertDialogTrigger as-child><Button type="button" variant="outline" class="danger-outline" :disabled="busy.racknerd"><Trash2 :size="15" />清除凭据</Button></AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader><AlertDialogTitle>清除 RackNerd 凭据？</AlertDialogTitle><AlertDialogDescription>清除后面板将停止拉取官方套餐流量；节点和代理服务不会受到影响。需要再次显示套餐流量时必须重新输入 Key 与 Hash。</AlertDialogDescription></AlertDialogHeader>
                  <AlertDialogFooter><AlertDialogCancel>取消</AlertDialogCancel><AlertDialogAction class="destructive-dialog-action" @click="emit('clear-racknerd')">确认清除</AlertDialogAction></AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </div>
          </form>
          <p v-if="state.racknerd?.error" class="error-message" role="status">{{ state.racknerd.error }}</p>
        </div>
      </article>

      <article class="card">
        <header class="card-head">
          <div class="card-title">
            <h3>BBR 网络优化</h3>
            <small>应用项目内置的 BBR + fq 保守模板，不接受浏览器任意命令。</small>
          </div>
          <Badge :variant="state.status.bbr_enabled ? 'success' : 'warning'">{{ state.status.bbr_enabled ? '已启用' : '待处理' }}</Badge>
        </header>
        <div class="card-body stack">
          <div class="readout-grid">
            <div><span>拥塞控制</span><strong>{{ state.status.congestion_control || '未知' }}</strong></div>
            <div><span>队列规则</span><strong>{{ state.status.default_qdisc || '未知' }}</strong></div>
          </div>
          <p class="network-note"><Gauge :size="14" /><span>BBR 主要优化 TCP；Hysteria2 使用 QUIC 自有拥塞控制，不会被 BBR 直接加速。</span></p>
          <AlertDialog>
            <AlertDialogTrigger as-child><Button variant="outline" class="fit-action" :disabled="busy.bbr || Boolean(bbrDisabledReason)"><RotateCcw :size="15" :class="{ spin: busy.bbr }" />{{ busy.bbr ? '正在应用…' : '应用 BBR + fq' }}</Button></AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader><AlertDialogTitle>应用 BBR + fq 模板？</AlertDialogTitle><AlertDialogDescription>系统将写入项目固定的内核网络参数并立即加载。此操作不会执行浏览器提供的命令，也不会修改节点协议配置。</AlertDialogDescription></AlertDialogHeader>
              <AlertDialogFooter><AlertDialogCancel>取消</AlertDialogCancel><AlertDialogAction @click="emit('apply-bbr')">确认应用</AlertDialogAction></AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
          <p v-if="bbrDisabledReason" class="disabled-reason">当前不可用：{{ bbrDisabledReason }}</p>
        </div>
      </article>
    </div>

    <article class="card">
      <header class="card-head">
        <div class="card-title">
          <h3>面板更新</h3>
          <small>从官方 Release 检测，安装前强制校验 SHA-256。</small>
        </div>
        <Badge v-if="state.update?.available" variant="success">发现新版本</Badge><Badge v-else variant="outline">{{ updateChecking ? '检测中' : '稳定' }}</Badge>
      </header>
      <div class="card-body stack">
        <div class="update-summary">
          <div><span>当前版本</span><strong class="tnum">v{{ state.update?.current_version || '未知' }}</strong></div>
          <div><span>最新版本</span><strong class="tnum">{{ state.update?.latest_version ? `v${state.update.latest_version}` : '尚未检测' }}</strong></div>
          <div class="operation-readout"><span>操作状态</span><strong>{{ updateStatusText }}</strong></div>
        </div>
        <div class="action-row">
          <Button variant="outline" :disabled="updateChecking || updateRunning" @click="emit('check-update')"><RefreshCw :size="15" :class="{ spin: updateChecking }" />{{ updateChecking ? '正在检测…' : '检测更新' }}</Button>
          <AlertDialog v-if="state.update?.available">
            <AlertDialogTrigger as-child><Button :disabled="updateRunning || Boolean(updateDisabledReason)"><Download :size="15" />{{ updateRunning ? '正在更新…' : '立即更新' }}</Button></AlertDialogTrigger>
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
      </div>
    </article>

    <article class="card">
      <header class="card-head">
        <div class="card-title">
          <h3>安全与外观</h3>
          <small>服务器强制执行的访问边界，与仅保存在本机浏览器的外观偏好。</small>
        </div>
      </header>
      <div class="card-body">
        <div class="safety-layout">
          <section aria-label="访问安全">
            <ul class="security-list">
              <li><ShieldCheck :size="15" /><span>公网入口<strong>仅通过 HTTPS 访问</strong></span></li>
              <li><ShieldCheck :size="15" /><span>登录防护<strong>按来源限制登录尝试速率</strong></span></li>
              <li><ShieldCheck :size="15" /><span>安全会话<strong>HttpOnly · SameSite Strict</strong></span></li>
              <li><Database :size="15" /><span>敏感配置<strong>仅 root / hy2panel 可读取</strong></span></li>
            </ul>
          </section>
          <section aria-label="外观主题" class="theme-section">
            <ThemeToggle label="界面主题" />
            <p class="theme-note">跟随系统会根据设备偏好自动选择浅色或深色，不会同步到服务器。</p>
          </section>
        </div>
      </div>
    </article>

    <p v-if="maintenanceLocked" class="maintenance-lock-note" role="status">当前有维护任务执行中；相互冲突的版本、网络或面板操作会保持禁用，直至任务完成。</p>
  </div>
</template>

<style scoped>
.settings {
  display: grid;
  gap: 0.85rem;
  width: 100%;
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

.card-head > [data-slot='badge'] {
  flex: 0 0 auto;
}

.card-head > [data-slot='badge'] i {
  width: 0.36rem;
  height: 0.36rem;
  border-radius: 50%;
  background: currentColor;
}

.card-body {
  padding: 0.8rem 1rem 0.9rem;
}

.stack {
  display: grid;
  gap: 0.8rem;
}

/* ---------- core ---------- */

.core-body {
  display: grid;
  gap: 0.75rem;
}

.version-tiles {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) repeat(2, minmax(0, 0.8fr));
  gap: 0;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 0.6rem;
}

.tile {
  min-width: 0;
  padding: 0.68rem 0.75rem;
  background: color-mix(in srgb, var(--muted) 48%, var(--card));
}

.tile + .tile {
  border-left: 1px solid var(--border);
}

.tile span,
.tile small {
  display: block;
  color: var(--muted-foreground);
  font-size: 0.7rem;
}

.tile small {
  display: flex;
  align-items: center;
  gap: 0.3rem;
}

.tile strong {
  display: block;
  margin: 0.22rem 0;
  color: var(--foreground);
  font-size: 1.08rem;
  font-weight: 700;
  letter-spacing: -0.02em;
  overflow-wrap: anywhere;
}

.version-workbench {
  display: grid;
  grid-template-columns: minmax(14rem, 0.9fr) minmax(0, 1.1fr);
  align-items: start;
  gap: 0.7rem;
  padding: 0.7rem;
  border: 1px solid var(--border);
  border-radius: 0.6rem;
  background: color-mix(in srgb, var(--muted) 25%, var(--card));
}

.version-field {
  min-width: 0;
}

.version-field > label {
  display: block;
  margin: 0 0 0.4rem;
  color: var(--foreground);
  font-size: 0.78rem;
  font-weight: 600;
}

.version-field :deep([data-slot='native-select-wrapper']) {
  width: 100%;
}

.version-select {
  min-height: 2.5rem;
  border-radius: 0.6rem;
  color: var(--foreground);
}

.control-reason,
.disabled-reason {
  margin: 0.4rem 0 0;
  color: var(--muted-foreground);
  font-size: 0.72rem;
  line-height: 1.5;
}

.disabled-reason {
  color: var(--warning);
}

.release-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.65rem;
  min-width: 0;
  padding: 0.62rem 0.7rem;
  border-left: 1px solid var(--border);
}

.release-meta > div {
  display: grid;
  gap: 0.2rem;
  min-width: 0;
}

.release-meta span,
.release-meta small {
  color: var(--muted-foreground);
  font-size: 0.68rem;
}

.release-meta strong {
  min-width: 0;
  color: var(--foreground);
  font-size: 0.76rem;
  overflow-wrap: anywhere;
}

.release-meta a {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  flex: 0 0 auto;
  color: var(--primary);
  font-size: 0.74rem;
  font-weight: 600;
  text-decoration: none;
}

.release-meta a:hover {
  text-decoration: underline;
}

.action-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.55rem;
}

.danger-outline {
  color: var(--destructive);
}

.destructive-dialog-action {
  background: var(--destructive);
  color: var(--destructive-foreground);
}

/* ---------- timeline ---------- */

.operation-timeline {
  display: grid;
  gap: 0.7rem;
  padding: 0.75rem;
  border: 1px solid var(--border);
  border-radius: 0.65rem;
  background: color-mix(in srgb, var(--muted) 52%, var(--card));
}

.operation-timeline[data-tone='success'] {
  background: color-mix(in srgb, var(--success) 8%, var(--card));
}

.operation-timeline[data-tone='rollback'] {
  background: color-mix(in srgb, var(--warning) 10%, var(--card));
}

.operation-timeline[data-tone='danger'] {
  background: color-mix(in srgb, var(--destructive) 8%, var(--card));
}

.timeline-header,
.timeline-state {
  display: flex;
  align-items: center;
}

.timeline-header {
  justify-content: space-between;
  gap: 0.8rem;
}

.timeline-state {
  gap: 0.55rem;
  min-width: 0;
  color: var(--primary);
}

[data-tone='success'] .timeline-state {
  color: var(--success);
}

[data-tone='rollback'] .timeline-state {
  color: var(--warning);
}

[data-tone='danger'] .timeline-state {
  color: var(--destructive);
}

.timeline-state > div {
  display: grid;
  gap: 0.12rem;
  min-width: 0;
}

.timeline-state span {
  color: var(--muted-foreground);
  font-size: 0.7rem;
}

.timeline-state strong {
  color: var(--foreground);
  font-size: 0.82rem;
}

.timeline-track {
  position: relative;
  min-width: 0;
  padding-top: 0.15rem;
}

.timeline-progress {
  height: 0.2rem;
  background: color-mix(in srgb, var(--border) 80%, transparent);
}

.timeline-track ol {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 0.2rem;
  margin: 0.45rem 0 0;
  padding: 0;
  list-style: none;
}

.timeline-track ol.has-rollback {
  grid-template-columns: repeat(6, minmax(0, 1fr));
}

.timeline-track li {
  display: grid;
  justify-items: center;
  gap: 0.25rem;
  min-width: 0;
  color: var(--muted-foreground);
  font-size: 0.64rem;
  text-align: center;
}

.timeline-track li > span {
  display: grid;
  place-items: center;
  width: 1.25rem;
  height: 1.25rem;
  border: 1px solid var(--border);
  border-radius: 50%;
  background: var(--card);
  color: var(--muted-foreground);
}

.timeline-track li em {
  font-style: normal;
}

.timeline-track li[data-state='done'] > span,
.timeline-track li[data-state='rollback-success'] > span {
  border-color: var(--success);
  background: var(--success);
  color: #fff;
}

.timeline-track li[data-state='active'] > span {
  border-color: var(--primary);
  background: color-mix(in srgb, var(--primary) 10%, transparent);
  color: var(--primary);
}

.timeline-track li[data-state='failed'] > span {
  border-color: var(--destructive);
  background: color-mix(in srgb, var(--destructive) 10%, transparent);
  color: var(--destructive);
}

.timeline-track li[data-state='done'],
.timeline-track li[data-state='rollback-success'] {
  color: var(--foreground);
}

.timeline-track li[data-state='active'] {
  color: var(--primary);
  font-weight: 600;
}

.timeline-track li[data-state='failed'] {
  color: var(--destructive);
}

.timeline-message {
  display: flex;
  align-items: flex-start;
  gap: 0.45rem;
  color: var(--primary);
}

[data-tone='success'] .timeline-message {
  color: var(--success);
}

[data-tone='rollback'] .timeline-message {
  color: var(--warning);
}

[data-tone='danger'] .timeline-message {
  color: var(--destructive);
}

.timeline-message svg {
  flex: 0 0 auto;
  margin-top: 0.05rem;
}

.timeline-message p {
  margin: 0;
  color: var(--foreground);
  font-size: 0.74rem;
  line-height: 1.55;
  overflow-wrap: anywhere;
}

.core-empty {
  display: grid;
  justify-items: center;
  gap: 0.38rem;
  min-height: 9rem;
  place-content: center;
  color: var(--muted-foreground);
  text-align: center;
}

.core-empty strong {
  color: var(--foreground);
  font-size: 0.85rem;
}

.core-empty span {
  font-size: 0.72rem;
}

/* ---------- pair cards ---------- */

.pair-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
  align-items: start;
}

.credential-form {
  display: grid;
  gap: 0.65rem;
}

.credential-fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.6rem;
  padding: 0.7rem;
  border: 1px solid var(--border);
  border-radius: 0.6rem;
  background: color-mix(in srgb, var(--muted) 25%, var(--card));
}

.credential-fields :deep([data-slot='field-label']) {
  color: var(--foreground);
  font-size: 0.76rem;
  font-weight: 600;
}

.credential-fields :deep(input) {
  min-height: 2.5rem;
  border-radius: 0.6rem;
  color: var(--foreground);
}

.field-help {
  margin: 0;
  color: var(--muted-foreground);
  font-size: 0.72rem;
  line-height: 1.55;
}

.readout-grid,
.update-summary {
  display: grid;
  gap: 0.6rem;
}

.readout-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 0.6rem;
}

.update-summary {
  grid-template-columns: minmax(0, 0.8fr) minmax(0, 0.8fr) minmax(0, 1.4fr);
  gap: 0;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 0.6rem;
}

.readout-grid > div,
.update-summary > div {
  min-width: 0;
  padding: 0.62rem 0.7rem;
  background: color-mix(in srgb, var(--muted) 48%, var(--card));
}

.readout-grid > div + div,
.update-summary > div + div {
  border-left: 1px solid var(--border);
}

.readout-grid span,
.update-summary span {
  display: block;
  color: var(--muted-foreground);
  font-size: 0.7rem;
}

.readout-grid strong,
.update-summary strong {
  display: block;
  margin-top: 0.25rem;
  color: var(--foreground);
  font-size: 0.88rem;
  overflow-wrap: anywhere;
}

.operation-readout strong {
  color: var(--primary);
}

.network-note {
  display: flex;
  align-items: flex-start;
  gap: 0.48rem;
  margin: 0;
  color: var(--muted-foreground);
  font-size: 0.72rem;
  line-height: 1.55;
}

.network-note svg {
  flex: 0 0 auto;
  margin-top: 0.1rem;
  color: var(--primary);
}

.fit-action {
  width: fit-content;
}

.error-message {
  margin: 0;
  padding: 0.7rem 0.8rem;
  border-radius: 0.7rem;
  background: color-mix(in srgb, var(--destructive) 9%, transparent);
  color: var(--destructive);
  font-size: 0.72rem;
  line-height: 1.5;
}

.success-message {
  display: flex;
  align-items: flex-start;
  gap: 0.4rem;
  margin: 0;
  color: var(--success);
  font-size: 0.74rem;
  line-height: 1.55;
}

.success-message svg {
  flex: 0 0 auto;
  margin-top: 0.05rem;
}

/* ---------- security & theme ---------- */

.safety-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(15rem, 0.6fr);
  gap: 0;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 0.65rem;
}

.security-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0;
  margin: 0;
  padding: 0;
  list-style: none;
}

.security-list li {
  display: flex;
  align-items: flex-start;
  gap: 0.45rem;
  min-width: 0;
  padding: 0.65rem 0.7rem;
  background: color-mix(in srgb, var(--muted) 30%, var(--card));
}

.security-list li:nth-child(even) {
  border-left: 1px solid var(--border);
}

.security-list li:nth-child(n + 3) {
  border-top: 1px solid var(--border);
}

.security-list li > svg {
  flex: 0 0 auto;
  margin-top: 0.08rem;
  color: var(--success);
}

.security-list span {
  min-width: 0;
  color: var(--muted-foreground);
  font-size: 0.7rem;
}

.security-list strong {
  display: block;
  margin-top: 0.18rem;
  color: var(--foreground);
  font-size: 0.76rem;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.theme-section {
  display: grid;
  gap: 0.6rem;
  align-content: start;
  padding: 0.75rem;
  border-left: 1px solid var(--border);
  background: color-mix(in srgb, var(--muted) 48%, var(--card));
}

.theme-note {
  margin: 0;
  color: var(--muted-foreground);
  font-size: 0.7rem;
  line-height: 1.55;
}

.maintenance-lock-note {
  margin: 0;
  padding: 0.75rem 0.9rem;
  border-radius: 0.75rem;
  background: color-mix(in srgb, var(--warning) 10%, transparent);
  color: var(--warning);
  font-size: 0.74rem;
  line-height: 1.5;
}

.spin {
  animation: settings-spin 0.9s linear infinite;
}

@keyframes settings-spin {
  to { transform: rotate(360deg); }
}

@media (max-width: 980px) {
  .pair-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 860px) {
  .safety-layout {
    grid-template-columns: 1fr;
  }

  .theme-section {
    border-top: 1px solid var(--border);
    border-left: 0;
  }
}

@media (max-width: 760px) {
  .version-tiles {
    grid-template-columns: 1fr 1fr;
  }

  .version-tiles .tile:first-child {
    grid-column: 1 / -1;
    border-bottom: 1px solid var(--border);
  }

  .version-tiles .tile:nth-child(2) {
    border-left: 0;
  }

  .version-workbench {
    grid-template-columns: 1fr;
  }

  .release-meta {
    border-top: 1px solid var(--border);
    border-left: 0;
  }
}

@media (max-width: 640px) {
  .card-head {
    gap: 0.6rem;
  }

  .card-head > [data-slot='badge'] {
    flex: 0 0 auto;
  }

  .version-tiles {
    grid-template-columns: 1fr;
  }

  .version-tiles .tile:first-child {
    grid-column: auto;
  }

  .version-tiles .tile + .tile {
    border-top: 1px solid var(--border);
    border-left: 0;
  }

  .release-meta {
    align-items: flex-start;
    flex-direction: column;
  }

  .action-row {
    display: grid;
    grid-template-columns: 1fr;
  }

  .action-row > * {
    width: 100%;
  }

  .timeline-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .timeline-track {
    overflow: visible;
    padding-bottom: 0;
  }

  .timeline-track ol,
  .timeline-progress {
    min-width: 0;
  }

  .timeline-track li em {
    overflow-wrap: anywhere;
  }

  .credential-fields,
  .readout-grid {
    grid-template-columns: 1fr;
  }

  .readout-grid > div + div {
    border-top: 1px solid var(--border);
    border-left: 0;
  }

  .update-summary {
    grid-template-columns: 1fr 1fr;
  }

  .operation-readout {
    grid-column: 1 / -1;
    border-top: 1px solid var(--border);
    border-left: 0 !important;
  }

  .fit-action {
    width: 100%;
  }

  .security-list {
    grid-template-columns: 1fr;
  }

  .security-list li:nth-child(even) {
    border-left: 0;
  }

  .security-list li:nth-child(n + 2) {
    border-top: 1px solid var(--border);
  }
}

@media (max-width: 380px) {
  .card-head,
  .card-body {
    padding-inline: 0.75rem;
  }

  .card-head {
    gap: 0.55rem;
  }

  .card-head > [data-slot='badge'] {
    max-width: 7rem;
    white-space: normal;
    text-align: center;
  }

  .version-workbench,
  .credential-fields {
    padding: 0.6rem;
  }

  .update-summary {
    grid-template-columns: 1fr;
  }

  .update-summary > div + div {
    border-top: 1px solid var(--border);
    border-left: 0;
  }

  .operation-readout {
    grid-column: auto;
  }

  .timeline-track li {
    font-size: 0.58rem;
  }

  .timeline-track li > span {
    width: 1.15rem;
    height: 1.15rem;
  }
}

@media (prefers-reduced-motion: reduce) {
  .spin {
    animation: none;
  }
}
</style>
