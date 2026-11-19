<script setup>
import { computed } from 'vue'
import {
  ArrowDownToLine,
  ArrowUpFromLine,
  ChevronRight,
  Cpu,
  Gauge,
  HardDrive,
  MemoryStick
} from '@lucide/vue'
import HairlineFigure from './HairlineFigure.vue'
import NodeFlowFigure from './NodeFlowFigure.vue'
import { formatBytes, formatRate, formatUptime, percent, sumBytes } from '../utils'

const props = defineProps({
  config: { type: Object, default: null },
  status: { type: Object, default: () => ({}) },
  racknerd: { type: Object, default: () => ({ configured: false }) },
  healthy: { type: Boolean, default: false },
  maintenanceBusy: { type: Boolean, default: false }
})

const emit = defineEmits(['goto'])

const cpuPercent = computed(() => Math.min(100, Math.max(0, Number(props.status.cpu_percent || 0))))
const memPercent = computed(() => percent(props.status.memory_used_mb, props.status.memory_total_mb))
const diskPercent = computed(() => percent(props.status.disk_used_bytes, props.status.disk_total_bytes))
const totalRate = computed(() => formatRate(sumBytes(props.status.rx_bytes_per_second, props.status.tx_bytes_per_second)))
const rackInfo = computed(() => props.racknerd?.info || null)
const realityEnabled = computed(() => Boolean(props.config?.reality?.enabled))
const hysteriaEnabled = computed(() => Boolean(props.config?.hysteria2?.enabled))
const serviceLabel = computed(() => props.healthy ? 'sing-box 服务正常' : 'sing-box 服务异常')
const maintenanceLabel = computed(() => props.maintenanceBusy ? '维护任务运行中' : '维护队列空闲')
const panelLabel = computed(() => props.config?.panel?.domain || '当前管理会话')

function barClass(value) {
  if (value >= 90) return 'bar-bad'
  if (value >= 70) return 'bar-warn'
  return 'bar-ok'
}
</script>

