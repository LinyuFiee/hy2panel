<script setup>
import { computed } from 'vue'
import {
  Activity,
  ArrowDownToLine,
  ArrowUpFromLine,
  CircleHelp,
  Database,
  Gauge,
  GitBranch,
  Info,
  LoaderCircle,
  Play,
  Settings2,
  Zap
} from '@lucide/vue'
import NetworkChart from './NetworkChart.vue'
import ProtocolTrafficChart from './ProtocolTrafficChart.vue'
import { Alert, AlertDescription, AlertTitle } from './ui/alert'
import { Badge } from './ui/badge'
import { Button } from './ui/button'
import { Progress } from './ui/progress'
import { formatBytes, formatCount, formatMbps, formatRate, formatTimestamp, sumBytes } from '../utils'

const props = defineProps({
  state: { type: Object, required: true },
  samples: { type: Array, default: () => [] },
  starting: { type: Boolean, default: false },
  traffic: { type: Object, default: null },
  history: { type: Object, default: null },
  historyRange: { type: String, default: '1h' },
  historyLoading: { type: Boolean, default: false }
})

const emit = defineEmits(['speedtest', 'settings', 'history-range'])

const protocolDefinitions = [
  { key: 'reality', name: 'Reality', transport: 'TCP · VLESS' },
  { key: 'hysteria2', name: 'Hysteria2', transport: 'UDP · QUIC' }
]
const observedProtocols = ['reality', 'hysteria2', 'unattributed']

const status = computed(() => props.state.status || {})
const racknerd = computed(() => props.state.racknerd || { configured: false })
const speedtest = computed(() => props.state.speedtest || { running: false })

function clampPercent(value) {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return 0
  return Math.min(100, Math.max(0, numeric))
}

function protocolTraffic(key) {
  return props.traffic?.traffic?.[key] || { upload: 0, download: 0 }
}

function protocolRates(key) {
  return props.traffic?.rates?.[key] || { upload: 0, download: 0 }
}

function protocolTotal(key) {
  const value = protocolTraffic(key)
  return sumBytes(value.upload, value.download)
}

function protocolRateTotal(key) {
  const value = protocolRates(key)
  return sumBytes(value.upload, value.download)
}

function observedDirection(direction) {
  const reported = props.traffic?.total?.[direction]
  if (reported !== undefined && reported !== null) return reported
  return sumBytes(...observedProtocols.map(key => protocolTraffic(key)[direction]))
}

const coveragePercent = computed(() => clampPercent(Number(props.traffic?.attribution_coverage || 0) * 100))
const unattributedPercent = computed(() => Math.max(0, 100 - coveragePercent.value))
const activeFlowTotal = computed(() => sumBytes(...observedProtocols.map(key => props.traffic?.active_flows?.[key] || 0)))
const liveDownloadTotal = computed(() => sumBytes(...observedProtocols.map(key => protocolRates(key).download)))
const liveUploadTotal = computed(() => sumBytes(...observedProtocols.map(key => protocolRates(key).upload)))
const observedTotal = computed(() => sumBytes(observedDirection('download'), observedDirection('upload')))
const hostLiveTotal = computed(() => sumBytes(status.value.rx_bytes_per_second, status.value.tx_bytes_per_second))
const hostObservedTotal = computed(() => sumBytes(status.value.rx_total_bytes, status.value.tx_total_bytes))
const racknerdPercent = computed(() => clampPercent(racknerd.value.info?.percent_used || 0))

const trafficState = computed(() => {
  if (!props.traffic?.available) return { label: '协议采集不可用', variant: 'destructive', state: 'offline' }
  if (props.traffic.stale) return { label: '协议数据已过期', variant: 'warning', state: 'stale' }
  return { label: '协议采集中', variant: 'success', state: 'live' }
})

