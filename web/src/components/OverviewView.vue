<script setup>
import { computed, ref } from 'vue'
import { useDocumentVisibility, useElementVisibility, usePreferredReducedMotion } from '@vueuse/core'
import { Clock3, Cpu, HardDrive, MemoryStick, Monitor, Pause, Play, Server, Stethoscope } from '@lucide/vue'
import { Button } from './ui/button'
import OverviewSubscription from './OverviewSubscription.vue'
import { formatBytes, formatTimestamp, formatUptime } from '../utils'

const props = defineProps({
  config: { type: Object, required: true },
  status: { type: Object, default: () => ({}) },
  report: { type: Object, default: null },
  diagnosing: Boolean,
  reportInvalidated: Boolean,
  subscriptionStatus: { type: Function, required: true },
  manageSubscription: { type: Function, required: true },
  session: { type: Object, required: true },
  copied: Boolean
})
const emit = defineEmits(['diagnostics', 'copied', 'revoked'])

function endpoint(host, port) {
  if (!host || !port) return '未设置接入地址'
  return `${host.includes(':') && !host.startsWith('[') ? `[${host}]` : host}:${port}`
}
const protocols = computed(() => [
  { name: 'Reality', transport: 'TCP', enabled: props.config.reality?.enabled, address: endpoint(props.config.panel?.public_host, props.config.reality?.port) },
  { name: 'Hysteria2', transport: 'UDP', enabled: props.config.hysteria2?.enabled, address: endpoint(props.config.hysteria2?.domain, props.config.hysteria2?.port) }
])
const protocolList = ref(null)
const paused = ref(false)
const visible = useElementVisibility(protocolList)
const pageVisibility = useDocumentVisibility()
const motionPreference = usePreferredReducedMotion()
const canAnimate = computed(() => protocols.value.some(protocol => protocol.enabled) && motionPreference.value !== 'reduce')
const playing = computed(() => canAnimate.value && !paused.value && visible.value && pageVisibility.value === 'visible')
const tracks = [
  { direction: 'outgoing', path: 'M22 12h5c6 0 6-5 12-5h66c6 0 6 5 12 5h5' },
  { direction: 'incoming', path: 'M122 12h-5c-6 0-6 5-12 5H39c-6 0-6-5-12-5h-5' }
]
const checkNames = { config: 'sing-box 配置', service: '核心服务', 'hy2-listen': 'Hysteria2 监听', certificate: '节点证书', dns: '域名解析' }
const statusLabels = { pass: '通过', warn: '提醒', fail: '失败', info: '提示' }
const checks = computed(() => Object.entries(checkNames).map(([id, name]) =>
  props.report?.checks?.find(check => check.id === id) ?? { id, name, message: '本次自检未返回此项。' }
))
function usage(used, total, multiplier = 1) {
  if (used == null || !(Number(total) > 0)) return '—'
  return `${formatBytes(Number(used) * multiplier)} / ${formatBytes(Number(total) * multiplier)}`
}
function share(used, total) {
  if (used == null || !Number.isFinite(Number(used)) || !(Number(total) > 0)) return null
  return Math.max(0, Math.min(100, Number(used) / Number(total) * 100))
}
const resources = computed(() => {
  const s = props.status
  const cpu = Number.isFinite(s.cpu_percent) ? s.cpu_percent : null
  return [
    { name: 'CPU 使用率', icon: Cpu, value: cpu === null ? '—' : `${cpu.toFixed(1)}%`, percent: cpu === null ? null : share(cpu, 100) },
    { name: '内存', icon: MemoryStick, value: usage(s.memory_used_mb, s.memory_total_mb, 1024 * 1024), percent: share(s.memory_used_mb, s.memory_total_mb) },
    { name: '磁盘', icon: HardDrive, value: usage(s.disk_used_bytes, s.disk_total_bytes), percent: share(s.disk_used_bytes, s.disk_total_bytes) }
  ]
})
</script>

