export function createPanelApi(client) {
  const get = path => client.request(path)
  const post = (path, json = {}) => client.request(path, { method: 'POST', json })

  return {
    login: password => post('/api/login', { password }),
    logout: () => post('/api/logout'),
    state: () => get('/api/state'),
    systemStatus: () => get('/api/system/status'),
    trafficLive: () => get('/api/traffic/live'),
    trafficHistory: range => get(`/api/traffic/history?range=${encodeURIComponent(range)}`),
    singBoxVersions: () => get('/api/sing-box/versions'),
    switchSingBox: version => post('/api/sing-box/switch', { version }),
    rollbackSingBox: () => post('/api/sing-box/rollback'),
    saveNodes: ({ public_host, reality, hysteria2 }) => post('/api/config', { public_host, reality, hysteria2 }),
    saveRackNerd: ({ apiKey, apiHash }) => post('/api/settings/racknerd', {
      api_key: apiKey,
      api_hash: apiHash,
      clear: false
    }),
    clearRackNerd: () => post('/api/settings/racknerd', { api_key: '', api_hash: '', clear: true }),
    updateRecovery: autoPort => post('/api/settings/recovery', { auto_port: autoPort }),
    logs: (unit, lines = 150) => get(`/api/logs?unit=${encodeURIComponent(unit)}&lines=${encodeURIComponent(lines)}`),
    diagnostics: () => post('/api/diagnostics'),
    checkUpdates: () => post('/api/update/check'),
    applyUpdate: () => post('/api/update/apply'),
    applyBBR: () => post('/api/bbr'),
    startSpeedTest: () => post('/api/speedtest')
  }
}