const trafficNotice = computed(() => {
  if (!props.traffic?.available) return props.traffic?.error || '协议监控暂不可用，主机网络采样仍会继续。'
  if (!props.traffic.stale) return ''
  const lastSuccess = props.traffic.last_success
    ? `最后成功采样于 ${formatTimestamp(props.traffic.last_success, true)}。`
    : '尚无可用的成功采样时间。'
  return `${lastSuccess}${props.traffic.error ? ` ${props.traffic.error}` : ' 协议数值会保留，但不会伪造缺失数据。'}`
})

const speedButtonLabel = computed(() => {
  if (props.starting && !speedtest.value.running) return '正在启动…'
  if (speedtest.value.running) return '测速进行中…'
  return '开始测速'
})
</script>

<template>
  <div class="traffic">
    <section class="live-section" aria-labelledby="protocol-live-heading">
      <div class="topline">
        <h2 id="protocol-live-heading">协议实时</h2>
        <div class="topline-right" aria-live="polite" aria-atomic="true">
          <Badge :variant="trafficState.variant" class="live-badge">
            <span class="dot" :class="trafficState.state" aria-hidden="true"></span>
            {{ trafficState.label }}
          </Badge>
          <time v-if="traffic?.sampled_at" class="sampled tnum" :datetime="traffic.sampled_at">
            采样 {{ formatTimestamp(traffic.sampled_at) }}
          </time>
          <span v-else class="sampled tnum">等待首次采样</span>
        </div>
      </div>

      <Alert v-if="trafficNotice" :variant="traffic?.stale ? 'warning' : 'destructive'" class="traffic-alert">
        <CircleHelp />
        <AlertTitle>{{ traffic?.stale ? '采样已中断' : '无法读取协议流量' }}</AlertTitle>
        <AlertDescription>{{ trafficNotice }}</AlertDescription>
      </Alert>

      <section class="stat-grid" aria-label="协议流量关键指标">
        <article class="stat">
          <div class="stat-head"><Activity :size="15" aria-hidden="true" /><span>当前代理速率</span></div>
          <strong class="stat-value">{{ formatRate(sumBytes(liveDownloadTotal, liveUploadTotal)) }}</strong>
          <div class="stat-pair" aria-label="当前代理上下行速率">
            <span><ArrowDownToLine :size="12" aria-hidden="true" />{{ formatRate(liveDownloadTotal) }}</span>
            <span><ArrowUpFromLine :size="12" aria-hidden="true" />{{ formatRate(liveUploadTotal) }}</span>
          </div>
        </article>

        <article class="stat">
          <div class="stat-head"><GitBranch :size="15" aria-hidden="true" /><span>活动代理流</span></div>
          <strong class="stat-value">{{ formatCount(activeFlowTotal) }}</strong>
          <div class="stat-pair">
            <span>Reality {{ formatCount(traffic?.active_flows?.reality || 0) }}</span>
            <span>Hysteria2 {{ formatCount(traffic?.active_flows?.hysteria2 || 0) }}</span>
          </div>
        </article>

        <article class="stat">
          <div class="stat-head"><Database :size="15" aria-hidden="true" /><span>代理观察累计</span></div>
          <strong class="stat-value">{{ formatBytes(observedTotal) }}</strong>
          <div class="stat-pair">
            <span>下行 {{ formatBytes(observedDirection('download')) }}</span>
            <span>上行 {{ formatBytes(observedDirection('upload')) }}</span>
          </div>
        </article>

        <article class="stat">
          <div class="stat-head"><Gauge :size="15" aria-hidden="true" /><span>归属覆盖率</span></div>
          <strong class="stat-value">{{ coveragePercent.toFixed(1) }}%</strong>
          <div class="stat-pair">
            <span>未归属 {{ unattributedPercent.toFixed(1) }}%</span>
            <span>{{ formatCount(traffic?.active_flows?.unattributed || 0) }} 条活动流</span>
          </div>
        </article>
      </section>

      <section class="proto-grid" aria-label="Reality 与 Hysteria2 协议流量">
      <article
        v-for="protocol in protocolDefinitions"
        :key="protocol.key"
        class="card"
        :class="protocol.key"
        :data-testid="`${protocol.key}-traffic-card`"
      >
        <header class="card-head">
          <div class="card-title">
            <h3>{{ protocol.name }}</h3>
            <small>{{ protocol.transport }}</small>
          </div>
          <Badge variant="outline" class="flow-badge">
            <Activity :size="12" aria-hidden="true" />
            {{ formatCount(traffic?.active_flows?.[protocol.key] || 0) }} 活动流
          </Badge>
        </header>
        <div class="card-body">
          <div class="rate-grid">
            <div>
              <span><ArrowDownToLine :size="13" aria-hidden="true" />实时下行</span>
              <strong>{{ formatRate(protocolRates(protocol.key).download) }}</strong>
            </div>
            <div>
              <span><ArrowUpFromLine :size="13" aria-hidden="true" />实时上行</span>
              <strong>{{ formatRate(protocolRates(protocol.key).upload) }}</strong>
            </div>
          </div>
          <div class="cumulative">
            <div class="cumulative-main">
              <span>累计代理载荷</span>
              <strong>{{ formatBytes(protocolTotal(protocol.key)) }}</strong>
            </div>
            <dl>
              <div><dt>下行</dt><dd>{{ formatBytes(protocolTraffic(protocol.key).download) }}</dd></div>
              <div><dt>上行</dt><dd>{{ formatBytes(protocolTraffic(protocol.key).upload) }}</dd></div>
              <div><dt>当前合计</dt><dd>{{ formatRate(protocolRateTotal(protocol.key)) }}</dd></div>
            </dl>
          </div>
        </div>
      </article>
    </section>
    </section>

    <article class="card" data-testid="unattributed-traffic">
      <header class="card-head">
        <div class="card-title">
          <h3>未归属流量</h3>
          <small>不能安全映射到指定入站的代理流量独立保留，不回填到 Reality 或 Hysteria2。</small>
        </div>
        <Badge variant="secondary">{{ unattributedPercent.toFixed(1) }}% 未归属</Badge>
      </header>
      <div class="card-body unattributed-body">
        <div class="coverage">
          <div class="coverage-line">
            <span>协议归属覆盖率</span>
            <strong>{{ coveragePercent.toFixed(1) }}%</strong>
          </div>
          <Progress :model-value="coveragePercent" :max="100" class="coverage-progress" aria-label="协议归属覆盖率" />
        </div>
        <dl class="mini-stats">
          <div><dt>累计</dt><dd>{{ formatBytes(protocolTotal('unattributed')) }}</dd></div>
          <div><dt>当前速率</dt><dd>{{ formatRate(protocolRateTotal('unattributed')) }}</dd></div>
          <div><dt>活动代理流</dt><dd>{{ formatCount(traffic?.active_flows?.unattributed || 0) }}</dd></div>
        </dl>
      </div>
    </article>

    <article class="card">
      <header class="card-head">
        <div class="card-title">
          <h3>协议流量趋势</h3>
          <small>按分钟归档；采样断线以缺口显示，不插值补齐。</small>
        </div>
        <Badge variant="outline">每分钟归档</Badge>
      </header>
      <div class="card-body chart-body">
        <ProtocolTrafficChart
          :history="history"
          :range="historyRange"
          :loading="historyLoading"
          @range="emit('history-range', $event)"
        />
      </div>
    </article>

    <div class="infra-grid">
      <article class="card">
        <header class="card-head">
          <div class="card-title">
            <h3>主机网络</h3>
            <small>网卡吞吐包含代理、面板、SSH、测速与同机其他进程。</small>
          </div>
          <Badge variant="success"><span class="dot live" aria-hidden="true"></span>实时</Badge>
        </header>
        <div class="card-body chart-body host-chart">
          <NetworkChart :samples="samples" />
        </div>
        <footer class="foot-3">
          <div><span>当前网卡吞吐</span><strong>{{ formatRate(hostLiveTotal) }}</strong></div>
          <div><span>本次启动累计</span><strong>{{ formatBytes(hostObservedTotal) }}</strong></div>
          <div><span>接收 / 发送</span><strong>{{ formatBytes(status.rx_total_bytes) }} / {{ formatBytes(status.tx_total_bytes) }}</strong></div>
        </footer>
      </article>

      <div class="service-stack">
        <article class="card">
          <header class="card-head">
            <div class="card-title">
              <h3>本月套餐流量</h3>
              <small>RackNerd 官方只读接口</small>
            </div>
            <Badge v-if="racknerd.info" :variant="racknerd.error ? 'warning' : 'success'">
              {{ racknerd.error ? '缓存数据' : '官方数据' }}
            </Badge>
            <Badge v-else variant="secondary">{{ racknerd.configured ? '等待数据' : '未连接' }}</Badge>
          </header>
          <div class="card-body service-body">
            <template v-if="racknerd.configured && racknerd.info">
              <div class="quota-value">
                <strong>{{ formatBytes(racknerd.info.bandwidth_used) }}</strong>
                <span>/ {{ formatBytes(racknerd.info.bandwidth_total) }}</span>
              </div>
              <div class="quota-label">
                <span>已使用 {{ racknerdPercent.toFixed(2) }}%</span>
                <span>剩余 {{ formatBytes(racknerd.info.bandwidth_free) }}</span>
              </div>
              <Progress :model-value="racknerdPercent" :max="100" aria-label="RackNerd 本月套餐流量使用率" />
              <p v-if="racknerd.error" class="service-warning" role="status">显示最近一次成功缓存：{{ racknerd.error }}</p>
            </template>

            <div v-else class="service-empty">
              <Database :size="24" aria-hidden="true" />
              <p>{{ racknerd.configured ? racknerd.error || '正在读取 RackNerd 套餐流量。' : '连接只读 API 后显示供应商套餐用量。' }}</p>
              <Button variant="outline" size="sm" @click="emit('settings')">
                <Settings2 :size="14" aria-hidden="true" />{{ racknerd.configured ? '检查配置' : '连接 RackNerd' }}
              </Button>
            </div>
          </div>
        </article>

        <article class="card">
          <header class="card-head">
            <div class="card-title">
              <h3>VPS 到测速站</h3>
              <small>测量服务器出口，不代表客户端经 HY2 或 Reality 的速度。</small>
            </div>
            <Badge v-if="speedtest.running || starting" variant="warning">运行中</Badge>
            <Badge v-else-if="speedtest.error" variant="destructive">最近一次失败</Badge>
            <Badge v-else-if="speedtest.result" variant="success">已完成</Badge>
            <Badge v-else variant="secondary">未测试</Badge>
          </header>
          <div class="card-body service-body">
            <div v-if="speedtest.result && !speedtest.running && !starting" class="speed-readout">
              <div>
                <span><ArrowDownToLine :size="13" aria-hidden="true" />下载</span>
                <strong>{{ formatMbps(speedtest.result.download_mbps) }}<small> Mbps</small></strong>
              </div>
              <div>
                <span><ArrowUpFromLine :size="13" aria-hidden="true" />上传</span>
                <strong>{{ formatMbps(speedtest.result.upload_mbps) }}<small> Mbps</small></strong>
              </div>
              <dl>
                <div><dt>延迟</dt><dd>{{ Number(speedtest.result.ping || 0).toFixed(1) }} ms</dd></div>
                <div><dt>抖动</dt><dd>{{ Number(speedtest.result.jitter || 0).toFixed(1) }} ms</dd></div>
              </dl>
              <p>上次成功：{{ speedtest.result.timestamp && !speedtest.result.timestamp.startsWith('0001-') ? formatTimestamp(speedtest.result.timestamp, true) : '时间未知' }} · {{ speedtest.result.server_name || '测速站未知' }} · 耗流 {{ formatBytes(sumBytes(speedtest.result.bytes_sent, speedtest.result.bytes_received)) }}</p>
            </div>

            <div v-else-if="speedtest.running || starting" class="speed-running" role="status" aria-live="polite">
              <LoaderCircle class="loading-icon" :size="24" aria-hidden="true" />
              <div>
                <strong>{{ starting && !speedtest.running ? '正在连接测速节点' : '正在测量服务器链路' }}</strong>
                <span>网卡采样：↓ {{ formatMbps(speedtest.live?.download_mbps) }} Mbps · ↑ {{ formatMbps(speedtest.live?.upload_mbps) }} Mbps</span>
              </div>
            </div>

            <div v-else class="service-empty">
              <Zap :size="24" aria-hidden="true" />
              <p>测试 VPS 到 LibreSpeed 节点的吞吐、延迟与抖动。</p>
            </div>

            <p v-if="speedtest.error && !speedtest.running" class="service-warning" role="status">{{ speedtest.error }}</p>
            <Button class="speed-action" :disabled="speedtest.running || starting" @click="emit('speedtest')">
              <LoaderCircle v-if="speedtest.running || starting" class="loading-icon" :size="15" aria-hidden="true" />
              <Play v-else :size="15" aria-hidden="true" />
              {{ speedButtonLabel }}
            </Button>
          </div>
        </article>
      </div>
    </div>

    <p class="measurement-note">
      <Info :size="14" aria-hidden="true" />
      <span>协议流量是 sing-box 观察到的代理有效载荷近似值；采样中断会保留缺口，不用于结算、配额执法或审计。网卡累计包含其他进程，供应商套餐用量以 RackNerd 官方数据为准。测速会短暂占用带宽并计入套餐流量。</span>
    </p>
  </div>
