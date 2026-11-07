<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import { Activity, AlertCircle, CheckCircle2, ChevronRight, CircleAlert, ClipboardCheck, FileClock, Info, LoaderCircle, RefreshCw, RotateCcw, ScrollText, ShieldAlert, ShieldCheck, Wrench } from '@lucide/vue'

const props = defineProps({
  state: { type: Object, required: true },
  report: { type: Object, default: null },
  entries: { type: Array, default: () => [] },
  logUnit: { type: String, default: 'sing-box' },
  busy: { type: Object, required: true }
})
const emit = defineEmits(['diagnostics', 'logs', 'recovery'])
const logBox = ref(null)
const summary = computed(() => props.report ? `${props.report.fail} 项异常 · ${props.report.warn} 项提醒 · ${props.report.pass} 项正常` : '尚未运行自检')
const lastPortChange = computed(() => {
  const timestamp = props.state.config.recovery?.last_port_change || 0
  return timestamp > 0 ? new Date(timestamp * 1000).toLocaleString() : '尚未自动切换'
})
const statusIcons = { pass: CheckCircle2, warn: CircleAlert, fail: AlertCircle, info: Info }
const statusText = { pass: '正常', warn: '提醒', fail: '异常', info: '说明' }

watch(() => props.entries, async () => {
  await nextTick()
  if (logBox.value) logBox.value.scrollTop = logBox.value.scrollHeight
})
</script>

<template>
  <div class="view-stack">
    <section class="maintenance-grid">
      <article class="panel-card recovery-card">
        <div class="panel-heading">
          <div><span class="section-kicker">AUTOMATIC RECOVERY</span><h3>故障自动恢复</h3></div>
          <RotateCcw :size="20" class="muted-icon" />
        </div>
        <div class="recovery-row">
          <div class="recovery-icon"><ShieldCheck :size="20" /></div>
          <div><strong>进程崩溃自动重启</strong><p>systemd 即时重启，看门狗每分钟复查实际监听。</p></div>
          <span class="status-pill ok">始终启用</span>
        </div>
        <div class="recovery-row">
          <div class="recovery-icon lavender"><ShieldAlert :size="20" /></div>
          <div><strong>连续异常后自动换端口</strong><p>连续 3 次监听失败才触发，24 小时最多切换一次。</p></div>
          <label class="toggle" title="开启自动换端口"><input type="checkbox" :checked="state.config.recovery?.auto_port || false" :disabled="busy.recovery" @change="emit('recovery', $event.target.checked)" /><span></span></label>
        </div>
        <div class="recovery-note">
          <Info :size="15" />
          <p>只能可靠发现本机服务和监听异常。运营商或 GFW 封堵时，本机可能仍显示监听正常，因此需要客户端或外部探测复核。自动换端口后，必须重新导入客户端链接。</p>
        </div>
        <small>上次自动切换：{{ lastPortChange }}</small>
      </article>

      <article class="panel-card diagnostic-card">
        <div class="panel-heading">
          <div><span class="section-kicker">GUIDED DIAGNOSTICS</span><h3>服务端问题自检</h3></div>
          <ClipboardCheck :size="20" class="muted-icon" />
        </div>
        <div class="diagnostic-summary">
          <div class="diagnostic-icon"><Wrench :size="24" /></div>
          <div><strong>{{ summary }}</strong><p>检查配置、服务、监听、证书、DNS、BBR 和资源。</p></div>
        </div>
        <button class="primary-button full-button" :disabled="busy.diagnostics" @click="emit('diagnostics')">
          <LoaderCircle v-if="busy.diagnostics" :size="16" class="spin" /><Activity v-else :size="16" />{{ busy.diagnostics ? '正在逐项检查…' : '开始自检' }}
        </button>
      </article>
    </section>

    <section v-if="report" class="panel-card">
      <div class="panel-heading">
        <div><span class="section-kicker">DIAGNOSTIC REPORT</span><h3>检查结果与修复建议</h3></div>
        <span class="soft-pill">{{ new Date(report.checked_at).toLocaleTimeString() }}</span>
      </div>
      <div class="diagnostic-list">
        <article v-for="check in report.checks" :key="check.id" class="diagnostic-item" :class="check.status">
          <component :is="statusIcons[check.status] || Info" :size="19" />
          <div class="diagnostic-content">
            <div><strong>{{ check.name }}</strong><span class="diagnostic-badge" :class="check.status">{{ statusText[check.status] }}</span></div>
            <p>{{ check.message }}</p>
            <small v-if="check.suggestion"><ChevronRight :size="13" />{{ check.suggestion }}</small>
          </div>
        </article>
      </div>
    </section>

    <section class="panel-card log-card">
      <div class="panel-heading log-heading">
        <div><span class="section-kicker">REDACTED JOURNAL</span><h3>运行日志</h3></div>
        <ScrollText :size="20" class="muted-icon" />
      </div>
      <div class="log-toolbar">
        <div class="log-tabs">
          <button :class="{ active: logUnit === 'sing-box' }" @click="emit('logs', 'sing-box')">sing-box</button>
          <button :class="{ active: logUnit === 'panel' }" @click="emit('logs', 'panel')">面板</button>
          <button :class="{ active: logUnit === 'watchdog' }" @click="emit('logs', 'watchdog')">看门狗</button>
          <button :class="{ active: logUnit === 'apply' }" @click="emit('logs', 'apply')">配置应用</button>
        </div>
        <button class="secondary-button small-button" :disabled="busy.logs" @click="emit('logs', logUnit)"><RefreshCw :size="14" :class="{ spin: busy.logs }" />刷新</button>
      </div>
      <div ref="logBox" class="log-viewer">
        <div v-if="busy.logs && !entries.length" class="log-empty">正在读取日志…</div>
        <div v-else-if="!entries.length" class="log-empty">当前单元暂无日志。</div>
        <div v-for="(entry, index) in entries" v-else :key="`${logUnit}-${index}`" class="log-line">{{ entry }}</div>
      </div>
      <p class="security-note">仅允许固定 systemd 单元，最多返回 300 行；节点和 API 凭据会在返回前脱敏。</p>
    </section>
  </div>
</template>