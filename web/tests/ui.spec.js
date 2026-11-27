import { test, expect } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

const runtimeIssues = new WeakMap()

test.beforeEach(async ({ page }) => {
  const issues = []
  runtimeIssues.set(page, issues)
  page.on('pageerror', error => issues.push(`pageerror: ${error.message}`))
  page.on('console', message => {
    if (message.type() === 'error' || message.type() === 'warning') {
      issues.push(`console.${message.type()}: ${message.text()}`)
    }
  })
})

test.afterEach(async ({ page }) => {
  expect(runtimeIssues.get(page) || []).toEqual([])
})

async function resetMock(page) {
  await page.request.get('/api/test/reset')
}

async function openConsole(page, section = 'overview') {
  await resetMock(page)
  await page.goto(`/#/${section}`)
  const titles = { overview: '运行概览', traffic: '流量监控', nodes: '节点管理', maintenance: '维护与诊断', settings: '系统设置' }
  await expect(page.getByRole('banner').getByRole('heading', { level: 1, name: titles[section], exact: true })).toBeVisible()
  await expect(page.getByRole('main').getByRole('heading', { level: 1 })).toHaveCount(0)
}

async function openLogin(page) {
  await resetMock(page)
  await page.route('**/api/state', route => route.fulfill({ status: 401, contentType: 'application/json', body: '{"error":"请先登录"}' }), { times: 1 })
  await page.goto('/')
  await expect(page.getByRole('heading', { name: '管理员登录' })).toBeVisible()
  runtimeIssues.get(page)?.splice(0)
}

async function expectNoHorizontalOverflow(page) {
  const dimensions = await page.evaluate(() => ({
    viewport: window.innerWidth,
    document: document.documentElement.scrollWidth,
    body: document.body.scrollWidth
  }))
  expect(dimensions.document).toBeLessThanOrEqual(dimensions.viewport + 1)
  expect(dimensions.body).toBeLessThanOrEqual(dimensions.viewport + 1)
}

async function expectNoSeriousA11yViolations(page, { disable = [] } = {}) {
  const builder = new AxeBuilder({ page })
  if (disable.length) builder.disableRules(disable)
  const results = await builder.analyze()
  expect(results.violations.filter(item => ['critical', 'serious'].includes(item.impact))).toEqual([])
}

test('login uses the direct Hairline padlock and submits credentials', async ({ page }, testInfo) => {
  await openLogin(page)
  const figure = page.locator('[data-hairline="padlock"]')
  await expect(figure).toBeVisible()
  await expect(figure).toHaveAttribute('role', 'img')
  await expect(figure).toHaveAttribute('aria-label', /交互式安全插图/)
  await expect(figure.locator('svg')).toHaveAttribute('aria-hidden', 'true')
  await expect(page.getByLabel('管理员密码')).toBeFocused()
  await page.screenshot({ path: testInfo.outputPath('login.png'), fullPage: true })
  await page.getByLabel('管理员密码').fill('mock-password')
  await page.getByRole('button', { name: '登录' }).click()
  await expect(page.getByRole('heading', { name: '运行概览' })).toBeVisible()
  await expectNoSeriousA11yViolations(page)
})

test('connection overview keeps HY2 checks separate from device connectivity', async ({ page }, testInfo) => {
  await openConsole(page)
  await expect(page.getByRole('button', { name: '创建并复制订阅' })).toBeEnabled()
  await expect(page.getByRole('heading', { name: 'HY2 服务端检查' })).toHaveCount(0)
  await expect(page.locator('[data-hairline], [data-figure="node-flow"]')).toHaveCount(0)
  await page.getByRole('button', { name: '深色' }).click()
  await expect(page.locator('meta[name="theme-color"]')).toHaveAttribute('content', '#0b0b0c')
  await expectNoSeriousA11yViolations(page)
  await page.screenshot({ path: testInfo.outputPath('overview.png'), fullPage: true })
  await page.getByRole('button', { name: '诊断 HY2' }).click()
  await expect(page.getByRole('heading', { name: 'HY2 服务端检查' })).toBeVisible()
  await expect(page.getByText('UDP 443 正在监听')).toBeVisible()
  await expect(page.getByText('模拟提醒：请核对解析记录')).toBeVisible()
  await expect(page.getByText(/设备连接未测试/).first()).toBeVisible()
})