</template>

<style scoped>
.traffic {
  --hairline: color-mix(in srgb, var(--border) 78%, transparent);
  --hairline-soft: color-mix(in srgb, var(--border) 48%, transparent);
  display: grid;
  gap: 0.85rem;
  min-width: 0;
  font-variant-numeric: tabular-nums;
}

.live-section {
  min-width: 0;
  display: grid;
  gap: 0.75rem;
  padding-block: 0.1rem 0.15rem;
}

.topline {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 0.75rem;
  padding-bottom: 0.45rem;
  border-bottom: 1px solid var(--hairline);
}

.topline h2 {
  margin: 0;
  font-size: 1rem;
  font-weight: 650;
  letter-spacing: -0.018em;
  line-height: 1.25;
}

.topline-right {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
}

.sampled {
  min-width: 0;
  color: var(--muted-foreground);
  font-size: 0.67rem;
  white-space: nowrap;
}

.live-badge {
  gap: 0.35rem;
  border-radius: 0.35rem;
}

.dot {
  width: 0.38rem;
  height: 0.38rem;
  display: inline-block;
  flex: 0 0 auto;
  border-radius: 50%;
  background: currentColor;
}

.dot.live {
  box-shadow: 0 0 0 0.16rem color-mix(in srgb, currentColor 12%, transparent);
}