<template>
  <div class="overview">
    <section class="relay-hero" aria-labelledby="relay-heading">
      <div class="relay-copy">
        <span class="section-index">RELAY / 01</span>
        <div class="relay-title-row">
          <div>
            <h2 id="relay-heading">单节点转发面板</h2>
            <p>集中查看服务状态、维护占用与面板入口，再进入协议和流量分区处理具体事务。</p>
          </div>
          <span class="service-state" :class="healthy ? 'is-ok' : 'is-bad'">
            <i aria-hidden="true"></i>{{ healthy ? '服务正常' : '服务异常' }}
          </span>
        </div>

        <dl class="relay-context">
          <div>
            <dt>服务</dt>
            <dd><span class="context-dot" :class="healthy ? 'ok' : 'bad'" aria-hidden="true"></span>{{ serviceLabel }}</dd>
          </div>
          <div>
            <dt>维护</dt>
            <dd><span class="context-dot" :class="maintenanceBusy ? 'warn' : 'ok'" aria-hidden="true"></span>{{ maintenanceLabel }}</dd>
          </div>
          <div>
            <dt>面板</dt>
            <dd class="tnum">{{ panelLabel }}</dd>
          </div>
          <div>
            <dt>运行时间</dt>
            <dd class="tnum">{{ formatUptime(status.uptime_seconds) }}</dd>
          </div>
        </dl>

        <p class="schematic-note">
          <span aria-hidden="true">FIG. 01</span>
          交互式转发示意，不是实时遥测；移动指针可查看中继路径的逐级响应。
        </p>
      </div>

      <figure class="relay-figure">
        <div class="figure-rail" aria-hidden="true">
          <span>RELAY SCHEMATIC</span>
          <span>INTERACTIVE / NOT LIVE</span>
        </div>
        <div class="figure-surface">
          <HairlineFigure
            class="relay-drawing"
            figure="relay"
            name="relay"
            play
            :intensity="0.52"
            theme="auto"
            label="交互式中继路径示意：移动指针可查看节点之间逐级点亮的转发路径；此图不是实时遥测"
          />
        </div>
        <figcaption>交互式中继结构示意 · 非实时遥测</figcaption>
      </figure>
    </section>

    <section class="stat-section" aria-labelledby="resource-heading">
      <header class="section-heading">
        <div>
          <span class="section-index">HOST / 02</span>
          <h2 id="resource-heading">资源状态</h2>
        </div>
        <span class="section-meta">CURRENT SAMPLE</span>
      </header>

      <div class="stat-grid">
        <article class="stat">
          <div class="stat-head"><Cpu :size="15" /><span>CPU 使用率</span><em>01</em></div>
          <strong class="stat-value tnum">{{ cpuPercent.toFixed(1) }}%</strong>
          <div class="track" :class="barClass(cpuPercent)" role="meter" aria-label="CPU 使用率" aria-valuemin="0" aria-valuemax="100" :aria-valuenow="cpuPercent"><i :style="{ width: `${cpuPercent}%` }"></i></div>
          <small class="stat-sub tnum">负载 {{ status.load_average || '—' }}</small>
        </article>

        <article class="stat">
          <div class="stat-head"><MemoryStick :size="15" /><span>内存</span><em>02</em></div>
          <strong class="stat-value tnum">{{ formatBytes(Number(status.memory_used_mb || 0) * 1024 * 1024) }}</strong>
          <div class="track" :class="barClass(memPercent)" role="meter" aria-label="内存使用率" aria-valuemin="0" aria-valuemax="100" :aria-valuenow="memPercent"><i :style="{ width: `${memPercent}%` }"></i></div>
          <small class="stat-sub tnum">共 {{ formatBytes(Number(status.memory_total_mb || 0) * 1024 * 1024) }} · {{ memPercent.toFixed(0) }}%</small>
        </article>

        <article class="stat">
          <div class="stat-head"><HardDrive :size="15" /><span>磁盘</span><em>03</em></div>
          <strong class="stat-value tnum">{{ formatBytes(status.disk_used_bytes) }}</strong>
          <div class="track" :class="barClass(diskPercent)" role="meter" aria-label="磁盘使用率" aria-valuemin="0" aria-valuemax="100" :aria-valuenow="diskPercent"><i :style="{ width: `${diskPercent}%` }"></i></div>
          <small class="stat-sub tnum">共 {{ formatBytes(status.disk_total_bytes) }} · {{ diskPercent.toFixed(0) }}%</small>
        </article>

        <article class="stat">
          <div class="stat-head"><Gauge :size="15" /><span>主机实时速率</span><em>04</em></div>
          <strong class="stat-value tnum">{{ totalRate }}</strong>
          <div class="rate-pair tnum">
            <small><ArrowDownToLine :size="12" />{{ formatRate(status.rx_bytes_per_second) }}</small>
            <small><ArrowUpFromLine :size="12" />{{ formatRate(status.tx_bytes_per_second) }}</small>
          </div>
          <small class="stat-sub">上下行合计</small>
        </article>
      </div>
    </section>

    <section class="protocol-path" aria-labelledby="protocol-path-heading">
      <header class="protocol-copy">
        <span class="section-index">PROTOCOL PATH / 03</span>
        <h2 id="protocol-path-heading">协议路径</h2>
        <p>项目内置拓扑用于解释客户端数据进入 sing-box 后如何分配到 Reality、Hysteria2 与普通转发路径。</p>
        <span class="protocol-disclaimer">交互式协议示意 · 不代表当前连接或实时流量</span>
      </header>
      <div class="protocol-figure">
        <NodeFlowFigure
          play
          :intensity="0.55"
          label="交互式协议路径示意：手机客户端数据进入云服务器 sing-box，并按 Reality、Hysteria2 与普通数据路径转发；此图不是实时遥测"
        />
      </div>
    </section>

    <section class="group-section" aria-labelledby="operations-heading">
      <header class="section-heading">
        <div>
          <span class="section-index">OPERATIONS / 04</span>
          <h2 id="operations-heading">服务入口</h2>
        </div>
        <span class="section-meta">3 SECTIONS</span>
      </header>

      <div class="group-grid">
        <div class="group">
          <h3>节点协议</h3>
          <div class="group-card">
            <div class="row">
              <span class="dot" :class="realityEnabled ? 'ok' : 'off'"></span>
              <div class="row-main">
                <strong>Reality</strong>
                <small class="tnum">TCP · {{ config?.reality?.port || '—' }}</small>
              </div>
              <em>{{ realityEnabled ? '在线' : '停用' }}</em>
            </div>
            <div class="row">
              <span class="dot" :class="hysteriaEnabled ? 'ok' : 'off'"></span>
              <div class="row-main">
                <strong>Hysteria2</strong>
                <small class="tnum">UDP · {{ config?.hysteria2?.port || '—' }}</small>
              </div>
              <em>{{ hysteriaEnabled ? '在线' : '停用' }}</em>
            </div>
            <button class="row row-action" type="button" @click="emit('goto', 'nodes')">
              <span class="row-main"><strong>管理节点</strong><small>协议与客户端凭据</small></span>
              <ChevronRight :size="15" />
            </button>
          </div>
        </div>

        <div class="group">
          <h3>套餐流量</h3>
          <div class="group-card">
            <div class="row quota-row">
              <div class="row-main">
                <strong class="tnum">{{ rackInfo ? formatBytes(rackInfo.bandwidth_used) : '未配置' }}</strong>
                <small class="tnum">
                  {{ rackInfo ? `本月已用 ${Number(rackInfo.percent_used || 0).toFixed(1)}% · 共 ${formatBytes(rackInfo.bandwidth_total)}` : '在设置中填写 RackNerd API 凭据' }}
                </small>
              </div>
            </div>
            <div v-if="rackInfo" class="row track-row">
              <div class="track rack-track" :class="barClass(Number(rackInfo.percent_used || 0))" role="meter" aria-label="本月套餐流量使用率" aria-valuemin="0" aria-valuemax="100" :aria-valuenow="Math.min(Number(rackInfo.percent_used || 0), 100)">
                <i :style="{ width: `${Math.min(Number(rackInfo.percent_used || 0), 100)}%` }"></i>
              </div>
            </div>
            <button class="row row-action" type="button" @click="emit('goto', 'traffic')">
              <span class="row-main"><strong>流量分析</strong><small>协议与主机口径</small></span>
              <ChevronRight :size="15" />
            </button>
          </div>
        </div>

        <div class="group">
          <h3>系统</h3>
          <div class="group-card">
            <div class="row">
              <span class="dot" :class="healthy ? 'ok' : 'bad'"></span>
              <div class="row-main">
                <strong>{{ healthy ? '服务正常' : '服务异常' }}</strong>
                <small>{{ maintenanceBusy ? '维护任务运行中' : '无维护任务' }}</small>
              </div>
            </div>
            <div class="row uptime-row">
              <div class="row-main">
                <small>主机运行时间</small>
                <strong class="tnum">{{ formatUptime(status.uptime_seconds) }}</strong>
              </div>
            </div>
            <button class="row row-action" type="button" @click="emit('goto', 'maintenance')">
              <span class="row-main"><strong>维护中心</strong><small>自检、恢复与日志</small></span>
              <ChevronRight :size="15" />
            </button>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.overview {
  display: grid;
  gap: 1.35rem;
  width: 100%;
  min-width: 0;
  font-variant-numeric: tabular-nums;
}

