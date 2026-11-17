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
  Network,
  Play,
  Settings2,
  Zap
} from '@lucide/vue'
import NetworkChart from './NetworkChart.vue'
import ProtocolTrafficChart from './ProtocolTrafficChart.vue'
import { Alert, AlertDescription, AlertTitle } from './ui/alert'
import { Badge } from './ui/badge'
import { Button } from './ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle
} from './ui/card'
import { Progress } from './ui/progress'
import { Separator } from './ui/separator'
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
  { key: 'reality', name: 'Reality', transport: 'TCP · VLESS', tone: 'reality' },
  { key: 'hysteria2', name: 'Hysteria2', transport: 'UDP · QUIC', tone: 'hysteria2' }
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
  <div class="traffic-workspace">
    <section class="protocol-live-section" aria-labelledby="protocol-live-heading">
      <div class="section-intro">
        <div>
          <span class="workspace-kicker"><Activity :size="14" /> LIVE PROTOCOLS</span>
          <h2 id="protocol-live-heading">协议实时</h2>
          <p>分开观察 Reality 与 Hysteria2 的当前速率、活动连接和累计代理载荷。</p>
        </div>
        <div class="sample-state">
          <Badge :variant="trafficState.variant" class="sample-badge">
            <span class="sample-dot" :class="trafficState.state" aria-hidden="true"></span>
            {{ trafficState.label }}
          </Badge>
          <span>{{ traffic?.sampled_at ? `采样 ${formatTimestamp(traffic.sampled_at)}` : '等待首次采样' }}</span>
        </div>
      </div>

      <Alert v-if="trafficNotice" :variant="traffic?.stale ? 'warning' : 'destructive'" class="traffic-alert">
        <CircleHelp />
        <AlertTitle>{{ traffic?.stale ? '采样已中断' : '无法读取协议流量' }}</AlertTitle>
        <AlertDescription>{{ trafficNotice }}</AlertDescription>
      </Alert>

      <div class="telemetry-grid" aria-label="协议流量关键指标">
        <Card class="traffic-card telemetry-card">
          <div class="telemetry-icon"><Activity :size="17" /></div>
          <span>当前代理速率</span>
          <strong>{{ formatRate(sumBytes(liveDownloadTotal, liveUploadTotal)) }}</strong>
          <div class="telemetry-pair">
            <span><ArrowDownToLine :size="12" />{{ formatRate(liveDownloadTotal) }}</span>
            <span><ArrowUpFromLine :size="12" />{{ formatRate(liveUploadTotal) }}</span>
          </div>
        </Card>

        <Card class="traffic-card telemetry-card">
          <div class="telemetry-icon"><GitBranch :size="17" /></div>
          <span>活动代理流</span>
          <strong>{{ formatCount(activeFlowTotal) }}</strong>
          <div class="telemetry-pair">
            <span>Reality {{ formatCount(traffic?.active_flows?.reality || 0) }}</span>
            <span>Hysteria2 {{ formatCount(traffic?.active_flows?.hysteria2 || 0) }}</span>
          </div>
        </Card>

        <Card class="traffic-card telemetry-card">
          <div class="telemetry-icon"><Database :size="17" /></div>
          <span>代理观察累计</span>
          <strong>{{ formatBytes(observedTotal) }}</strong>
          <div class="telemetry-pair">
            <span>下行 {{ formatBytes(observedDirection('download')) }}</span>
            <span>上行 {{ formatBytes(observedDirection('upload')) }}</span>
          </div>
        </Card>

        <Card class="traffic-card telemetry-card coverage-summary">
          <div class="telemetry-icon"><Gauge :size="17" /></div>
          <span>协议归属覆盖率</span>
          <strong>{{ coveragePercent.toFixed(1) }}%</strong>
          <div class="telemetry-pair">
            <span>未归属 {{ unattributedPercent.toFixed(1) }}%</span>
            <span>{{ formatCount(traffic?.active_flows?.unattributed || 0) }} 条活动流</span>
          </div>
        </Card>
      </div>

      <div class="protocol-surface-grid">
        <Card
          v-for="protocol in protocolDefinitions"
          :key="protocol.key"
          class="traffic-card live-protocol-surface"
          :class="protocol.tone"
          :data-testid="`${protocol.key}-traffic-card`"
        >
          <CardHeader class="protocol-surface-header">
            <div class="protocol-identity">
              <span class="protocol-mark"><GitBranch :size="19" /></span>
              <div>
                <span>{{ protocol.transport }}</span>
                <CardTitle>{{ protocol.name }}</CardTitle>
              </div>
            </div>
            <Badge variant="outline" class="flow-badge">
              <Activity :size="12" />
              {{ formatCount(traffic?.active_flows?.[protocol.key] || 0) }} 活动流
            </Badge>
          </CardHeader>

          <CardContent class="protocol-surface-content">
            <div class="protocol-rate-grid">
              <div>
                <span><ArrowDownToLine :size="14" />实时下行</span>
                <strong>{{ formatRate(protocolRates(protocol.key).download) }}</strong>
              </div>
              <div>
                <span><ArrowUpFromLine :size="14" />实时上行</span>
                <strong>{{ formatRate(protocolRates(protocol.key).upload) }}</strong>
              </div>
            </div>
            <Separator />
            <div class="protocol-cumulative">
              <div>
                <span>累计代理载荷</span>
                <strong>{{ formatBytes(protocolTotal(protocol.key)) }}</strong>
              </div>
              <dl>
                <div><dt>下行</dt><dd>{{ formatBytes(protocolTraffic(protocol.key).download) }}</dd></div>
                <div><dt>上行</dt><dd>{{ formatBytes(protocolTraffic(protocol.key).upload) }}</dd></div>
                <div><dt>当前合计</dt><dd>{{ formatRate(protocolRateTotal(protocol.key)) }}</dd></div>
              </dl>
            </div>
          </CardContent>
        </Card>
      </div>

      <Card class="traffic-card attribution-surface" data-testid="unattributed-traffic">
        <CardHeader class="attribution-header">
          <div>
            <span class="workspace-kicker">ATTRIBUTION SAFETY</span>
            <CardTitle>未归属流量</CardTitle>
            <CardDescription>不能安全映射到指定入站的代理流量会独立保留，不会回填到 Reality 或 Hysteria2。</CardDescription>
          </div>
          <Badge variant="secondary">{{ unattributedPercent.toFixed(1) }}% 未归属</Badge>
        </CardHeader>
        <CardContent class="attribution-content">
          <div class="coverage-readout">
            <div>
              <span>协议归属覆盖率</span>
              <strong>{{ coveragePercent.toFixed(1) }}%</strong>
            </div>
            <Progress
              :model-value="coveragePercent"
              :max="100"
              class="coverage-progress"
              aria-label="协议归属覆盖率"
            />
            <small>只统计能够明确归类的代理载荷；未归属部分始终单列。</small>
          </div>
          <dl class="unattributed-metrics">
            <div>
              <dt>累计</dt>
              <dd>{{ formatBytes(protocolTotal('unattributed')) }}</dd>
            </div>
            <div>
              <dt>当前速率</dt>
              <dd>{{ formatRate(protocolRateTotal('unattributed')) }}</dd>
            </div>
            <div>
              <dt>活动代理流</dt>
              <dd>{{ formatCount(traffic?.active_flows?.unattributed || 0) }}</dd>
            </div>
          </dl>
        </CardContent>
      </Card>
    </section>

    <section class="history-section" aria-labelledby="protocol-history-heading">
      <Card class="traffic-card history-surface">
        <CardHeader class="analytics-card-header">
          <div>
            <span class="workspace-kicker">HISTORICAL SIGNAL</span>
            <h2 id="protocol-history-heading">协议流量趋势</h2>
            <CardDescription>按浏览器本地时间查看归档；采样断线以缺口显示，不插值补齐。</CardDescription>
          </div>
          <Badge variant="outline">每分钟归档</Badge>
        </CardHeader>
        <CardContent class="history-content">
          <ProtocolTrafficChart
            :history="history"
            :range="historyRange"
            :loading="historyLoading"
            @range="emit('history-range', $event)"
          />
        </CardContent>
      </Card>
    </section>

    <section class="infrastructure-section" aria-labelledby="infrastructure-heading">
      <h2 id="infrastructure-heading" class="visually-hidden">主机网络、套餐流量与服务器测速</h2>

      <Card class="traffic-card host-network-surface">
        <CardHeader class="analytics-card-header">
          <div>
            <span class="workspace-kicker">HOST INTERFACE</span>
            <CardTitle>主机网络</CardTitle>
            <CardDescription>网卡实时吞吐包含代理、面板、SSH、测速与同机其他进程。</CardDescription>
          </div>
          <Badge variant="success"><span class="sample-dot live" aria-hidden="true"></span>实时</Badge>
        </CardHeader>
        <CardContent class="network-content">
          <NetworkChart :samples="samples" />
        </CardContent>
        <CardFooter class="host-network-footer">
          <div>
            <span>当前网卡吞吐</span>
            <strong>{{ formatRate(hostLiveTotal) }}</strong>
          </div>
          <div>
            <span>本次启动累计</span>
            <strong>{{ formatBytes(hostObservedTotal) }}</strong>
          </div>
          <div>
            <span>接收 / 发送</span>
            <strong>{{ formatBytes(status.rx_total_bytes) }} / {{ formatBytes(status.tx_total_bytes) }}</strong>
          </div>
        </CardFooter>
      </Card>

      <div class="service-stack">
        <Card class="traffic-card service-surface racknerd-surface">
          <CardHeader class="service-header">
            <div class="service-heading-row">
              <span class="service-icon"><Database :size="18" /></span>
              <div>
                <span class="workspace-kicker">RACKNERD</span>
                <CardTitle>本月套餐流量</CardTitle>
              </div>
            </div>
            <Badge v-if="racknerd.info" :variant="racknerd.error ? 'warning' : 'success'">
              {{ racknerd.error ? '缓存数据' : '官方数据' }}
            </Badge>
            <Badge v-else variant="secondary">{{ racknerd.configured ? '等待数据' : '未连接' }}</Badge>
          </CardHeader>

          <CardContent class="service-content">
            <template v-if="racknerd.configured && racknerd.info">
              <div class="quota-value">
                <strong>{{ formatBytes(racknerd.info.bandwidth_used) }}</strong>
                <span>/ {{ formatBytes(racknerd.info.bandwidth_total) }}</span>
              </div>
              <div class="quota-progress-label">
                <span>已使用 {{ racknerdPercent.toFixed(2) }}%</span>
                <span>剩余 {{ formatBytes(racknerd.info.bandwidth_free) }}</span>
              </div>
              <Progress :model-value="racknerdPercent" :max="100" aria-label="RackNerd 本月套餐流量使用率" />
              <p v-if="racknerd.error" class="service-warning" role="status">显示最近一次成功缓存：{{ racknerd.error }}</p>
              <p v-else class="service-caption">RackNerd 只读 info 接口 · 每 5 分钟更新</p>
            </template>

            <div v-else class="service-empty">
              <Database :size="26" />
              <p>{{ racknerd.configured ? racknerd.error || '正在读取 RackNerd 官方套餐流量。' : '连接只读 API 后显示供应商套餐用量。' }}</p>
              <Button variant="outline" size="sm" @click="emit('settings')">
                <Settings2 :size="14" />{{ racknerd.configured ? '检查配置' : '连接 RackNerd' }}
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card class="traffic-card service-surface speedtest-surface">
          <CardHeader class="service-header">
            <div class="service-heading-row">
              <span class="service-icon speed"><Gauge :size="18" /></span>
              <div>
                <span class="workspace-kicker">LIBRESPEED</span>
                <CardTitle>服务器测速</CardTitle>
              </div>
            </div>
            <Badge v-if="speedtest.running || starting" variant="warning">运行中</Badge>
            <Badge v-else-if="speedtest.result" variant="success">最近完成</Badge>
            <Badge v-else variant="secondary">按需运行</Badge>
          </CardHeader>

          <CardContent class="service-content speedtest-content">
            <div v-if="speedtest.result && !speedtest.running && !starting" class="speedtest-readout">
              <div>
                <span><ArrowDownToLine :size="14" />下载</span>
                <strong>{{ formatMbps(speedtest.result.download_mbps) }}<small> Mbps</small></strong>
              </div>
              <div>
                <span><ArrowUpFromLine :size="14" />上传</span>
                <strong>{{ formatMbps(speedtest.result.upload_mbps) }}<small> Mbps</small></strong>
              </div>
              <dl>
                <div><dt>延迟</dt><dd>{{ Number(speedtest.result.ping || 0).toFixed(1) }} ms</dd></div>
                <div><dt>抖动</dt><dd>{{ Number(speedtest.result.jitter || 0).toFixed(1) }} ms</dd></div>
              </dl>
              <p>{{ speedtest.result.server_name || 'LibreSpeed' }} · 本次耗流 {{ formatBytes(sumBytes(speedtest.result.bytes_sent, speedtest.result.bytes_received)) }}</p>
            </div>

            <div v-else-if="speedtest.running || starting" class="speedtest-running" role="status" aria-live="polite">
              <LoaderCircle class="loading-icon" :size="25" />
              <div>
                <strong>{{ starting && !speedtest.running ? '正在连接测速节点' : '正在测量服务器链路' }}</strong>
                <span>下载 {{ formatMbps(speedtest.live?.download_mbps) }} Mbps · 上传 {{ formatMbps(speedtest.live?.upload_mbps) }} Mbps</span>
              </div>
            </div>

            <div v-else class="service-empty speedtest-empty">
              <Zap :size="26" />
              <p>按需测试 VPS 到 LibreSpeed 节点的吞吐、延迟与抖动。</p>
            </div>

            <p v-if="speedtest.error && !speedtest.running" class="service-warning" role="status">{{ speedtest.error }}</p>
            <Button class="speedtest-action" :disabled="speedtest.running || starting" @click="emit('speedtest')">
              <LoaderCircle v-if="speedtest.running || starting" class="loading-icon" :size="15" />
              <Play v-else :size="15" />
              {{ speedButtonLabel }}
            </Button>
            <p class="service-caption">测速会短暂占用带宽并计入供应商套餐流量。</p>
          </CardContent>
        </Card>
      </div>
    </section>

    <Alert class="measurement-note" role="note">
      <Info />
      <AlertTitle>数据口径</AlertTitle>
      <AlertDescription>
        协议流量是 sing-box 观察到的代理有效载荷近似值；采样中断会保留缺口，不用于结算、配额执法或审计。网卡累计包含其他进程，供应商套餐用量以 RackNerd 官方数据为准。
      </AlertDescription>
    </Alert>
  </div>