.dot.stale {
  animation: sample-pulse 1.8s ease-in-out infinite;
}

.traffic-alert {
  margin: 0;
  border-color: var(--hairline);
  border-radius: 0.5rem;
  padding-block: 0.65rem;
  box-shadow: none;
}

.traffic-alert :deep([data-slot='alert-description']) {
  color: inherit;
}

/* ---------- stat cards ---------- */

.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0;
  border: 1px solid var(--hairline);
  border-radius: 0.55rem;
  overflow: hidden;
  background: var(--card);
}

.stat {
  position: relative;
  min-width: 0;
  display: grid;
  align-content: start;
  gap: 0.32rem;
  padding: 0.72rem 0.8rem;
  background: var(--card);
}

.stat + .stat {
  border-left: 1px solid var(--hairline);
}

.stat-head {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  color: var(--muted-foreground);
  font-size: 0.68rem;
  font-weight: 500;
}

.stat-head svg {
  color: var(--muted-foreground);
}

.stat-value {
  font-size: clamp(1.02rem, 1.45vw, 1.22rem);
  font-weight: 680;
  letter-spacing: -0.018em;
  line-height: 1.2;
  overflow-wrap: anywhere;
}

.stat-pair {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.4rem;
  color: var(--muted-foreground);
  font-size: 0.64rem;
  line-height: 1.35;
}

