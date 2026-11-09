import { test, expect } from '@playwright/test'

async function openDashboard(page) {
  await page.goto('/')
  await expect(page.getByRole('heading', { name: '让连接轻松一点。' })).toBeVisible()
}

test('dashboard renders metrics and Hairline without runtime errors', async ({ page }) => {
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  await openDashboard(page)
  await expect(page.getByRole('heading', { name: '本月流量' })).toBeVisible()
  await expect(page.getByRole('heading', { name: '实时网络' })).toBeVisible()
  await expect(page.getByRole('heading', { name: '服务器测速' })).toBeVisible()
  await expect(page.getByText('已启用', { exact: true })).toBeVisible()
  await expect(page.locator('.hairline-figure svg')).toBeVisible()
  await expect(page.locator('.download-line')).toHaveAttribute('d', /L/)
  await expect(page.locator('.sidebar-brand .logo-image')).toHaveAttribute('src', '/logo.png')
  await expect(page.locator('.sidebar-brand .logo-image')).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)')
  expect(await page.locator('.sidebar-brand .logo-image').evaluate(image => image.naturalWidth > 0)).toBe(true)
  expect((await page.request.get('/logo.png')).status()).toBe(200)
  const githubLink = page.locator('.sidebar-bottom .github-link')
  await expect(githubLink).toHaveAttribute('href', 'https://github.com/LinyuFiee/hy2panel')
  await expect(githubLink).toHaveAttribute('target', '_blank')
  await expect(githubLink).toHaveAttribute('rel', 'noopener noreferrer')
  await page.screenshot({ path: '../docs/images/dashboard.png', fullPage: true })
})

async function qrCodesReady(page) {
  return page.locator('.qr-wrap img').evaluateAll(images => images.every(image => image.complete && image.naturalWidth > 0))
}

test('node configuration and client links are accessible', async ({ page }) => {
  await openDashboard(page)
  await page.getByRole('button', { name: '节点管理', exact: true }).click()
  await expect(page.getByRole('heading', { name: '🇺🇸 US-Reality', exact: true })).toBeVisible()
  await expect(page.getByRole('heading', { name: '🇺🇸 US-Hysteria2', exact: true })).toBeVisible()
  await expect(page.locator('.node-card details')).toHaveCount(0)
  await expect(page.getByLabel('UUID', { exact: true })).toBeVisible()
  await expect(page.getByLabel('Reality 私钥', { exact: true })).toHaveAttribute('type', 'password')
  await expect(page.getByLabel('连接密码', { exact: true })).toBeVisible()
  await expect(page.getByLabel('Reality 客户端链接', { exact: true })).toHaveValue(/^vless:\/\//)
  await expect(page.getByLabel('Hysteria2 客户端链接', { exact: true })).toHaveValue(/^hysteria2:\/\//)
  const qrCodes = page.locator('.qr-wrap img')
  await expect(qrCodes).toHaveCount(2)
  await expect(qrCodes.first()).toBeVisible()
  await expect(qrCodes.nth(1)).toBeVisible()
  expect(await qrCodesReady(page)).toBe(true)
  await page.getByRole('button', { name: '复制链接', exact: true }).first().click()
  await expect(page.getByRole('button', { name: '已复制', exact: true })).toBeVisible()
  await page.screenshot({ path: '../docs/images/nodes.png', fullPage: true })
  await page.setViewportSize({ width: 390, height: 844 })
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1)
  expect(overflow).toBe(false)
})

test('settings expose RackNerd and BBR controls', async ({ page }) => {
  await openDashboard(page)
  await page.getByRole('button', { name: '系统设置', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'RackNerd 官方流量' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'BBR 网络优化' })).toBeVisible()
  await expect(page.getByLabel('API Key', { exact: true })).toHaveAttribute('type', 'password')
  await expect(page.getByLabel('API Hash', { exact: true })).toHaveAttribute('type', 'password')
  await expect(page.getByRole('heading', { name: '面板更新' })).toBeVisible()
  await page.getByRole('button', { name: '检测更新', exact: true }).click()
  await expect(page.getByText('发现新版本', { exact: true })).toBeVisible()
  await expect(page.getByText('v0.5.0', { exact: true })).toBeVisible()
  page.once('dialog', dialog => dialog.accept())
  await page.getByRole('button', { name: '立即更新', exact: true }).click()
  await expect(page.locator('.update-card a[href="https://github.com/LinyuFiee/hy2panel"]')).toHaveAttribute('target', '_blank')
  await page.screenshot({ path: '../docs/images/settings.png', fullPage: true })
})

test('speed test can start and finish', async ({ page }) => {
  await openDashboard(page)
  await page.getByRole('button', { name: '开始测速', exact: true }).click()
  await expect(page.getByRole('button', { name: '测速进行中…' })).toBeDisabled()
  await expect(page.locator('.speed-live')).toBeVisible()
  await expect(page.locator('.speed-live strong').first()).toContainText('512.4')
  await expect(page.getByRole('button', { name: '开始测速', exact: true })).toBeEnabled({ timeout: 10000 })
  await expect(page.locator('.speed-results strong').first()).toContainText('683')
})

test('maintenance shows diagnostics, logs and recovery controls', async ({ page }) => {
  await openDashboard(page)
  await page.getByRole('button', { name: '日志与自检', exact: true }).click()
  await expect(page.getByRole('heading', { name: '服务端问题自检' })).toBeVisible()
  await expect(page.locator('.log-line')).toHaveCount(3)
  await page.getByRole('button', { name: '开始自检', exact: true }).click()
  await expect(page.getByRole('heading', { name: '检查结果与修复建议' })).toBeVisible()
  await expect(page.getByText('sing-box.service 正在运行')).toBeVisible()
  const recoveryToggle = page.locator('.recovery-row .toggle input')
  await recoveryToggle.check()
  await expect(recoveryToggle).toBeChecked()
  await page.screenshot({ path: '../docs/images/maintenance.png', fullPage: true })
})

test('mobile layout does not overflow horizontally', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await openDashboard(page)
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1)
  expect(overflow).toBe(false)
  await page.screenshot({ path: '../docs/images/mobile.png', fullPage: true })
})