</template>

<style scoped>
:global(.control-main:has(.traffic-workspace)) {
  box-sizing: border-box;
}

.traffic-workspace {
  --workspace-shadow: 0 0.7rem 2rem color-mix(in oklab, var(--foreground) 4%, transparent);
  --reality-accent: var(--chart-1);
  --hysteria-accent: var(--chart-2);
  display: grid;
  gap: 1rem;
  min-width: 0;
  color: var(--foreground);
  font-variant-numeric: tabular-nums;
}

.protocol-live-section {
  display: grid;
  gap: 0.85rem;
  min-width: 0;
}

.section-intro,
.analytics-card-header,
.attribution-header,
.service-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.section-intro {
  padding: 0.15rem 0.1rem 0.25rem;
}

.section-intro > div:first-child,
.analytics-card-header > div:first-child,
.attribution-header > div:first-child {
  min-width: 0;
}

.workspace-kicker {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  color: var(--primary);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.58rem;
  font-weight: 750;
  letter-spacing: 0.13em;
  line-height: 1.4;
}

.section-intro h2,
.analytics-card-header h2 {
  margin: 0.38rem 0 0;
  color: var(--foreground);
  font-size: 1.15rem;
  font-weight: 680;
  letter-spacing: -0.025em;
}

.section-intro p {
  max-width: 44rem;
  margin: 0.42rem 0 0;
  color: var(--muted-foreground);
  font-size: 0.72rem;
  line-height: 1.6;
}

