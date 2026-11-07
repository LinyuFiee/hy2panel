import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const mockState = {
  config: {
    panel: { domain: 'panel.example.com', admin_cidr: '203.0.113.10/32', public_host: '192.0.2.10' },
    reality: { enabled: true, port: 443, server_name: 'www.microsoft.com', uuid: '00000000-0000-4000-8000-000000000000', private_key: 'mock-private-key', public_key: 'mock-public-key', short_id: '0123456789abcdef' },
    hysteria2: { enabled: true, port: 443, domain: 'hy2.example.com', password: 'mock-password', up_mbps: 100, down_mbps: 100 }
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
    result: { timestamp: '2026-11-07T10:00:00Z', server_name: 'San Jose · LibreSpeed', ping: 18.6, jitter: 1.4, download_bps: 683000000, upload_bps: 412000000, bytes_sent: 200000000, bytes_received: 600000000 }
  }
}

let mockTick = 0

export default defineConfig({
  plugins: [
    vue(),
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
          if (req.url === '/api/speedtest') {
            mockState.speedtest.running = true
            res.end(JSON.stringify({ running: true }))
            setTimeout(() => { mockState.speedtest.running = false }, 3000)
            return
          }
          if (req.url === '/api/state') {
            mockTick += 1
            mockState.status.rx_bytes_per_second = 900000 + Math.sin(mockTick * 0.8) * 520000 + mockTick * 18000
            mockState.status.tx_bytes_per_second = 260000 + Math.cos(mockTick * 0.7) * 130000
            mockState.status.cpu_percent = 16 + Math.sin(mockTick) * 6
          }
          res.end(JSON.stringify(mockState))
        })
      }
    }
  ],
  build: {
    outDir: 'dist',
    emptyOutDir: true
  }
})