<template>
  <div class="overview">
    <div class="connection-column">
      <section class="connection-sheet" aria-labelledby="connect-heading">
        <div class="access">
          <h2 id="connect-heading">连接到节点</h2>
          <OverviewSubscription
            :subscription-status="subscriptionStatus" :manage-subscription="manageSubscription" :session="session"
            :has-protocol="protocols.some(protocol => protocol.enabled)" :copied="copied"
            @copied="emit('copied')" @revoked="emit('revoked')"
          >
            <Button type="button" variant="outline" :disabled="diagnosing || !config.hysteria2?.enabled" @click="emit('diagnostics')">
              <Stethoscope aria-hidden="true" />{{ diagnosing ? '正在诊断…' : '诊断 HY2' }}
            </Button>
          </OverviewSubscription>
        </div>
        <div class="protocols">
          <header class="protocol-head">
            <h3 id="protocol-heading">节点协议</h3>
            <div class="flow-key">
              <span>设备连接未测试</span>
              <button v-if="canAnimate" type="button" class="flow-toggle" :aria-label="paused ? '播放传输示意' : '暂停传输示意'" :title="paused ? '播放传输示意（非实时流量）' : '暂停传输示意（非实时流量）'" @click="paused = !paused">
                <component :is="paused ? Play : Pause" :size="13" aria-hidden="true" />传输示意
              </button>
              <span v-else class="flow-label">传输示意</span>
            </div>
          </header>
          <ul ref="protocolList" class="protocol-list" :class="{ 'is-playing': playing }" aria-labelledby="protocol-heading">
            <li v-for="protocol in protocols" :key="protocol.name" :class="{ inactive: !protocol.enabled }">
              <div class="protocol-id"><strong>{{ protocol.name }}</strong><span class="transport">{{ protocol.transport }}</span><span class="endpoint">{{ protocol.address }}</span></div>
              <svg class="packet-flow" :class="{ 'is-disabled': !protocol.enabled }" viewBox="0 0 144 24" aria-hidden="true" focusable="false">
                <Monitor x="1" y="3" :size="18" :stroke-width="1.5" />
                <Server x="125" y="3" :size="18" :stroke-width="1.5" />
                <g v-for="track in tracks" :key="track.direction">
                  <path class="wire" :d="track.path" />
                  <path v-if="protocol.enabled" class="packet" :class="track.direction" :d="track.path" pathLength="1" />
                </g>
              </svg>
              <span class="config-state">{{ protocol.enabled ? '已配置' : '已停用' }}</span>
            </li>
          </ul>
        </div>
      </section>

      <span class="sr-only" role="status">{{ diagnosing ? '正在检查服务器' : report && config.hysteria2?.enabled ? 'HY2 服务端检查已完成，结果见下方。' : '' }}</span>
      <p v-if="reportInvalidated && config.hysteria2?.enabled" class="diagnostic-note" role="status">节点配置已更改，请重新诊断。</p>
      <section v-if="report && config.hysteria2?.enabled" class="report" aria-labelledby="report-heading">
        <header class="report-head">
          <h2 id="report-heading">HY2 服务端检查</h2>
          <time :datetime="report.checked_at">检查时间 {{ formatTimestamp(report.checked_at, true) }}</time>
        </header>
        <p>仅检查服务器。请在设备上更新订阅，选择 Hysteria2 后测试连接。</p>
        <ul class="checks">
          <li v-for="check in checks" :key="check.id">
            <strong>{{ check.name }}</strong>
            <div class="check-detail">{{ check.message }}<p v-if="check.suggestion">建议：{{ check.suggestion }}</p></div>
            <span :class="['check-result', check.status]">{{ statusLabels[check.status] || '未检查' }}</span>
          </li>
        </ul>
      </section>

    </div>
    <section class="resources" aria-labelledby="resource-heading">
      <h2 id="resource-heading">主机资源</h2>
      <dl class="readings">
        <div v-for="metric in resources" :key="metric.name" class="reading">
          <dt><component :is="metric.icon" :size="15" aria-hidden="true" />{{ metric.name }}</dt>
          <dd>
            {{ metric.value }}
            <span v-if="metric.percent !== null" class="gauge" aria-hidden="true"><i :style="{ width: `${metric.percent}%` }"></i></span>
          </dd>
        </div>
        <div class="reading uptime"><dt><Clock3 :size="15" aria-hidden="true" />运行时间</dt><dd>{{ status.uptime_seconds != null ? formatUptime(status.uptime_seconds) : '—' }}</dd></div>
      </dl>
    </section>
  </div>
</template>