.sample-state {
  display: grid;
  justify-items: end;
  gap: 0.38rem;
  flex: 0 0 auto;
}

.sample-state > span:last-child {
  color: var(--muted-foreground);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.57rem;
}

.sample-badge {
  gap: 0.4rem;
}

.sample-dot {
  width: 0.38rem;
  height: 0.38rem;
  display: inline-block;
  flex: 0 0 auto;
  border-radius: 50%;
  background: currentColor;
}

.sample-dot.live {
  box-shadow: 0 0 0 0.2rem color-mix(in oklab, currentColor 14%, transparent);
}

.sample-dot.stale {
  animation: sample-pulse 1.8s ease-in-out infinite;
}

.traffic-alert {
  margin: 0;
}

.traffic-alert :deep([data-slot='alert-description']) {
  color: inherit;
}

.traffic-card {
  min-width: 0;
  gap: 0;
  overflow: hidden;
  padding: 0;
  border-color: color-mix(in oklab, var(--border) 90%, var(--foreground));
  background: var(--card);
  box-shadow: var(--workspace-shadow);
}

.telemetry-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.7rem;
}

.telemetry-card {
  position: relative;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  gap: 0.28rem 0.65rem;
  padding: 0.9rem;
  box-shadow: none;
}

.telemetry-card::after {
  position: absolute;
  inset: auto 0 0;
  height: 2px;
  background: color-mix(in oklab, var(--primary) 55%, transparent);
  content: '';
}

