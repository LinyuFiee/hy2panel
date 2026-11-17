import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

const mockState = {
  config: {
    panel: { domain: 'panel.example.com', public_host: '192.0.2.10' },
    reality: { enabled: true, port: 443, server_name: 'www.microsoft.com', uuid: '00000000-0000-4000-8000-000000000000', private_key: 'mock-private-key', public_key: 'mock-public-key', short_id: '0123456789abcdef' },
    hysteria2: { enabled: true, port: 443, domain: 'hy2.example.com', password: 'mock-password', up_mbps: 100, down_mbps: 100 },
    recovery: { auto_port: false, last_port_change: 0 }
  },
  links: {
    reality: 'vless://00000000-0000-4000-8000-000000000000@192.0.2.10:443?security=reality&type=tcp#Reality',
    hysteria2: 'hysteria2://mock-password@hy2.example.com:443?insecure=0#Hysteria2'
  },
  status: {
    sing_box: 'active', load_average: '0.18 0.12 0.09', memory_used_mb: 486, memory_total_mb: 1024,
    cpu_percent: 18.6, disk_used_bytes: 6442450944, disk_total_bytes: 21474836480, uptime_seconds: 268400,
    rx_bytes_per_second: 1284000, tx_bytes_per_second: 386000, rx_total_bytes: 824633720832, tx_total_bytes: 103079215104,
    congestion_control: 'bbr', default_qdisc: 'fq', bbr_enabled: true
  },
  racknerd: {
    configured: true, available: true,
    info: { bandwidth_total: 3221225472000, bandwidth_used: 1288490188800, bandwidth_free: 1932735283200, percent_used: 40, hostname: 'demo', vm_state: 'online', updated_at: '2026-11-07T10:00:00Z' }
  },
  speedtest: {
    running: false,
    live: null,
    result: { timestamp: '2026-11-07T10:00:00Z', server_name: 'San Jose · LibreSpeed', ping: 18.6, jitter: 1.4, download_mbps: 683, upload_mbps: 412, bytes_sent: 200000000, bytes_received: 600000000 }
  },
  node_meta: { country_code: 'US', country_name: 'United States', flag: '🇺🇸', reality_name: '🇺🇸 US-Reality', hysteria2_name: '🇺🇸 US-Hysteria2' },
  update: { current_version: '0.6.0', latest_version: '0.6.0', release_url: 'https://github.com/LinyuFiee/hy2panel/releases/latest', available: false, checking: false, updating: false }
}

const mockTraffic = {
  available: true,
  stale: false,
  sampled_at: '2026-11-16T10:32:00Z',
  last_success: '2026-11-16T10:32:00Z',
  traffic: {
    reality: { upload: '9007199254740993', download: '18934876160' },
    hysteria2: { upload: '5368709120', download: '12884901888' },
    unattributed: { upload: '268435456', download: '805306368' }
  },
  total: { upload: '9007204891889665', download: '32624934912' },
  rates: {
    reality: { upload: 586240, download: 2846720 },
    hysteria2: { upload: 294912, download: 1622016 },
    unattributed: { upload: 18432, download: 53248 }
  },
  active_flows: { reality: 12, hysteria2: 7, unattributed: 2 },
  attribution_coverage: 0.973
}

const singBoxVersions = {
  current_version: '1.14.3',
  previous_version: '1.14.2',
  recommended_version: '1.14.4',
  can_rollback: true,
  versions: [
    { version: '1.14.4', tag: 'v1.14.4', status: 'recommended', asset_name: 'sing-box-1.14.4-linux-amd64.tar.gz', size: 31710753, sha256: '699a1b510aa4396b2176d32d1181920399ebf1e267835da6f3b598d0f7c24968', release_url: 'https://github.com/SagerNet/sing-box/releases/tag/v1.14.4' },
    { version: '1.14.3', tag: 'v1.14.3', status: 'supported', asset_name: 'sing-box-1.14.3-linux-amd64.tar.gz', size: 31584231, sha256: '799a1b510aa4396b2176d32d1181920399ebf1e267835da6f3b598d0f7c24969', release_url: 'https://github.com/SagerNet/sing-box/releases/tag/v1.14.3' },
    { version: '1.14.2', tag: 'v1.14.2', status: 'supported', asset_name: 'sing-box-1.14.2-linux-amd64.tar.gz', size: 31457280, sha256: '899a1b510aa4396b2176d32d1181920399ebf1e267835da6f3b598d0f7c24960', release_url: 'https://github.com/SagerNet/sing-box/releases/tag/v1.14.2' }
  ],
  operation: {}
}

const initialMockState = JSON.stringify(mockState)
const initialMockTraffic = JSON.stringify(mockTraffic)
const initialSingBoxVersions = JSON.stringify(singBoxVersions)

function replaceObject(target, source) {
  for (const key of Object.keys(target)) delete target[key]
  Object.assign(target, JSON.parse(source))
}

