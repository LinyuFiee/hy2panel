<script setup>
import { computed } from 'vue'
import { Activity, ArrowDownToLine, ArrowUpFromLine, CircleHelp, Cpu, Database, Gauge, GitBranch, HardDrive, MemoryStick, Network, Play, Settings2, ShieldCheck, Zap } from '@lucide/vue'
import HairlineFigure from './HairlineFigure.vue'
import NetworkChart from './NetworkChart.vue'
import ProtocolTrafficChart from './ProtocolTrafficChart.vue'
import { formatBytes, formatCount, formatMbps, formatRate, formatTimestamp, formatUptime, percent, sumBytes } from '../utils'

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
const status = computed(() => props.state.status || {})
const racknerd = computed(() => props.state.racknerd || { configured: false })
const speedtest = computed(() => props.state.speedtest || { running: false })
const memoryPercent = computed(() => percent(status.value.memory_used_mb, status.value.memory_total_mb))
const diskPercent = computed(() => percent(status.value.disk_used_bytes, status.value.disk_total_bytes))
const trafficPercent = computed(() => racknerd.value.info?.percent_used || 0)
const coveragePercent = computed(() => Math.min(100, Math.max(0, Number(props.traffic?.attribution_coverage || 0) * 100)))
const protocolDefinitions = [
  { key: 'reality', name: 'Reality', className: 'reality', kicker: 'TCP / VLESS' },
  { key: 'hysteria2', name: 'Hysteria2', className: 'hysteria2', kicker: 'UDP / QUIC' }
]
const trafficNotice = computed(() => {
  if (!props.traffic || !props.traffic.available) return props.traffic?.error || '协议监控暂不可用，主机网络数据不受影响。'
  if (props.traffic.stale) {
    const lastSuccess = props.traffic.last_success ? `，最后成功于 ${formatTimestamp(props.traffic.last_success, true)}` : ''
    return `协议数据已暂停更新${lastSuccess}${props.traffic.error ? `：${props.traffic.error}` : ''}`
  }
  return ''
})

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
</script>