.telemetry-card.coverage-summary::after {
  background: color-mix(in oklab, var(--chart-3) 70%, transparent);
}

.telemetry-icon {
  grid-row: 1 / 3;
  width: 2.1rem;
  height: 2.1rem;
  display: grid;
  place-items: center;
  border-radius: 0.6rem;
  background: color-mix(in oklab, var(--primary) 11%, var(--card));
  color: var(--primary);
}

.telemetry-card > span {
  min-width: 0;
  color: var(--muted-foreground);
  font-size: 0.62rem;
}

.telemetry-card > strong {
  min-width: 0;
  overflow-wrap: anywhere;
  font-size: clamp(0.88rem, 1.35vw, 1.08rem);
  line-height: 1.2;
  letter-spacing: -0.025em;
}

.telemetry-pair {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.45rem;
  min-width: 0;
  margin-top: 0.48rem;
  color: var(--muted-foreground);
  font-size: 0.55rem;
}

.telemetry-pair span {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 0.2rem;
  overflow-wrap: anywhere;
}

.protocol-surface-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.85rem;
}

.live-protocol-surface {
  --protocol-accent: var(--reality-accent);
  position: relative;
}

.live-protocol-surface.hysteria2 {
  --protocol-accent: var(--hysteria-accent);
}

.live-protocol-surface::before {
  position: absolute;
  z-index: 1;
  inset: 0 0 auto;
  height: 3px;
  background: var(--protocol-accent);
  content: '';
}

