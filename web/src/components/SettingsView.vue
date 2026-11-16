<script setup>
import { computed, ref, watch } from 'vue'
import { CheckCircle2, CloudCog, Database, Download, ExternalLink, KeyRound, LockKeyhole, RefreshCw, RotateCcw, ServerCog, ShieldCheck, Trash2, Zap } from '@lucide/vue'
import { formatBytes } from '../utils'

const props = defineProps({
  state: { type: Object, required: true },
  busy: { type: Object, required: true },
  singBox: { type: Object, default: null },
  singBoxLoading: { type: Boolean, default: false }
})
const emit = defineEmits(['save-racknerd', 'clear-racknerd', 'apply-bbr', 'check-update', 'apply-update', 'switch-sing-box', 'rollback-sing-box'])
const apiKey = ref('')
const apiHash = ref('')
const selectedVersion = ref('')
const versions = computed(() => props.singBox?.versions || [])
const operation = computed(() => props.singBox?.operation || {})
const operationRunning = computed(() => operation.value.status === 'running')
const controlsDisabled = computed(() => props.singBoxLoading || operationRunning.value || props.busy.singBox)
const selectedRelease = computed(() => versions.value.find(entry => entry.version === selectedVersion.value) || null)
const canSwitch = computed(() => Boolean(selectedVersion.value) && selectedVersion.value !== props.singBox?.current_version && !controlsDisabled.value)

watch(() => [props.singBox?.recommended_version, props.singBox?.current_version, versions.value.length], () => {
  if (!versions.value.some(entry => entry.version === selectedVersion.value)) {
    selectedVersion.value = props.singBox?.recommended_version || props.singBox?.current_version || versions.value[0]?.version || ''
  }
}, { immediate: true })

function clearCredentials() {
  apiKey.value = ''
  apiHash.value = ''
}

function submitSwitch() {
  if (canSwitch.value) emit('switch-sing-box', selectedVersion.value)
}

function operationLabel(value) {
  return {
    requested: '已提交',
    downloading: '下载核心',
    verifying: '校验文件',
    installing: '安装核心',
    restarting: '重启服务',
    healthcheck: '健康检查',
    completed: '已完成',
    failed: '失败',
    rollback: '正在回滚'
  }[value] || value || '空闲'
}

defineExpose({ clearCredentials })
</script>