let mockTick = 0
let operationPhaseTimer
let operationCompleteTimer

function advanceMetrics() {
  mockTick += 1
  mockState.status.rx_bytes_per_second = Math.max(0, 900000 + Math.sin(mockTick * 0.8) * 520000 + mockTick * 18000)
  mockState.status.tx_bytes_per_second = Math.max(0, 260000 + Math.cos(mockTick * 0.7) * 130000)
  mockState.status.cpu_percent = 16 + Math.sin(mockTick) * 6
  mockTraffic.sampled_at = new Date().toISOString()
  mockTraffic.last_success = mockTraffic.sampled_at
  mockTraffic.rates.reality.download = 2450000 + Math.sin(mockTick * 0.7) * 520000
  mockTraffic.rates.hysteria2.download = 1280000 + Math.cos(mockTick * 0.6) * 310000
}

function historyPayload(range) {
  const sizes = { '1h': 60, '6h': 72, '24h': 96, '7d': 112 }
  const steps = { '1h': 60000, '6h': 5 * 60000, '24h': 15 * 60000, '7d': 90 * 60000 }
  const count = sizes[range] || sizes['1h']
  const step = steps[range] || steps['1h']
  const now = Date.parse('2026-11-16T10:32:00Z')
  const points = Array.from({ length: count }, (_, index) => {
    const realityDownload = Math.round(52000000 + Math.sin(index * 0.31) * 18000000 + index * 110000)
    const realityUpload = Math.round(13000000 + Math.cos(index * 0.27) * 4500000)
    const hysteriaDownload = Math.round(29000000 + Math.cos(index * 0.23) * 9000000)
    const hysteriaUpload = Math.round(9000000 + Math.sin(index * 0.29) * 3200000)
    const unattributedDownload = 900000 + (index % 4) * 120000
    const unattributedUpload = 240000 + (index % 3) * 60000
    const firstGap = Math.floor(count * 0.42)
    const gap = index === firstGap || index === firstGap + 1
    return {
      minute: new Date(now - (count - 1 - index) * step).toISOString(),
      traffic: {
        reality: { upload: String(realityUpload), download: String(realityDownload) },
        hysteria2: { upload: String(hysteriaUpload), download: String(hysteriaDownload) },
        unattributed: { upload: String(unattributedUpload), download: String(unattributedDownload) }
      },
      total: {
        upload: String(realityUpload + hysteriaUpload + unattributedUpload),
        download: String(realityDownload + hysteriaDownload + unattributedDownload)
      },
      gap
    }
  })
  return { range, available: true, points }
}

function beginCoreOperation(action, targetVersion = '') {
  clearTimeout(operationPhaseTimer)
  clearTimeout(operationCompleteTimer)
  const operation = {
    id: `mock-${Date.now()}`,
    action,
    status: 'running',
    phase: 'requested',
    target_version: targetVersion,
    message: action === 'switch' ? `准备切换到 v${targetVersion}` : '准备回滚上一版本'
  }
  singBoxVersions.operation = operation
  operationPhaseTimer = setTimeout(() => {
    singBoxVersions.operation = { ...operation, phase: 'verifying', message: '正在校验官方资产 SHA-256' }
  }, 500)
  operationCompleteTimer = setTimeout(() => {
    const oldCurrent = singBoxVersions.current_version
    const nextVersion = action === 'rollback' ? singBoxVersions.previous_version : targetVersion
    singBoxVersions.current_version = nextVersion
    singBoxVersions.previous_version = oldCurrent
    singBoxVersions.can_rollback = true
    singBoxVersions.operation = { ...operation, status: 'succeeded', phase: 'completed', message: `sing-box 已运行 v${nextVersion}` }
  }, 1700)
  return operation
}