.protocol-surface-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.8rem;
  padding: 1rem 1rem 0.85rem;
}

.protocol-identity,
.service-heading-row {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 0.7rem;
}

.protocol-mark,
.service-icon {
  width: 2.35rem;
  height: 2.35rem;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  border-radius: 0.68rem;
  background: color-mix(in oklab, var(--protocol-accent) 12%, var(--card));
  color: var(--protocol-accent);
}

.protocol-identity > div,
.service-heading-row > div {
  min-width: 0;
}

.protocol-identity > div > span {
  display: block;
  color: var(--muted-foreground);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.53rem;
  letter-spacing: 0.08em;
}

.protocol-identity h3,
.service-heading-row h3 {
  margin-top: 0.25rem;
  font-size: 0.95rem;
  letter-spacing: -0.02em;
}

.flow-badge {
  color: var(--protocol-accent);
}

.protocol-surface-content {
  display: grid;
  gap: 0.9rem;
  padding: 0 1rem 1rem;
}

.protocol-rate-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.6rem;
}

.protocol-rate-grid > div {
  min-width: 0;
  padding: 0.8rem;
  border: 1px solid color-mix(in oklab, var(--protocol-accent) 14%, var(--border));
  border-radius: 0.65rem;
  background: color-mix(in oklab, var(--protocol-accent) 5%, var(--card));
}