.relay-hero,
.protocol-path {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 0.85rem;
  background: var(--card);
}

.relay-hero {
  display: grid;
  grid-template-columns: minmax(18rem, 0.88fr) minmax(22rem, 1.12fr);
}

.relay-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  padding: clamp(1.25rem, 2.4vw, 2rem);
  border-right: 1px solid var(--border);
}

.section-index,
.section-meta,
.figure-rail,
.protocol-disclaimer,
.schematic-note > span {
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  letter-spacing: 0.1em;
}

.section-index {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  color: var(--muted-foreground);
  font-size: 0.58rem;
  font-weight: 650;
}

.section-index::before {
  width: 1.35rem;
  height: 1px;
  content: '';
  background: currentColor;
  opacity: 0.7;
}

.relay-title-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  margin-top: 0.9rem;
}

.relay-title-row h2,
.protocol-copy h2,
.section-heading h2 {
  margin: 0;
  color: var(--foreground);
  font-weight: 650;
  letter-spacing: -0.035em;
}

.relay-title-row h2 {
  font-size: clamp(1.55rem, 2.5vw, 2.15rem);
}

.relay-title-row p {
  max-width: 31rem;
  margin: 0.55rem 0 0;
  color: var(--muted-foreground);
  font-size: 0.76rem;
  line-height: 1.65;
}

.service-state {
  display: inline-flex;
  align-items: center;
  gap: 0.38rem;
  flex: 0 0 auto;
  padding: 0.35rem 0.55rem;
  border: 1px solid var(--border);
  border-radius: 999px;
  color: var(--muted-foreground);
  font-size: 0.65rem;
  white-space: nowrap;
}

.service-state i {
  width: 0.38rem;
  height: 0.38rem;
  border-radius: 50%;
  background: currentColor;
}

.service-state.is-ok {
  border-color: color-mix(in srgb, var(--success) 28%, var(--border));
  color: var(--success);
}

.service-state.is-bad {
  border-color: color-mix(in srgb, var(--destructive) 28%, var(--border));
  color: var(--destructive);
}

.relay-context {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  margin: auto 0 1.1rem;
  padding: 1.1rem 0 0;
}

.relay-context > div {
  min-width: 0;
  padding: 0.72rem 0.8rem;
  border-top: 1px solid var(--border);
}