<template>
  <div class="settings-grid">
    <section class="panel-card singbox-card" data-testid="sing-box-version-card">
      <div class="settings-heading">
        <div class="protocol-icon sky"><ServerCog :size="22" /></div>
        <div><h3>sing-box 核心</h3><p>只允许切换到兼容清单内、已固定校验值的官方版本。</p></div>
        <span class="status-pill" :class="operationRunning ? 'warn' : 'ok'">{{ operationRunning ? '维护中' : '可用' }}</span>
      </div>
      <div v-if="singBox" class="singbox-readout">
        <div><span>当前版本</span><strong>v{{ singBox.current_version || '未知' }}</strong></div>
        <div><span>上一版本</span><strong>{{ singBox.previous_version ? `v${singBox.previous_version}` : '暂无' }}</strong></div>
        <div><span>推荐版本</span><strong>v{{ singBox.recommended_version || '未知' }}</strong></div>
      </div>
      <div v-if="singBox" class="singbox-controls">
        <label for="sing-box-version">目标版本
          <select id="sing-box-version" v-model="selectedVersion" :disabled="controlsDisabled || !versions.length">
            <option v-for="entry in versions" :key="entry.version" :value="entry.version">
              v{{ entry.version }}{{ entry.status === 'recommended' ? ' · 推荐' : '' }}{{ entry.version === singBox.current_version ? ' · 当前' : '' }}
            </option>
          </select>
        </label>
        <div class="singbox-version-meta">
          <span v-if="selectedRelease">{{ selectedRelease.asset_name }} · {{ formatBytes(selectedRelease.size) }}</span>
          <span v-else>正在读取兼容版本…</span>
          <a v-if="selectedRelease?.release_url" :href="selectedRelease.release_url" target="_blank" rel="noopener noreferrer">Release <ExternalLink :size="13" /></a>
        </div>
        <div class="button-row">
          <button class="primary-button" type="button" :disabled="!canSwitch" @click="submitSwitch"><RefreshCw :size="16" :class="{ spin: operationRunning && operation.action === 'switch' }" />{{ operationRunning && operation.action === 'switch' ? '正在切换…' : '切换核心' }}</button>
          <button class="secondary-button" type="button" :disabled="controlsDisabled || !singBox.can_rollback" @click="emit('rollback-sing-box')"><RotateCcw :size="16" :class="{ spin: operationRunning && operation.action === 'rollback' }" />{{ operationRunning && operation.action === 'rollback' ? '正在回滚…' : '回滚上一版本' }}</button>
        </div>
      </div>
      <div v-else class="empty-state compact"><p>{{ singBoxLoading ? '正在读取 sing-box 兼容版本…' : '核心版本信息暂不可用。' }}</p></div>
      <div v-if="operation.id || operation.status" class="operation-status" :class="operation.status" role="status" aria-live="polite">
        <div><span>任务状态</span><strong>{{ operation.status || 'unknown' }}</strong></div>
        <div><span>阶段</span><strong>{{ operationLabel(operation.phase) }}</strong></div>
        <p>{{ operation.message || '核心维护任务正在处理。' }}</p>
      </div>
      <p class="field-help singbox-help">切换会短暂重启 sing-box；下载资产、SHA-256 校验与失败回滚均由服务器端完成。</p>
    </section>

    <section class="panel-card">
      <div class="settings-heading">
        <div class="protocol-icon mint"><CloudCog :size="22" /></div>
        <div><h3>RackNerd 官方流量</h3><p>只读调用 info 接口，不执行关机或重启。</p></div>
        <span v-if="state.racknerd?.configured" class="status-pill ok"><CheckCircle2 :size="14" />已配置</span>
      </div>
      <form @submit.prevent="emit('save-racknerd', { apiKey, apiHash })">
        <label>API Key<input v-model="apiKey" type="password" autocomplete="off" :placeholder="state.racknerd?.configured ? '已保存，输入新 Key 可替换' : '输入每台 VPS 独立的 API Key'" /></label>
        <label>API Hash<input v-model="apiHash" type="password" autocomplete="off" :placeholder="state.racknerd?.configured ? '已保存，输入新 Hash 可替换' : '输入 API Hash'" /></label>
        <p class="field-help">凭据仅保存在 VPS 的 600 权限配置中，不会通过状态 API 回显。留空的字段会保留原值。</p>
        <div class="button-row">
          <button class="primary-button" type="submit" :disabled="busy.racknerd"><KeyRound :size="16" />{{ busy.racknerd ? '正在验证…' : '保存并获取流量' }}</button>
          <button v-if="state.racknerd?.configured" class="secondary-button danger-button" type="button" :disabled="busy.racknerd" @click="emit('clear-racknerd')"><Trash2 :size="16" />清除</button>
        </div>
      </form>
      <p v-if="state.racknerd?.error" class="warning-text">{{ state.racknerd.error }}</p>
    </section>

    <section class="panel-card">
      <div class="settings-heading">
        <div class="protocol-icon lavender"><Zap :size="22" /></div>
        <div><h3>BBR 网络优化</h3><p>应用项目内置的 BBR + fq 保守调优模板。</p></div>
        <span class="status-pill" :class="state.status.bbr_enabled ? 'ok' : 'warn'">{{ state.status.bbr_enabled ? '正常' : '待处理' }}</span>
      </div>
      <div class="setting-readout">
        <div><span>拥塞控制</span><strong>{{ state.status.congestion_control }}</strong></div>
        <div><span>队列规则</span><strong>{{ state.status.default_qdisc }}</strong></div>
      </div>
      <p class="field-help">仅应用固定模板，不执行来自浏览器的任意系统命令。Hysteria2 不受 BBR 直接加速。</p>
      <button class="secondary-button" :disabled="busy.bbr" @click="emit('apply-bbr')"><RotateCcw :size="16" />{{ busy.bbr ? '正在应用…' : '应用 BBR + fq' }}</button>
    </section>

    <section class="panel-card update-card">
      <div class="settings-heading">
        <div class="protocol-icon mint"><Download :size="22" /></div>
        <div><h3>面板更新</h3><p>从 LinyuFiee/hy2panel 官方 Release 检测并校验更新。</p></div>
        <span v-if="state.update?.available" class="status-pill ok">发现新版本</span>
      </div>
      <div class="setting-readout">
        <div><span>当前版本</span><strong>v{{ state.update?.current_version || '未知' }}</strong></div>
        <div><span>最新版本</span><strong>{{ state.update?.latest_version ? `v${state.update.latest_version}` : '尚未检测' }}</strong></div>
      </div>
      <div class="button-row">
        <button class="secondary-button" :disabled="state.update?.checking || state.update?.updating" @click="emit('check-update')"><RefreshCw :size="16" :class="{ spin: state.update?.checking }" />{{ state.update?.checking ? '正在检测…' : '检测更新' }}</button>
        <button v-if="state.update?.available" class="primary-button" :disabled="state.update?.updating" @click="emit('apply-update')"><Download :size="16" />{{ state.update?.updating ? '正在更新…' : '立即更新' }}</button>
        <a class="secondary-button link-button" href="https://github.com/LinyuFiee/hy2panel" target="_blank" rel="noopener noreferrer">GitHub <ExternalLink :size="14" /></a>
        <a v-if="state.update?.release_url" class="secondary-button link-button" :href="state.update.release_url" target="_blank" rel="noopener noreferrer">Release <ExternalLink :size="14" /></a>
      </div>
      <p v-if="state.update?.message" class="field-help update-message">{{ state.update.message }}</p>
      <p v-if="state.update?.error" class="warning-text">{{ state.update.error }}</p>
      <p class="field-help">升级包强制校验 SHA-256，失败自动回滚。更新时面板会短暂重启，不会覆盖节点配置。</p>
    </section>

    <section class="panel-card security-card">
      <div class="settings-heading">
        <div class="protocol-icon peach"><LockKeyhole :size="22" /></div>
        <div><h3>访问安全</h3><p>当前安装的安全边界。</p></div>
      </div>
      <ul class="security-list">
        <li><ShieldCheck :size="16" /><span>公网入口<strong>仅通过 HTTPS 访问</strong></span></li>
        <li><ShieldCheck :size="16" /><span>登录防护<strong>按来源限制登录尝试速率</strong></span></li>
        <li><ShieldCheck :size="16" /><span>安全会话<strong>HttpOnly · SameSite Strict</strong></span></li>
        <li><Database :size="16" /><span>敏感配置<strong>仅 root / hy2panel 可读取</strong></span></li>
      </ul>
    </section>
  </div>
</template>