.protocol-rate-grid span,
.protocol-rate-grid strong {
  display: block;
}

.protocol-rate-grid span {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  color: var(--muted-foreground);
  font-size: 0.59rem;
}

.protocol-rate-grid span svg {
  color: var(--protocol-accent);
}

.protocol-rate-grid strong {
  margin-top: 0.5rem;
  overflow-wrap: anywhere;
  font-size: clamp(0.82rem, 1.45vw, 1rem);
}

.protocol-cumulative {
  display: grid;
  grid-template-columns: minmax(0, 0.8fr) minmax(0, 1.2fr);
  align-items: end;
  gap: 1rem;
}

.protocol-cumulative > div > span,
.coverage-readout > div > span {
  display: block;
  color: var(--muted-foreground);
  font-size: 0.58rem;
}

.protocol-cumulative > div > strong,
.coverage-readout > div > strong {
  display: block;
  margin-top: 0.28rem;
  overflow-wrap: anywhere;
  font-size: 1.08rem;
  letter-spacing: -0.025em;
}

.protocol-cumulative dl,
.unattributed-metrics,
.speedtest-readout dl {
  margin: 0;
}

.protocol-cumulative dl {
  display: grid;
  gap: 0.28rem;
}

.protocol-cumulative dl > div {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.55rem;
}

.protocol-cumulative dt,
.protocol-cumulative dd {
  font-size: 0.57rem;
}

.protocol-cumulative dt {
  color: var(--muted-foreground);
}

.protocol-cumulative dd {
  margin: 0;
  overflow-wrap: anywhere;
  text-align: right;
}

.attribution-header {
  align-items: center;
  padding: 1rem 1rem 0;
}

.attribution-header h3 {
  margin-top: 0.32rem;
  font-size: 0.95rem;
}

.attribution-header p,
.analytics-card-header p {
  margin-top: 0.32rem;
  font-size: 0.65rem;
  line-height: 1.5;
}

.attribution-content {
  display: grid;
  grid-template-columns: minmax(15rem, 0.85fr) minmax(0, 1.15fr);
  align-items: center;
  gap: 1.15rem;
  padding: 0.9rem 1rem 1rem;
}

.coverage-readout {
  min-width: 0;
}

.coverage-readout > div {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.8rem;
}

.coverage-progress {
  height: 0.45rem;
  margin-top: 0.65rem;
}

.coverage-readout small {
  display: block;
  margin-top: 0.45rem;
  color: var(--muted-foreground);
  font-size: 0.54rem;
  line-height: 1.45;
}

.unattributed-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.55rem;
}