<style scoped>
.overview { display: grid; grid-template-columns: minmax(0, 2.35fr) minmax(14rem, 1fr); align-items: start; gap: clamp(1.25rem, 3vw, 3rem); min-width: 0; font-variant-numeric: tabular-nums; }
.connection-column { display: grid; gap: 1.5rem; min-width: 0; }
.connection-sheet { overflow: hidden; border: 1px solid var(--border); border-radius: var(--radius); background: var(--card); }
.access, .protocols { padding: 1.5rem; }
.access { background: color-mix(in srgb, var(--card) 64%, var(--muted)); border-bottom: 1px solid var(--border); }
.protocols { padding-block: 1.1rem .5rem; }
h2, h3 { margin: 0; font-size: 1rem; font-weight: 630; line-height: 1.4; }
.access h2 { font-size: 1.5rem; letter-spacing: -.03em; }
.protocol-head, .report-head { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: .3rem 1rem; }
.flow-key, time, .report > p, .diagnostic-note, .check-detail p, dt { color: var(--muted-foreground); font-size: .8125rem; }
.flow-key, .flow-toggle { display: inline-flex; align-items: center; gap: .4rem; }
.flow-key { gap: .65rem; }
.flow-toggle, .flow-label { border-inline-start: 1px solid var(--border); padding-inline-start: .65rem; }
.flow-toggle { color: inherit; font: inherit; min-height: 1.75rem; }
.flow-toggle:hover, .flow-toggle:focus-visible { color: var(--foreground); }
ul { list-style: none; padding: 0; margin: 0; }
.protocol-list { margin: .65rem 0 0; }
.checks li { display: grid; grid-template-columns: minmax(6.5rem, .36fr) minmax(0, 1fr) auto; gap: .4rem 1rem; align-items: baseline; }
li + li { border-top: 1px solid var(--border); }
.protocol-list li { display: grid; grid-template-columns: minmax(0, 1fr) minmax(5rem, 9rem) auto; gap: .75rem; align-items: center; padding-block: 1.05rem; }
.protocol-id { min-width: 0; display: flex; flex-wrap: wrap; align-items: baseline; gap: .18rem .55rem; }
.protocol-id .endpoint { flex-basis: 100%; }
.inactive .protocol-id { color: var(--muted-foreground); }
.packet-flow { display: block; width: 100%; color: var(--muted-foreground); --packet-color: var(--chart-1); --phase: -.65s; }
.protocol-list li:nth-child(2) .packet-flow { --packet-color: var(--chart-2); --phase: -1.55s; }
.packet-flow.is-disabled { opacity: .45; }
.packet-flow path.wire, .packet-flow path.packet { fill: none; stroke-linecap: round; }
.wire { stroke: currentColor; stroke-width: 1; opacity: .4; }
.packet { stroke: var(--packet-color); stroke-width: 2.8; stroke-dasharray: .068 1.02; stroke-dashoffset: -.32; animation: transfer 2.8s linear var(--phase) infinite paused; }
.packet.incoming { stroke-width: 2; stroke-dashoffset: -.64; animation-delay: calc(var(--phase) - 1.1s); }
.is-playing .packet { animation-play-state: running; }
@keyframes transfer { from { stroke-dashoffset: .068; } to { stroke-dashoffset: -1.02; } }
@media (prefers-reduced-motion: reduce) { .packet-flow .packet { animation: none !important; } }
.protocol-list strong { font-size: 1rem; font-weight: 620; }
.endpoint { font-family: ui-monospace, SFMono-Regular, Consolas, monospace; font-size: .8125rem; color: var(--muted-foreground); overflow-wrap: anywhere; }
.transport { color: var(--muted-foreground); font-size: .75rem; }
.config-state, .check-result { font-size: .8125rem; color: var(--muted-foreground); white-space: nowrap; }
.report > p { margin: .35rem 0 1rem; }
.checks, .resources dl { border-block: 1px solid var(--border); }
.checks li { padding-block: .7rem; font-size: .875rem; }
.checks strong { font-weight: 550; }
.check-detail { min-width: 0; overflow-wrap: anywhere; }
.check-detail p { margin: .2rem 0 0; }
.check-result.warn { color: var(--warning); }
.check-result.fail { color: var(--destructive); }
.diagnostic-note { margin: -1rem 0 0; }
.resources { min-width: 0; padding: 1.5rem 0 0; }
.readings { margin: .85rem 0 0; }
.reading { padding: .9rem 0; }
.reading + .reading { border-top: 1px solid var(--border); }
.reading dt { display: flex; align-items: center; gap: .55rem; }
.reading dd { margin: .3rem 0 0; font-size: 1.125rem; line-height: 1.4; font-weight: 560; overflow-wrap: anywhere; }
.gauge { display: block; height: 3px; background: var(--secondary); margin-top: .65rem; border-radius: 2px; overflow: hidden; }
.gauge i { display: block; height: 100%; background: var(--muted-foreground); }
.uptime { display: flex; align-items: baseline; justify-content: space-between; flex-wrap: wrap; gap: .35rem .65rem; }
.uptime dd { margin: 0; font-size: .875rem; }
@media (max-width: 1080px) {
  .overview { grid-template-columns: minmax(0, 1fr); gap: 1.75rem; }
  .resources { padding: 0; }
  .readings { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 1.25rem; padding-block: .4rem; }
  .reading + .reading { border: 0; }
  .uptime { display: block; }
  .uptime dd { margin-top: .3rem; }
}
@media (max-width: 680px) {
  .access, .protocols { padding-inline: 1rem; }
  .access { padding-block: 1.15rem; }
  .protocol-list li { grid-template-columns: minmax(0,1fr) auto; gap: .125rem .6rem; padding-block: .85rem; }
  .protocol-id { grid-row: 1 / 3; }
  .config-state { grid-column: 2; grid-row: 1; text-align: right; }
  .packet-flow { grid-column: 2; grid-row: 2; width: 5rem; }
  .checks li { grid-template-columns: minmax(0,1fr) auto; gap: .25rem .6rem; }
  .check-result { grid-row: 1; grid-column: 2; }
  .check-detail { grid-column: 1 / -1; }
  .readings { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 1.5rem; }
  .reading:nth-child(n+3) { border-top: 1px solid var(--border); }
  .flow-toggle { min-height: 2.75rem; }
}
</style>