.relay-context > div:nth-child(odd) {
  padding-left: 0;
  border-right: 1px solid var(--border);
}

.relay-context dt {
  margin-bottom: 0.3rem;
  color: var(--muted-foreground);
  font-size: 0.62rem;
}

.relay-context dd {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 0.42rem;
  margin: 0;
  color: var(--foreground);
  font-size: 0.72rem;
  font-weight: 580;
  overflow-wrap: anywhere;
}

.context-dot {
  width: 0.38rem;
  height: 0.38rem;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--muted-foreground);
}

.context-dot.ok {
  background: var(--success);
}

.context-dot.warn {
  background: var(--warning);
}

.context-dot.bad {
  background: var(--destructive);
}

.schematic-note {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 0.7rem;
  margin: 0;
  padding-top: 0.9rem;
  border-top: 1px solid var(--border);
  color: var(--muted-foreground);
  font-size: 0.66rem;
  line-height: 1.55;
}

.schematic-note > span {
  color: var(--foreground);
  font-size: 0.55rem;
  white-space: nowrap;
}

.relay-figure {
  min-width: 0;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  margin: 0;
  background: var(--background);
  --hairline-plate: var(--background);
}

.figure-rail {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.8rem;
  min-height: 2.2rem;
  padding: 0.55rem 0.75rem;
  border-bottom: 1px solid var(--border);
  color: var(--muted-foreground);
  font-size: 0.52rem;
}

.figure-surface {
  min-width: 0;
  display: grid;
  place-items: center;
  padding: 0.25rem;
}

.relay-drawing {
  display: block;
  width: min(100%, 34rem);
  min-width: 0;
}

.relay-figure figcaption {
  padding: 0.62rem 0.75rem;
  border-top: 1px solid var(--border);
  color: var(--muted-foreground);
  font-size: 0.62rem;
}

.stat-section,
.group-section {
  min-width: 0;
  display: grid;
  gap: 0.65rem;
}

.section-heading {
  min-width: 0;
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 1rem;
  padding-inline: 0.15rem;
}

.section-heading h2 {
  margin-top: 0.3rem;
  font-size: 1.05rem;
  letter-spacing: -0.02em;
}

.section-meta {
  color: var(--muted-foreground);
  font-size: 0.52rem;
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 0.8rem;
  background: var(--card);
}

.stat {
  min-width: 0;
  display: grid;
  align-content: start;
  gap: 0.45rem;
  padding: 1rem;
}

.stat + .stat {
  border-left: 1px solid var(--border);
}

.stat-head {
  min-width: 0;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.42rem;
  color: var(--muted-foreground);
  font-size: 0.68rem;
  font-weight: 520;
}

.stat-head svg {
  color: var(--foreground);
}

.stat-head em {
  color: var(--muted-foreground);
  font: 0.55rem/1 ui-monospace, SFMono-Regular, Consolas, monospace;
  font-style: normal;
}

.stat-value {
  min-width: 0;
  overflow: hidden;
  color: var(--foreground);
  font-size: clamp(1.15rem, 2vw, 1.4rem);
  font-weight: 650;
  letter-spacing: -0.025em;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.track {
  height: 2px;
  overflow: hidden;
  background: var(--secondary);
}

.track i {
  display: block;
  height: 100%;
  background: var(--foreground);
}

.bar-ok i {
  background: var(--foreground);
}

.bar-warn i {
  background: var(--warning);
}

.bar-bad i {
  background: var(--destructive);
}

.stat-sub,
.rate-pair small {
  color: var(--muted-foreground);
  font-size: 0.65rem;
}

.rate-pair {
  min-width: 0;
  display: flex;
  justify-content: space-between;
  gap: 0.4rem;
}

.rate-pair small {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 0.22rem;
  white-space: nowrap;
}

.protocol-path {
  display: grid;
  grid-template-columns: minmax(15rem, 0.36fr) minmax(0, 1fr);
}

.protocol-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  padding: 1.2rem;
  border-right: 1px solid var(--border);
}

.protocol-copy h2 {
  margin-top: 0.65rem;
  font-size: 1.2rem;
}

.protocol-copy p {
  margin: 0.55rem 0 1rem;
  color: var(--muted-foreground);
  font-size: 0.7rem;
  line-height: 1.65;
}

.protocol-disclaimer {
  margin-top: auto;
  padding-top: 0.85rem;
  border-top: 1px solid var(--border);
  color: var(--muted-foreground);
  font-size: 0.53rem;
  line-height: 1.55;
}