.unattributed-metrics > div {
  min-width: 0;
  padding: 0.72rem;
  border: 1px solid var(--border);
  border-radius: 0.62rem;
  background: var(--muted);
}

.unattributed-metrics dt {
  color: var(--muted-foreground);
  font-size: 0.55rem;
}

.unattributed-metrics dd {
  margin: 0.32rem 0 0;
  overflow-wrap: anywhere;
  font-size: 0.74rem;
  font-weight: 680;
}

.history-section,
.infrastructure-section {
  min-width: 0;
}

.analytics-card-header {
  align-items: center;
  padding: 1rem 1rem 0.8rem;
  border-bottom: 1px solid var(--border);
}

.analytics-card-header h3 {
  margin-top: 0.35rem;
  font-size: 0.95rem;
}

.history-content {
  padding: 0.9rem 1rem 1rem;
}

.infrastructure-section {
  display: grid;
  grid-template-columns: minmax(0, 1.45fr) minmax(19rem, 0.72fr);
  align-items: stretch;
  gap: 0.85rem;
}

.host-network-surface {
  height: 100%;
}

.network-content {
  min-height: 16rem;
  display: flex;
  padding: 0.9rem 1rem;
}

.host-network-footer {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0;
  padding: 0;
  border-top: 1px solid var(--border);
}

.host-network-footer > div {
  min-width: 0;
  padding: 0.8rem 1rem;
}

.host-network-footer > div + div {
  border-left: 1px solid var(--border);
}

.host-network-footer span,
.host-network-footer strong {
  display: block;
}

.host-network-footer span {
  color: var(--muted-foreground);
  font-size: 0.54rem;
}

.host-network-footer strong {
  margin-top: 0.3rem;
  overflow-wrap: anywhere;
  font-size: 0.68rem;
}

.service-stack {
  display: grid;
  grid-template-rows: repeat(2, minmax(0, 1fr));
  gap: 0.85rem;
  min-width: 0;
}

.service-surface {
  height: 100%;
}

.service-header {
  align-items: center;
  padding: 0.95rem 1rem 0.75rem;
  border-bottom: 1px solid var(--border);
}

.service-icon {
  --protocol-accent: var(--chart-4);
}

.service-icon.speed {
  --protocol-accent: var(--chart-3);
}

.service-content {
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 0.9rem 1rem 1rem;
}