.stat-pair span {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  overflow-wrap: anywhere;
}

/* ---------- cards ---------- */

.card {
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--hairline);
  border-radius: 0.55rem;
  background: var(--card);
  box-shadow: none;
}

.card-head {
  min-width: 0;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.68rem 0.8rem;
  border-bottom: 1px solid var(--hairline);
}

.card-title {
  min-width: 0;
  display: grid;
  gap: 0.15rem;
}

.card-title h3 {
  margin: 0;
  font-size: 0.82rem;
  font-weight: 650;
  letter-spacing: -0.012em;
  line-height: 1.3;
}

.card-title small {
  max-width: 52rem;
  color: var(--muted-foreground);
  font-size: 0.65rem;
  line-height: 1.42;
}

.card-body {
  min-width: 0;
  padding: 0.78rem 0.8rem;
}

.proto-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.7rem;
}

.card.reality {
  --accent: var(--chart-1);
  border-top-color: color-mix(in srgb, var(--chart-1) 72%, var(--hairline));
}

.card.hysteria2 {
  --accent: var(--chart-2);
  border-top-color: color-mix(in srgb, var(--chart-2) 72%, var(--hairline));
}

.flow-badge {
  gap: 0.35rem;
}

.card.reality .flow-badge {
  color: var(--chart-1);
}