<template>
  <div class="view-stack">
    <section class="welcome-card">
      <div class="welcome-copy">
        <span class="eyebrow"><Activity :size="14" /> LIVE OVERVIEW</span>
        <h2>让连接轻松一点。</h2>
        <p>节点运行、协议流量和网络质量，都在一个清爽的位置。</p>
        <div class="welcome-badges">
          <span class="status-pill" :class="status.sing_box === 'active' ? 'ok' : 'warn'">
            <i></i>{{ status.sing_box === 'active' ? 'sing-box 运行中' : 'sing-box 需检查' }}
          </span>
          <span class="soft-pill">已运行 {{ formatUptime(status.uptime_seconds) }}</span>
        </div>
      </div>
      <HairlineFigure />
    </section>

    <section class="stat-grid">
      <article class="stat-card">
        <div class="stat-icon mint"><Cpu :size="20" /></div>
        <div class="stat-meta"><span>CPU 使用率</span><strong>{{ Number(status.cpu_percent || 0).toFixed(1) }}%</strong></div>
        <div class="mini-track"><i :style="{ width: `${Math.min(Number(status.cpu_percent || 0), 100)}%` }"></i></div>
        <small>负载 {{ status.load_average }}</small>
      </article>
      <article class="stat-card">
        <div class="stat-icon lavender"><MemoryStick :size="20" /></div>
        <div class="stat-meta"><span>内存</span><strong>{{ status.memory_used_mb }} MB</strong></div>
        <div class="mini-track lavender-track"><i :style="{ width: `${memoryPercent}%` }"></i></div>
        <small>共 {{ status.memory_total_mb }} MB · {{ memoryPercent.toFixed(0) }}%</small>
      </article>
      <article class="stat-card">
        <div class="stat-icon peach"><HardDrive :size="20" /></div>
        <div class="stat-meta"><span>系统磁盘</span><strong>{{ formatBytes(status.disk_used_bytes) }}</strong></div>
        <div class="mini-track peach-track"><i :style="{ width: `${diskPercent}%` }"></i></div>
        <small>共 {{ formatBytes(status.disk_total_bytes) }} · {{ diskPercent.toFixed(0) }}%</small>
      </article>
      <article class="stat-card">
        <div class="stat-icon sky"><Network :size="20" /></div>
        <div class="stat-meta"><span>累计主机流量</span><strong>{{ formatBytes(sumBytes(status.rx_total_bytes, status.tx_total_bytes)) }}</strong></div>
        <div class="traffic-split"><span>↓ {{ formatBytes(status.rx_total_bytes) }}</span><span>↑ {{ formatBytes(status.tx_total_bytes) }}</span></div>
        <small>自本次系统启动以来</small>
      </article>
    </section>

    <section class="protocol-section" aria-labelledby="protocol-traffic-heading">
      <div class="section-heading-row">
        <div><span class="section-kicker">PROTOCOL MONITOR</span><h2 id="protocol-traffic-heading">协议流量</h2></div>
        <span v-if="traffic?.available && !traffic.stale" class="live-dot">采样 {{ formatTimestamp(traffic.sampled_at) }}</span>
        <span v-else class="status-pill warn">{{ traffic?.stale ? '数据已过期' : '暂不可用' }}</span>
      </div>
      <div v-if="trafficNotice" class="protocol-alert" :class="traffic?.stale ? 'stale' : 'unavailable'" role="status">
        <CircleHelp :size="16" />{{ trafficNotice }}
      </div>
      <div class="protocol-card-grid">
        <article v-for="protocol in protocolDefinitions" :key="protocol.key" class="panel-card protocol-card" :class="protocol.className" :data-testid="`${protocol.key}-traffic-card`">
          <div class="protocol-card-heading">
            <div class="protocol-icon" :class="protocol.className"><GitBranch :size="21" /></div>
            <div><span>{{ protocol.kicker }}</span><h3>{{ protocol.name }}</h3></div>
            <strong>{{ formatCount(traffic?.active_flows?.[protocol.key] || 0) }}<small> 活动代理流</small></strong>
          </div>
          <div class="protocol-live-grid">
            <div><ArrowDownToLine :size="17" /><span>实时下行</span><strong>{{ formatRate(protocolRates(protocol.key).download) }}</strong></div>
            <div><ArrowUpFromLine :size="17" /><span>实时上行</span><strong>{{ formatRate(protocolRates(protocol.key).upload) }}</strong></div>
          </div>
          <div class="protocol-total">
            <span>累计流量</span><strong>{{ formatBytes(protocolTotal(protocol.key)) }}</strong>
            <small>↓ {{ formatBytes(protocolTraffic(protocol.key).download) }} · ↑ {{ formatBytes(protocolTraffic(protocol.key).upload) }}</small>
          </div>
        </article>
      </div>
      <article class="panel-card attribution-card" data-testid="unattributed-traffic">
        <div>
          <span class="section-kicker">ATTRIBUTION</span>
          <h3>未归属流量</h3>
          <p>无法安全归类到 Reality 或 Hysteria2 的代理流量会单独保留。</p>
        </div>
        <div class="attribution-metrics">
          <div><span>累计</span><strong>{{ formatBytes(protocolTotal('unattributed')) }}</strong></div>
          <div><span>当前速率</span><strong>↓ {{ formatRate(protocolRates('unattributed').download) }} · ↑ {{ formatRate(protocolRates('unattributed').upload) }}</strong></div>
          <div><span>活动代理流</span><strong>{{ formatCount(traffic?.active_flows?.unattributed || 0) }}</strong></div>
        </div>
        <div class="coverage-meter">
          <div><span>协议归属覆盖率</span><strong>{{ coveragePercent.toFixed(1) }}%</strong></div>
          <div class="large-track"><i :style="{ width: `${coveragePercent}%` }"></i></div>
        </div>
      </article>
      <article class="panel-card protocol-history-panel">
        <div class="panel-heading">
          <div><span class="section-kicker">HISTORY</span><h3>协议流量趋势</h3></div>
          <small>时间戳按浏览器本地时间显示</small>
        </div>
        <ProtocolTrafficChart :history="history" :range="historyRange" :loading="historyLoading" @range="emit('history-range', $event)" />
      </article>
    </section>

    <section class="dashboard-grid">
      <article class="panel-card network-panel">
        <div class="panel-heading">
          <div><span class="section-kicker">HOST REALTIME</span><h3>主机网络</h3></div>
          <span class="live-dot">实时</span>
        </div>
        <NetworkChart :samples="samples" />
      </article>

      <article class="panel-card bandwidth-panel">
        <div class="panel-heading">
          <div><span class="section-kicker">RACKNERD</span><h3>本月流量</h3></div>
          <Database :size="20" class="muted-icon" />
        </div>
        <template v-if="racknerd.configured && racknerd.info">
          <div class="bandwidth-number">{{ formatBytes(racknerd.info.bandwidth_used) }}<span> / {{ formatBytes(racknerd.info.bandwidth_total) }}</span></div>
          <div class="large-track"><i :style="{ width: `${trafficPercent}%` }"></i></div>
          <div class="bandwidth-details"><span>已使用 {{ Number(trafficPercent).toFixed(2) }}%</span><span>剩余 {{ formatBytes(racknerd.info.bandwidth_free) }}</span></div>
          <small v-if="racknerd.error" class="warning-text">显示缓存数据：{{ racknerd.error }}</small>
          <small v-else>数据来自 RackNerd 官方 API，每 5 分钟更新</small>
        </template>
        <div v-else class="empty-state compact">
          <Database :size="28" />
          <p>{{ racknerd.configured ? racknerd.error || '正在读取官方流量…' : '连接 RackNerd API 后，自动展示套餐流量。' }}</p>
          <button class="text-button" @click="emit('settings')">去配置 <Settings2 :size="15" /></button>
        </div>
      </article>

      <article class="panel-card speed-panel">
        <div class="panel-heading">
          <div><span class="section-kicker">LIBRESPEED</span><h3>服务器测速</h3></div>
          <Gauge :size="20" class="muted-icon" />
        </div>
        <div v-if="speedtest.result && !speedtest.running" class="speed-results">
          <div><ArrowDownToLine :size="18" /><span>下载</span><strong>{{ formatMbps(speedtest.result.download_mbps) }}<small> Mbps</small></strong></div>
          <div><ArrowUpFromLine :size="18" /><span>上传</span><strong>{{ formatMbps(speedtest.result.upload_mbps) }}<small> Mbps</small></strong></div>
          <div class="latency-line"><span>延迟 {{ Number(speedtest.result.ping || 0).toFixed(1) }} ms</span><span>抖动 {{ Number(speedtest.result.jitter || 0).toFixed(1) }} ms</span></div>
          <small>{{ speedtest.result.server_name }} · 耗流 {{ formatBytes(sumBytes(speedtest.result.bytes_sent, speedtest.result.bytes_received)) }}</small>
        </div>
        <div v-else-if="speedtest.running" class="speed-results speed-live">
          <div><ArrowDownToLine :size="18" /><span>实时下载吞吐</span><strong>{{ formatMbps(speedtest.live?.download_mbps) }}<small> Mbps</small></strong></div>
          <div><ArrowUpFromLine :size="18" /><span>实时上传吞吐</span><strong>{{ formatMbps(speedtest.live?.upload_mbps) }}<small> Mbps</small></strong></div>
          <div class="latency-line"><span class="live-dot">测速中</span><span>每秒更新</span></div>
          <small>实时值包含其他连接，最终结果以 LibreSpeed 为准。</small>
        </div>
        <div v-else class="speed-placeholder">
          <Zap :size="28" />
          <p>按需测试 VPS 到 LibreSpeed 节点的网络质量。</p>
        </div>
        <p v-if="speedtest.error && !speedtest.running" class="warning-text">{{ speedtest.error }}</p>
        <button class="primary-button full-button" :disabled="speedtest.running || starting" @click="emit('speedtest')">
          <Play :size="16" />{{ speedtest.running ? '测速进行中…' : '开始测速' }}
        </button>
        <small>测速会短暂占用带宽并消耗套餐流量。</small>
      </article>

      <article class="panel-card bbr-panel">
        <div class="panel-heading">
          <div><span class="section-kicker">CONGESTION CONTROL</span><h3>BBR 加速</h3></div>
          <ShieldCheck :size="20" class="muted-icon" />
        </div>
        <div class="bbr-status" :class="{ active: status.bbr_enabled }">
          <span>{{ status.bbr_enabled ? '已启用' : '未完整启用' }}</span>
          <strong>{{ status.congestion_control }} + {{ status.default_qdisc }}</strong>
        </div>
        <p>BBR 优化 TCP 连接；Hysteria2 使用 QUIC 自有拥塞控制。</p>
        <button class="text-button" @click="emit('settings')">查看网络设置 <Settings2 :size="15" /></button>
      </article>
    </section>
  </div>
</template>