export default defineConfig({
  plugins: [
    vue(),
    tailwindcss(),
    {
      name: 'mock-ui',
      configureServer(server) {
        if (process.env.MOCK_UI !== '1') return
        server.middlewares.use((req, res, next) => {
          if (!req.url?.startsWith('/api/')) return next()
          res.setHeader('Content-Type', 'application/json; charset=utf-8')
          if (req.url === '/api/login') {
            res.end('{"status":"ok"}')
            return
          }
          if (req.url === '/api/test/reset') {
            clearTimeout(operationPhaseTimer)
            clearTimeout(operationCompleteTimer)
            mockTick = 0
            replaceObject(mockState, initialMockState)
            replaceObject(mockTraffic, initialMockTraffic)
            replaceObject(singBoxVersions, initialSingBoxVersions)
            res.end('{"status":"ok"}')
            return
          }
          if (req.url === '/api/logout') {
            res.end('{"status":"ok"}')
            return
          }
          if (req.url === '/api/system/status') {
            advanceMetrics()
            res.end(JSON.stringify({ status: mockState.status, speedtest: mockState.speedtest }))
            return
          }
          if (req.url === '/api/traffic/live') {
            advanceMetrics()
            res.end(JSON.stringify(mockTraffic))
            return
          }
          if (req.url?.startsWith('/api/traffic/history')) {
            const url = new URL(req.url, 'http://localhost')
            const range = url.searchParams.get('range') || '1h'
            if (!['1h', '6h', '24h', '7d'].includes(range)) {
              res.statusCode = 400
              res.end('{"error":"unsupported traffic history range"}')
              return
            }
            res.end(JSON.stringify(historyPayload(range)))
            return
          }
          if (req.url === '/api/sing-box/versions') {
            res.end(JSON.stringify(singBoxVersions))
            return
          }
          if (req.url === '/api/sing-box/switch') {
            let body = ''
            req.on('data', chunk => { body += chunk })
            req.on('end', () => {
              if (singBoxVersions.operation.status === 'running') {
                res.statusCode = 409
                res.end('{"error":"已有核心维护任务正在运行"}')
                return
              }
              let target = ''
              try { target = JSON.parse(body).version } catch {}
              if (!singBoxVersions.versions.some(entry => entry.version === target)) {
                res.statusCode = 400
                res.end('{"error":"该 sing-box 版本不在兼容清单中"}')
                return
              }
              res.statusCode = 202
              res.end(JSON.stringify(beginCoreOperation('switch', target)))
            })
            return
          }
          if (req.url === '/api/sing-box/rollback') {
            if (singBoxVersions.operation.status === 'running') {
              res.statusCode = 409
              res.end('{"error":"已有核心维护任务正在运行"}')
              return
            }
            if (!singBoxVersions.can_rollback) {
              res.statusCode = 409
              res.end('{"error":"没有可回滚的上一版本"}')
              return
            }
            res.statusCode = 202
            res.end(JSON.stringify(beginCoreOperation('rollback')))
            return
          }
          if (req.url?.startsWith('/api/logs')) {
            res.end(JSON.stringify({ unit: 'sing-box', entries: ['2026-11-07T10:00:00+00:00 sing-box started', '2026-11-07T10:00:01+00:00 Reality TCP 443 listening', '2026-11-07T10:00:02+00:00 Hysteria2 UDP 443 listening'] }))
            return
          }
          if (req.url === '/api/diagnostics') {
            res.end(JSON.stringify({ checked_at: new Date().toISOString(), pass: 7, warn: 1, fail: 0, checks: [
              { id: 'config', name: 'sing-box 配置', status: 'pass', message: '配置语法与协议字段校验通过' },
              { id: 'service', name: '核心服务', status: 'pass', message: 'sing-box.service 正在运行' },
              { id: 'reality-listen', name: 'Reality 监听', status: 'pass', message: 'TCP 443 正在监听' },
              { id: 'hy2-listen', name: 'Hysteria2 监听', status: 'pass', message: 'UDP 443 正在监听' },
              { id: 'bbr', name: 'BBR 网络优化', status: 'pass', message: 'BBR + fq 已启用' },
              { id: 'external', name: '外部连通性', status: 'info', message: '本机无法判断运营商或 GFW 是否封堵端口', suggestion: '使用客户端或外部 TCP/UDP 探测复核。' },
              { id: 'dns', name: '域名解析', status: 'warn', message: '模拟提醒：请核对解析记录', suggestion: '检查 DNS 服务商解析。' }
            ] }))
            return
          }
          if (req.url === '/api/settings/recovery') {
            let body = ''
            req.on('data', chunk => { body += chunk })
            req.on('end', () => {
              try { mockState.config.recovery.auto_port = Boolean(JSON.parse(body).auto_port) } catch {}
              res.end(JSON.stringify(mockState))
            })
            return
          }
          if (req.url === '/api/update/check') {
            mockState.update.latest_version = '0.6.0'
            mockState.update.available = true
            res.end(JSON.stringify(mockState.update))
            return
          }
          if (req.url === '/api/update/apply') {
            mockState.update.updating = true
            mockState.update.message = '正在下载并校验更新，面板将短暂重启'
            res.end(JSON.stringify(mockState.update))
            setTimeout(() => {
              mockState.update.updating = false
              mockState.update.available = false
              mockState.update.current_version = '0.6.0'
              mockState.update.message = '更新完成，请刷新页面'
            }, 2000)
            return
          }
          if (req.url === '/api/speedtest') {
            mockState.speedtest.running = true
            mockState.speedtest.live = { download_mbps: 512.4, upload_mbps: 128.2 }
            res.end(JSON.stringify({ running: true, live: mockState.speedtest.live }))
            setTimeout(() => {
              mockState.speedtest.running = false
              mockState.speedtest.live = null
            }, 3000)
            return
          }
          if (req.url === '/api/state') advanceMetrics()
          res.end(JSON.stringify(mockState))
        })
      }
    }
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true
  }
})