.card.hysteria2 .flow-badge {
  color: var(--chart-2);
}

.rate-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0;
  border: 1px solid var(--hairline-soft);
  border-radius: 0.4rem;
  overflow: hidden;
}

.rate-grid > div {
  min-width: 0;
  padding: 0.62rem 0.68rem;
  background: color-mix(in srgb, var(--accent) 3.5%, var(--card));
}

.rate-grid > div + div {
  border-left: 1px solid var(--hairline-soft);
}

.rate-grid span {
  display: flex;
  align-items: center;
  gap: 0.28rem;
  color: var(--muted-foreground);
  font-size: 0.65rem;
}

.rate-grid span svg {
  color: var(--accent);
}

.rate-grid strong {
  display: block;
  margin-top: 0.3rem;
  overflow-wrap: anywhere;
  font-size: 0.9rem;
  font-weight: 650;
  letter-spacing: -0.015em;
}

.cumulative {
  display: grid;
  grid-template-columns: minmax(0, 0.82fr) minmax(0, 1.18fr);
  align-items: end;
  gap: 0.8rem;
  margin-top: 0.7rem;
  padding-top: 0.7rem;
  border-top: 1px solid var(--hairline-soft);
}

.cumulative-main > span {
  display: block;
  color: var(--muted-foreground);
  font-size: 0.64rem;
}

.cumulative-main > strong {
  display: block;
  margin-top: 0.2rem;
  overflow-wrap: anywhere;
  font-size: 1rem;
  letter-spacing: -0.018em;
}

.cumulative dl {
  display: grid;
  gap: 0.24rem;
  margin: 0;
}

.cumulative dl > div {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.5rem;
  font-size: 0.66rem;
}

.cumulative dt {
  color: var(--muted-foreground);
}

.cumulative dd {
  margin: 0;
  overflow-wrap: anywhere;
  text-align: right;
  font-weight: 600;
}

/* ---------- unattributed ---------- */

.unattributed-body {
  display: grid;
  grid-template-columns: minmax(12rem, 0.85fr) minmax(0, 1.15fr);
  align-items: center;
  gap: 0.9rem;
}

.coverage-line {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.8rem;
}

.coverage-line span {
  color: var(--muted-foreground);
  font-size: 0.68rem;
}

.coverage-line strong {
  font-size: 1rem;
  letter-spacing: -0.018em;
}

.coverage-progress {
  height: 0.28rem;
  margin-top: 0.52rem;
  border-radius: 0;
}