.quota-value {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.quota-value strong {
  font-size: 1.35rem;
  letter-spacing: -0.04em;
}

.quota-value span {
  color: var(--muted-foreground);
  font-size: 0.68rem;
}

.quota-progress-label {
  display: flex;
  justify-content: space-between;
  gap: 0.5rem;
  margin: 0.8rem 0 0.42rem;
  color: var(--muted-foreground);
  font-size: 0.56rem;
}

.service-caption,
.service-warning,
.speedtest-readout > p {
  margin: 0.65rem 0 0;
  color: var(--muted-foreground);
  font-size: 0.55rem;
  line-height: 1.5;
}

.service-warning {
  color: var(--destructive);
}

.service-empty {
  min-height: 8.5rem;
  display: grid;
  align-content: center;
  justify-items: center;
  gap: 0.65rem;
  color: var(--muted-foreground);
  text-align: center;
}

.service-empty p {
  max-width: 19rem;
  margin: 0;
  color: var(--muted-foreground);
  font-size: 0.64rem;
  line-height: 1.55;
}

.speedtest-content {
  gap: 0.6rem;
}

.speedtest-readout {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.65rem;
}

.speedtest-readout > div {
  min-width: 0;
  padding: 0.65rem;
  border-radius: 0.6rem;
  background: var(--muted);
}

.speedtest-readout > div > span {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  color: var(--muted-foreground);
  font-size: 0.55rem;
}

.speedtest-readout > div > strong {
  display: block;
  margin-top: 0.35rem;
  overflow-wrap: anywhere;
  font-size: 1rem;
}

.speedtest-readout strong small {
  color: var(--muted-foreground);
  font-size: 0.52rem;
  font-weight: 500;
}

.speedtest-readout dl {
  grid-column: 1 / -1;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.5rem;
}

.speedtest-readout dl > div {
  display: flex;
  justify-content: space-between;
  gap: 0.4rem;
  padding: 0 0.2rem;
  font-size: 0.56rem;
}

.speedtest-readout dt {
  color: var(--muted-foreground);
}

.speedtest-readout dd {
  margin: 0;
  font-weight: 650;
}

.speedtest-readout > p {
  grid-column: 1 / -1;
  margin-top: 0;
}

.speedtest-running {
  min-height: 6.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.7rem;
  padding: 0.9rem;
  border: 1px dashed color-mix(in oklab, var(--primary) 35%, var(--border));
  border-radius: 0.65rem;
  background: color-mix(in oklab, var(--primary) 5%, var(--card));
  color: var(--primary);
}

.speedtest-running strong,
.speedtest-running span {
  display: block;
}

.speedtest-running strong {
  color: var(--foreground);
  font-size: 0.7rem;
}

.speedtest-running span {
  margin-top: 0.25rem;
  color: var(--muted-foreground);
  font-size: 0.56rem;
}

.speedtest-empty {
  min-height: 5.8rem;
}

.speedtest-action {
  width: 100%;
  margin-top: auto;
}

.loading-icon {
  animation: rotate 0.9s linear infinite;
}

.measurement-note {
  width: auto;
  min-width: 0;
  justify-self: stretch;
  margin: 0;
  background: color-mix(in oklab, var(--muted) 62%, var(--card));
}

.measurement-note :deep([data-slot='alert-description']) {
  color: var(--muted-foreground);
  line-height: 1.55;
}

.visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  margin: -1px;
  padding: 0;
  border: 0;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}

@keyframes sample-pulse {
  50% { opacity: 0.42; }
}

@keyframes rotate {
  to { transform: rotate(360deg); }
}

@media (max-width: 1180px) {
  .telemetry-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .infrastructure-section {
    grid-template-columns: 1fr;
  }

  .service-stack {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    grid-template-rows: auto;
  }
}

@media (max-width: 820px) {
  .protocol-surface-grid,
  .service-stack {
    grid-template-columns: 1fr;
  }

  .attribution-content {
    grid-template-columns: 1fr;
  }

  .network-content {
    min-height: 13rem;
  }
}

@media (max-width: 620px) {
  .traffic-workspace {
    gap: 0.8rem;
  }

  .section-intro,
  .analytics-card-header,
  .attribution-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .sample-state {
    width: 100%;
    justify-items: start;
  }

  .telemetry-grid {
    grid-template-columns: 1fr;
  }

  .protocol-surface-header,
  .service-header {
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .measurement-note {
    max-width: 100%;
  }

  .flow-badge,
  .service-header > [data-slot='badge'] {
    margin-left: 3.05rem;
  }

  .protocol-rate-grid,
  .protocol-cumulative,
  .unattributed-metrics {
    grid-template-columns: 1fr;
  }

  .protocol-cumulative {
    align-items: stretch;
  }

  .host-network-footer {
    grid-template-columns: 1fr;
  }

  .host-network-footer > div + div {
    border-top: 1px solid var(--border);
    border-left: 0;
  }

  .quota-progress-label {
    align-items: flex-start;
    flex-direction: column;
  }
}

@media (max-width: 420px) {
  .telemetry-pair {
    align-items: flex-start;
    flex-direction: column;
  }

  .speedtest-readout {
    grid-template-columns: 1fr;
  }

  .speedtest-readout dl,
  .speedtest-readout > p {
    grid-column: auto;
  }
}

@media (prefers-reduced-motion: reduce) {
  .sample-dot.stale,
  .loading-icon {
    animation: none;
  }
}
</style>