.protocol-figure {
  min-width: 0;
  display: grid;
  align-items: center;
  padding: 0.5rem 0.75rem;
  background: var(--card);
}

.protocol-figure :deep(.node-flow) {
  min-width: 0;
}

.group-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.75rem;
  align-items: start;
}

.group {
  min-width: 0;
}

.group h3 {
  margin: 0 0 0.4rem 0.2rem;
  color: var(--muted-foreground);
  font-size: 0.68rem;
  font-weight: 620;
  letter-spacing: 0.015em;
}

.group-card {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 0.75rem;
  background: var(--card);
}

.row {
  width: 100%;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 0.65rem;
  margin: 0;
  padding: 0.72rem 0.85rem;
  border: 0;
  background: transparent;
  color: var(--foreground);
  font: inherit;
  text-align: left;
}

.row + .row {
  border-top: 1px solid var(--border);
}

.row-main {
  min-width: 0;
  flex: 1;
  display: grid;
  gap: 0.16rem;
}

.row-main strong {
  min-width: 0;
  color: var(--foreground);
  font-size: 0.78rem;
  font-weight: 620;
  overflow-wrap: anywhere;
}

.row-main small {
  min-width: 0;
  color: var(--muted-foreground);
  font-size: 0.64rem;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.row > em {
  color: var(--muted-foreground);
  font-size: 0.64rem;
  font-style: normal;
  white-space: nowrap;
}

.row-action {
  color: var(--muted-foreground);
}

.row-action svg {
  flex: 0 0 auto;
  color: var(--muted-foreground);
}

.row-action:hover {
  background: var(--accent);
}

.row-action:hover .row-main strong {
  color: var(--primary);
}

.dot {
  width: 0.42rem;
  height: 0.42rem;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--muted-foreground);
  opacity: 0.45;
}

.dot.ok {
  background: var(--success);
  opacity: 1;
}

.dot.bad {
  background: var(--destructive);
  opacity: 1;
}

.rack-track {
  width: 100%;
}

.track-row {
  padding-block: 0.65rem;
}

.uptime-row .row-main {
  grid-template-columns: auto minmax(0, 1fr);
  align-items: baseline;
  gap: 0.55rem;
}

.uptime-row .row-main strong {
  text-align: right;
}

@media (max-width: 1080px) {
  .relay-hero {
    grid-template-columns: minmax(17rem, 0.8fr) minmax(19rem, 1.2fr);
  }

  .stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .stat:nth-child(3) {
    border-left: 0;
  }

  .stat:nth-child(n + 3) {
    border-top: 1px solid var(--border);
  }

  .group-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 820px) {
  .relay-hero,
  .protocol-path {
    grid-template-columns: minmax(0, 1fr);
  }

  .relay-copy,
  .protocol-copy {
    border-right: 0;
    border-bottom: 1px solid var(--border);
  }

  .relay-context {
    margin-top: 1.5rem;
  }

  .relay-figure {
    min-height: 20rem;
  }

  .protocol-copy {
    display: block;
  }

  .protocol-disclaimer {
    display: block;
    margin-top: 1rem;
  }

  .group-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 560px) {
  .overview {
    gap: 1.1rem;
  }

  .relay-copy {
    padding: 1rem;
  }

  .relay-title-row {
    display: grid;
  }

  .service-state {
    justify-self: start;
  }

  .relay-context {
    grid-template-columns: minmax(0, 1fr);
    margin-top: 1rem;
  }

  .relay-context > div,
  .relay-context > div:nth-child(odd) {
    padding-inline: 0;
    border-right: 0;
  }

  .schematic-note {
    grid-template-columns: minmax(0, 1fr);
    gap: 0.35rem;
  }

  .relay-figure {
    min-height: 0;
  }

  .figure-rail span:last-child {
    display: none;
  }

  .stat-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .stat + .stat,
  .stat:nth-child(3),
  .stat:nth-child(n + 3) {
    border-top: 1px solid var(--border);
    border-left: 0;
  }

  .protocol-copy,
  .protocol-figure {
    padding: 1rem;
  }

  .protocol-figure :deep(.node-flow svg) {
    min-height: 8rem;
  }

  .section-meta {
    display: none;
  }
}

@media (max-width: 360px) {
  .relay-copy,
  .protocol-copy,
  .protocol-figure,
  .stat,
  .row {
    padding-inline: 0.8rem;
  }

  .rate-pair {
    flex-direction: column;
    align-items: flex-start;
  }

  .uptime-row .row-main {
    grid-template-columns: minmax(0, 1fr);
  }

  .uptime-row .row-main strong {
    text-align: left;
  }
}
</style>