.mini-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0;
  margin: 0;
  border: 1px solid var(--hairline-soft);
  border-radius: 0.4rem;
  overflow: hidden;
}

.mini-stats > div {
  min-width: 0;
  padding: 0.58rem 0.62rem;
  background: color-mix(in srgb, var(--muted) 55%, var(--card));
}

.mini-stats > div + div {
  border-left: 1px solid var(--hairline-soft);
}

.mini-stats dt {
  color: var(--muted-foreground);
  font-size: 0.62rem;
}

.mini-stats dd {
  margin: 0.24rem 0 0;
  overflow-wrap: anywhere;
  font-size: 0.75rem;
  font-weight: 650;
}

/* ---------- charts ---------- */

.chart-body {
  padding: 0.72rem 0.8rem 0.78rem;
}

/* ---------- infrastructure ---------- */

.infra-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.45fr) minmax(18rem, 0.72fr);
  gap: 0.7rem;
  align-items: stretch;
}

.host-chart {
  min-height: 13.5rem;
  display: flex;
}

.foot-3 {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border-top: 1px solid var(--hairline);
}

.foot-3 > div {
  min-width: 0;
  padding: 0.62rem 0.72rem;
}

.foot-3 > div + div {
  border-left: 1px solid var(--hairline);
}

.foot-3 span {
  display: block;
  color: var(--muted-foreground);
  font-size: 0.62rem;
}

.foot-3 strong {
  display: block;
  margin-top: 0.22rem;
  overflow-wrap: anywhere;
  font-size: 0.73rem;
}

.service-stack {
  display: grid;
  grid-template-rows: repeat(2, minmax(0, 1fr));
  gap: 0.7rem;
  min-width: 0;
}

.service-stack > .card {
  height: 100%;
}

.service-body {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  gap: 0.5rem;
  min-height: 0;
}

