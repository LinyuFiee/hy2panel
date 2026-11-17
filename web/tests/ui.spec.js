import { test, expect } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

async function resetMock(page) {
  await page.request.get('/api/test/reset')
}

async function openConsole(page, section = 'overview') {
  await resetMock(page)
  await page.goto(`/#/${section}`)
  await expect(page.getByRole('heading', { name: section === 'overview' ? '运行概览' : section === 'traffic' ? '流量监控' : section === 'nodes' ? '节点管理' : section === 'maintenance' ? '维护与诊断' : '系统设置' })).toBeVisible()
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
  const results = await new AxeBuilder({ page }).disableRules(['color-contrast', ...disable]).analyze()
  expect(results.violations.filter(item => ['critical', 'serious'].includes(item.impact))).toEqual([])
}

async function installFigureLifecycleProbe(page, { reducedMotion = false } = {}) {
  await page.addInitScript(({ reduce }) => {
    const nativeMatchMedia = window.matchMedia.bind(window)
    window.matchMedia = query => {
      const result = nativeMatchMedia(query)
      if (query !== '(prefers-reduced-motion: reduce)') return result
      return {
        matches: reduce,
        media: query,
        onchange: null,
        addEventListener: result.addEventListener.bind(result),
        removeEventListener: result.removeEventListener.bind(result),
        addListener: result.addListener?.bind(result),
        removeListener: result.removeListener?.bind(result),
        dispatchEvent: result.dispatchEvent.bind(result)
      }
    }
    window.__figureLifecycle = { requests: 0, cancels: 0, active: new Set() }
    const nativeRequest = window.requestAnimationFrame.bind(window)
    const nativeCancel = window.cancelAnimationFrame.bind(window)
    window.requestAnimationFrame = callback => {
      window.__figureLifecycle.requests += 1
      let id = 0
      id = nativeRequest(time => {
        window.__figureLifecycle.active.delete(id)
        callback(time)
      })
      window.__figureLifecycle.active.add(id)
      return id
    }
    window.cancelAnimationFrame = id => {
      window.__figureLifecycle.cancels += 1
      window.__figureLifecycle.active.delete(id)
      return nativeCancel(id)
    }
    window.__figureIntersections = new Map()
    window.IntersectionObserver = class {
      constructor(callback) { this.callback = callback; this.targets = new Set() }
      observe(target) { this.targets.add(target); window.__figureIntersections.set(target, this); queueMicrotask(() => this.targets.has(target) && this.callback([{ target, isIntersecting: true, intersectionRatio: 1 }], this)) }
      unobserve(target) { this.targets.delete(target); window.__figureIntersections.delete(target) }
      disconnect() { for (const target of this.targets) window.__figureIntersections.delete(target); this.targets.clear() }
    }
    window.__setFigureIntersection = (target, visible) => {
      const observer = window.__figureIntersections.get(target)
      if (!observer) return false
      observer.callback([{ target, isIntersecting: visible, intersectionRatio: visible ? 1 : 0 }], observer)
      return true
    }
  }, { reduce: reducedMotion })
}

test('overview shell, theme and server rack render without runtime errors', async ({ page }, testInfo) => {
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  await openConsole(page)
  await expect(page.getByRole('navigation', { name: '控制台分区' })).toBeVisible()
  await expect(page.getByRole('heading', { name: '运行概览' })).toBeVisible()
  await expect(page.getByText('节点协议', { exact: true })).toBeVisible()
  const figure = page.locator('[data-figure="server-rack-flow"]')
  await expect(figure).toHaveAttribute('data-rack-units', '6')
  await expect(figure).toHaveAttribute('data-rack-ports', '8')
  await expect(figure).toHaveAttribute('data-route-order', 'ingress-port-blade-egress')
  await expect(figure).toHaveAttribute('role', 'img')
  await expect(figure.locator('svg')).toHaveAttribute('aria-hidden', 'true')
  await expect(page.getByText(/小路由器|router|antenna/i)).toHaveCount(0)
  await page.getByRole('button', { name: '深色' }).click()
  await expect(page.locator('html')).toHaveClass(/dark/)
  await expectNoSeriousA11yViolations(page)
  await page.screenshot({ path: testInfo.outputPath('overview.png'), fullPage: true })
  expect(errors).toEqual([])
})

test('server rack respects reduced motion and lifecycle cleanup', async ({ page }) => {
  await installFigureLifecycleProbe(page, { reducedMotion: true })
  await page.route('**/api/state', route => route.fulfill({ status: 401, contentType: 'application/json', body: '{"error":"请先登录"}' }))
  await page.goto('/')
  const figure = page.locator('[data-figure="server-rack-flow"]')
  await expect(figure).toBeVisible()
  await expect(figure).toHaveAttribute('data-animation-state', 'reduced-motion')
  const restingProgress = await figure.getAttribute('data-progress')
  await page.waitForTimeout(150)
  await expect(figure).toHaveAttribute('data-progress', restingProgress)
  expect(await page.evaluate(() => window.__figureLifecycle.requests)).toBe(0)
  const box = await figure.boundingBox()
  await page.mouse.move(box.x + box.width * 0.82, box.y + box.height * 0.75)
  await expect.poll(async () => Number(await figure.getAttribute('data-progress'))).toBeGreaterThan(0.65)
  await page.unroute('**/api/state')
})

test('nodes use progressive settings and on-demand credential dialogs', async ({ page }, testInfo) => {
  await openConsole(page, 'nodes')
  await expect(page.getByRole('heading', { name: '节点公网 Endpoint' })).toBeVisible()
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

test('maintenance exposes manual tabs, diagnostics and recovery policy', async ({ page }, testInfo) => {
  await openConsole(page, 'maintenance')
  await expect(page.getByRole('heading', { name: '恢复、诊断与日志，一处完成。' })).toBeVisible()
  await expect(page.getByRole('heading', { name: '服务端问题自检' })).toBeVisible()
  await page.getByRole('button', { name: '开始自检' }).click()
  await expect(page.getByRole('heading', { name: '检查结果与修复建议' })).toBeVisible()
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
  await expect(page.getByText('访问安全', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: '检测更新' }).click()
  await expect(page.getByText('v0.6.0', { exact: true }).first()).toBeVisible()
  await page.getByRole('button', { name: '立即更新' }).click()
  await page.getByRole('button', { name: '确认更新' }).click()
  await expectNoSeriousA11yViolations(page, { disable: ['aria-hidden-focus'] })
  await page.screenshot({ path: testInfo.outputPath('settings.png'), fullPage: true })
})

test('mobile sheet exposes every workspace and logout without overflow', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await openConsole(page)
  await page.getByRole('button', { name: '打开导航菜单' }).click()
  const sheet = page.getByRole('dialog')
  for (const label of ['概览', '节点', '流量', '维护', '设置']) await expect(sheet.getByText(label, { exact: true })).toBeVisible()
  await expect(sheet.getByRole('button', { name: '退出登录' })).toBeVisible()
  await sheet.getByText('流量', { exact: true }).click()
  await expect(page.getByRole('heading', { name: '流量监控' })).toBeVisible()
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: testInfo.outputPath('mobile.png'), fullPage: true })
})