test('packet illustration moves, pauses, and respects reduced motion and disabled protocols', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'no-preference' })
  await page.route('**/api/state', async route => {
    const response = await route.fetch()
    const state = await response.json()
    state.config.hysteria2.enabled = false
    await route.fulfill({ response, json: state })
  })
  await openConsole(page)
  const flow = page.getByRole('listitem').filter({ hasText: 'Reality' }).locator('.packet-flow')
  const outbound = flow.locator('.outgoing')
  const offset = () => outbound.evaluate(path => getComputedStyle(path).strokeDashoffset)
  await expect(flow.locator('.packet')).toHaveCount(2)
  await expect(page.getByRole('listitem').filter({ hasText: 'Hysteria2' }).locator('.packet')).toHaveCount(0)
  await expect(outbound).toHaveCSS('animation-play-state', 'running')
  const start = await offset()
  await expect.poll(offset).not.toBe(start)
  const firstFrame = await flow.screenshot()
  await expect.poll(async () => firstFrame.equals(await flow.screenshot())).toBe(false)
  await page.getByRole('button', { name: '暂停传输示意' }).click()
  await expect(outbound).toHaveCSS('animation-play-state', 'paused')
  const pausedFrame = await flow.screenshot()
  expect(pausedFrame.equals(await flow.screenshot())).toBe(true)
  await page.getByRole('button', { name: '播放传输示意' }).press('Enter')
  await expect(outbound).toHaveCSS('animation-play-state', 'running')
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await expect(outbound).toHaveCSS('animation-name', 'none')
  const reducedFrame = await flow.screenshot()
  expect(reducedFrame.equals(await flow.screenshot())).toBe(true)
  await expect(page.getByRole('button', { name: '暂停传输示意' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '创建并复制订阅' })).toBeEnabled()
})

test('subscription is opt-in, repeatable, rotatable and revocable', async ({ page }) => {
  await page.addInitScript(() => Object.defineProperty(navigator, 'clipboard', {
    value: { writeText: async value => { window.__copiedSubscription = value } }
  }))
  await openConsole(page)
  const create = page.getByRole('button', { name: '创建并复制订阅' })
  const copy = page.getByRole('button', { name: '复制订阅', exact: true })
  await create.click()
  await expect.poll(() => page.evaluate(() => window.__copiedSubscription)).toContain('/api/sub/')
  const first = await page.evaluate(() => window.__copiedSubscription)
  await page.reload()
  await copy.click()
  await expect.poll(() => page.evaluate(() => window.__copiedSubscription)).toBe(first)
  await page.getByText('订阅管理', { exact: true }).click()
  await page.getByRole('button', { name: '重置订阅', exact: true }).click()
  await page.getByRole('button', { name: '确认重置' }).click()
  await copy.click()
  await expect.poll(() => page.evaluate(() => window.__copiedSubscription)).not.toBe(first)
  await page.getByRole('button', { name: '撤销订阅', exact: true }).click()
  await page.getByRole('button', { name: '确认撤销' }).click()
  await expect(create).toBeEnabled()
  await expect(page.getByText(first, { exact: true })).toHaveCount(0)
})