.quota-value {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.quota-value strong {
  font-size: 1.18rem;
  letter-spacing: -0.018em;
}

.quota-value span {
  color: var(--muted-foreground);
  font-size: 0.66rem;
}

.quota-label {
  display: flex;
  justify-content: space-between;
  gap: 0.45rem;
  color: var(--muted-foreground);
  font-size: 0.65rem;
}

.service-caption,
.service-warning,
.speed-readout > p {
  margin: 0.25rem 0 0;
  color: var(--muted-foreground);
  font-size: 0.64rem;
  line-height: 1.45;
}

.service-warning {
  color: var(--destructive);
}

.service-empty {
  min-height: 6.2rem;
  flex: 1 1 auto;
  display: grid;
  align-content: center;
  justify-items: center;
  gap: 0.5rem;
  color: var(--muted-foreground);
  text-align: center;
}

.service-empty p {
  max-width: 18rem;
  margin: 0;
  color: var(--muted-foreground);
  font-size: 0.69rem;
  line-height: 1.45;
}

.speed-readout {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0;
  border: 1px solid var(--hairline-soft);
  border-radius: 0.4rem;
  overflow: hidden;
}

.speed-readout > div {
  min-width: 0;
  padding: 0.58rem 0.64rem;
  background: color-mix(in srgb, var(--muted) 55%, var(--card));
}

.speed-readout > div + div {
  border-left: 1px solid var(--hairline-soft);
}

.speed-readout > div > span {
  display: flex;
  align-items: center;
  gap: 0.24rem;
  color: var(--muted-foreground);
  font-size: 0.65rem;
}

.speed-readout > div > strong {
  display: block;
  margin-top: 0.24rem;
  overflow-wrap: anywhere;
  font-size: 0.92rem;
}

.speed-readout strong small {
  color: var(--muted-foreground);
  font-size: 0.62rem;
  font-weight: 500;
}

.speed-readout dl {
  grid-column: 1 / -1;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.45rem;
  margin: 0;
  padding: 0.52rem 0.64rem 0;
  border-top: 1px solid var(--hairline-soft);
}

.speed-readout dl > div {
  display: flex;
  justify-content: space-between;
  gap: 0.4rem;
  font-size: 0.66rem;
}

.speed-readout dt {
  color: var(--muted-foreground);
}

.speed-readout dd {
  margin: 0;
  font-weight: 600;
}

.speed-readout > p {
  grid-column: 1 / -1;
  padding: 0 0.64rem 0.55rem;
}

.speed-running {
  flex: 1 1 auto;
  min-height: 5.2rem;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.65rem;
  padding: 0.75rem;
  border: 1px solid var(--hairline-soft);
  border-radius: 0.4rem;
  background: color-mix(in srgb, var(--primary) 3%, var(--card));
  color: var(--primary);
}

.speed-running strong {
  display: block;
  color: var(--foreground);
  font-size: 0.76rem;
}

.speed-running span {
  display: block;
  margin-top: 0.22rem;
  color: var(--muted-foreground);
  font-size: 0.66rem;
  overflow-wrap: anywhere;
}

.speed-action {
  width: 100%;
  margin-top: auto;
}

.loading-icon {
  animation: rotate 0.9s linear infinite;
}

.measurement-note {
  display: flex;
  align-items: flex-start;
  gap: 0.45rem;
  margin: 0;
  padding-top: 0.58rem;
  border-top: 1px solid var(--hairline-soft);
  color: var(--muted-foreground);
  font-size: 0.65rem;
  line-height: 1.55;
}

.measurement-note svg {
  flex: 0 0 auto;
  margin-top: 0.15rem;
  color: var(--muted-foreground);
}

@keyframes sample-pulse {
  50% { opacity: 0.42; }
}

@keyframes rotate {
  to { transform: rotate(360deg); }
}

/* ---------- responsive ---------- */

@media (max-width: 1180px) {
  .stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .stat + .stat {
    border-left: 0;
  }

  .stat:nth-child(even) {
    border-left: 1px solid var(--hairline);
  }

  .stat:nth-child(n + 3) {
    border-top: 1px solid var(--hairline);
  }

  .infra-grid {
    grid-template-columns: 1fr;
  }

  .service-stack {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    grid-template-rows: auto;
  }
}

@media (max-width: 820px) {
  .proto-grid,
  .service-stack {
    grid-template-columns: 1fr;
  }

  .unattributed-body {
    grid-template-columns: 1fr;
  }

  .host-chart {
    min-height: 12rem;
  }
}

@media (max-width: 620px) {
  .topline {
    align-items: flex-start;
    flex-direction: column;
  }

  .topline-right {
    width: 100%;
    justify-content: space-between;
  }

  .stat-grid {
    grid-template-columns: 1fr;
  }

  .stat:nth-child(even) {
    border-left: 0;
  }

  .stat + .stat,
  .stat:nth-child(n + 3) {
    border-top: 1px solid var(--hairline);
  }

  .rate-grid,
  .cumulative,
  .mini-stats,
  .speed-readout {
    grid-template-columns: 1fr;
  }

  .rate-grid > div + div,
  .mini-stats > div + div,
  .speed-readout > div + div {
    border-top: 1px solid var(--hairline-soft);
    border-left: 0;
  }

  .cumulative {
    align-items: stretch;
  }

  .foot-3 {
    grid-template-columns: 1fr;
  }

  .foot-3 > div + div {
    border-top: 1px solid var(--hairline);
    border-left: 0;
  }
}

@media (max-width: 420px) {
  .traffic {
    gap: 0.7rem;
  }

  .topline-right {
    align-items: flex-start;
    flex-direction: column;
  }

  .card-head {
    align-items: flex-start;
    flex-direction: column;
    gap: 0.45rem;
  }

  .card-head > :last-child {
    max-width: 100%;
  }

  .stat-pair {
    align-items: flex-start;
    flex-direction: column;
  }

  .quota-label {
    align-items: flex-start;
    flex-direction: column;
  }
}

@media (prefers-reduced-motion: reduce) {
  .dot.stale,
  .loading-icon {
    animation: none;
  }
}
</style>