test('manual subscription copy is only visible while its dialog is open', async ({ page }) => {
  await page.addInitScript(() => Object.defineProperty(navigator, 'clipboard', {
    value: { writeText: async () => { throw new Error('clipboard unavailable') } }
  }))
  await openConsole(page)
  await page.getByRole('button', { name: '创建并复制订阅' }).click()
  const dialog = page.getByRole('dialog', { name: '手动复制订阅' })
  await expect(dialog.getByRole('textbox', { name: '订阅地址' })).toHaveValue(/^https:\/\/panel\.example\.com:8443\/api\/sub\//)
  await dialog.getByRole('button', { name: '关闭' }).click()
  await expect(page.getByRole('textbox', { name: '订阅地址' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '复制订阅', exact: true })).toBeEnabled()
})

test('a changed node cannot keep a previous diagnostic report', async ({ page }) => {
  await openConsole(page)
  await page.getByRole('button', { name: '诊断 HY2' }).click()
  await expect(page.getByRole('heading', { name: 'HY2 服务端检查' })).toBeVisible()
  await page.request.post('/api/config', { data: { hysteria2: { port: 24443 } } })
  await expect(page.getByText('节点配置已更改，请重新诊断。')).toBeVisible({ timeout: 8000 })
  await expect(page.getByRole('heading', { name: 'HY2 服务端检查' })).toHaveCount(0)
  await page.getByRole('button', { name: '诊断 HY2' }).click()
  await expect(page.getByText('UDP 24443 正在监听')).toBeVisible()
})

for (const width of [320, 390, 768, 1024, 1440]) {
  test(`overview actions fit at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 844 })
    await openConsole(page)
    const subscribe = page.getByRole('button', { name: '创建并复制订阅' })
    await expect(subscribe).toBeInViewport()
    await expect(page.getByRole('button', { name: '诊断 HY2' })).toBeInViewport()
    const protocols = await page.getByRole('list', { name: '节点协议' }).boundingBox()
    const resources = await page.getByRole('region', { name: '主机资源' }).boundingBox()
    expect((await subscribe.boundingBox()).y).toBeLessThan(protocols.y)
    if (width > 1080) expect(resources.x).toBeGreaterThanOrEqual(protocols.x + protocols.width)
    else expect(resources.y).toBeGreaterThanOrEqual(protocols.y + protocols.height)
    await expectNoHorizontalOverflow(page)
    await page.screenshot({ path: testInfo.outputPath('overview.png') })
  })
}

test('nodes use progressive settings and on-demand credential dialogs', async ({ page }, testInfo) => {
  await openConsole(page, 'nodes')
  await expect(page.getByText('节点公网地址', { exact: true })).toBeVisible()
  await expect(page.getByRole('heading', { name: '🇺🇸 US-Reality' })).toBeVisible()
  await expect(page.getByRole('heading', { name: '🇺🇸 US-Hysteria2' })).toBeVisible()
  await page.getByRole('button', { name: /高级密钥与身份/ }).click()
  await expect(page.getByRole('textbox', { name: 'Reality 私钥' })).toHaveAttribute('type', 'password')
  await expect(page.getByLabel('UUID')).toBeVisible()
  await page.getByRole('button', { name: /打开 .*Reality.*客户端凭据/ }).click()
  await expect(page.getByRole('dialog')).toContainText('客户端凭据')
  await expect(page.getByLabel(/Reality.*客户端链接/)).toHaveValue(/^vless:\/\//)
  await expect(page.getByRole('img', { name: /Reality.*二维码/ })).toBeVisible()
  await page.getByRole('button', { name: '复制链接' }).click()
  await expect(page.getByRole('button', { name: '已复制' })).toBeVisible()
  await page.getByRole('button', { name: '关闭' }).first().click()
  await expectNoSeriousA11yViolations(page)
  await page.screenshot({ path: testInfo.outputPath('nodes.png'), fullPage: true })
})

test('traffic workspace keeps protocol gaps and speedtest states', async ({ page }, testInfo) => {
  await openConsole(page, 'traffic')
  await expect(page.getByRole('heading', { name: '协议实时' })).toBeVisible()
  await expect(page.getByTestId('reality-traffic-card')).toContainText('Reality')
  await expect(page.getByTestId('unattributed-traffic')).toContainText('97.3%')
  await expect(page.getByTestId('protocol-traffic-chart')).toBeVisible()
  await expect(page.locator('.protocol-gap')).toHaveCount(2)
  await page.getByRole('button', { name: '6 小时' }).click()
  await expect(page.getByRole('button', { name: '6 小时' })).toHaveAttribute('data-state', 'on')
  await page.getByRole('button', { name: '开始测速' }).click()
  await expect(page.getByText('正在测量服务器链路')).toBeVisible()
  await expect(page.getByRole('button', { name: '开始测速' })).toBeEnabled({ timeout: 10000 })
  await page.screenshot({ path: testInfo.outputPath('traffic.png'), fullPage: true })
})

test('failed VPS test retains the last successful result', async ({ page }) => {
  const speedtest = {
    running: false, error: '测试站暂不可用',
    result: { timestamp: '2026-11-07T10:00:00Z', server_name: 'Previous test station', download_mbps: 123, upload_mbps: 45 }
  }
  for (const endpoint of ['**/api/state', '**/api/system/status']) {
    await page.route(endpoint, async route => {
      const response = await route.fetch()
      await route.fulfill({ response, json: { ...await response.json(), speedtest } })
    })
  }
  await openConsole(page, 'traffic')
  const card = page.getByRole('article').filter({ has: page.getByRole('heading', { name: 'VPS 到测速站' }) })
  await expect(card.getByText('最近一次失败')).toBeVisible()
  await expect(card.getByText(/上次成功：.*Previous test station/)).toBeVisible()
  await expect(card.getByText('测试站暂不可用')).toBeVisible()
})

test('maintenance exposes manual tabs, diagnostics and recovery policy', async ({ page }, testInfo) => {
  await openConsole(page, 'maintenance')
  await expect(page.getByRole('heading', { name: '恢复策略' })).toBeVisible()
  await expect(page.getByRole('heading', { name: '服务端自检' })).toBeVisible()
  await page.getByRole('button', { name: '开始自检' }).click()
  await expect(page.getByRole('heading', { name: '检查结果' })).toBeVisible()
  const tabs = page.getByRole('tablist', { name: '日志单元' })
  await expect(tabs).toBeVisible()
  await page.getByRole('tab', { name: '面板' }).click()
  await expect(page.getByRole('log')).toBeVisible()
  const recovery = page.getByLabel('连续异常后自动换端口')
  await recovery.setChecked(true, { force: true })
  await expect(recovery).toBeChecked()
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: testInfo.outputPath('maintenance.png'), fullPage: true })
})

test('settings use accessible confirmation dialogs and operation states', async ({ page }, testInfo) => {
  await openConsole(page, 'settings')
  const core = page.getByTestId('sing-box-version-card')
  await expect(core).toContainText('v1.14.4')
  await page.getByRole('button', { name: '切换核心' }).click()
  await expect(page.getByRole('alertdialog')).toContainText('切换到 sing-box v1.14.4')
  await page.getByRole('button', { name: '确认切换' }).click()
  await expect(core).toContainText('sing-box 已运行 v1.14.4', { timeout: 8000 })
  await page.getByRole('button', { name: '回滚上一版本' }).click()
  await page.getByRole('button', { name: '确认回滚' }).click()
  await expect(core).toContainText('sing-box 已运行 v1.14.3', { timeout: 8000 })
  await expect(page.getByText('仅通过 HTTPS 访问')).toBeVisible()
  await page.getByRole('button', { name: '检测更新' }).click()
  await expect(page.getByText('v0.6.1', { exact: true }).first()).toBeVisible()
  await page.getByRole('button', { name: '立即更新' }).click()
  await page.getByRole('button', { name: '确认更新' }).click()
  await expect(page.getByRole('alertdialog')).toHaveCount(0)
  await expectNoSeriousA11yViolations(page, { disable: ['aria-hidden-focus'] })
  await page.screenshot({ path: testInfo.outputPath('settings.png'), fullPage: true })
})

test('mobile bottom tab bar exposes every workspace and logout without overflow', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await openConsole(page)
  const tabbar = page.getByRole('navigation', { name: '底部导航' })
  await expect(tabbar).toBeVisible()
  for (const label of ['概览', '节点', '流量', '维护', '设置']) await expect(tabbar.getByText(label, { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: '退出登录' })).toBeVisible()
  await tabbar.getByText('流量', { exact: true }).click()
  await expect(page.getByRole('heading', { name: '流量监控' })).toBeVisible()
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: testInfo.outputPath('mobile.png'), fullPage: true })
})
