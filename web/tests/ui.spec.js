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
  await page.screenshot({ path: '../docs/images/dashboard.png', fullPage: true })
  expect(errors).toEqual([])
})

test('node configuration and client links are accessible', async ({ page }) => {
  await openDashboard(page)
  await page.getByRole('button', { name: '节点管理', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'VLESS Reality', exact: true })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Hysteria2', exact: true })).toBeVisible()
  await page.getByText('身份与密钥', { exact: true }).click()
  await expect(page.getByLabel('UUID', { exact: true })).toBeVisible()
  await page.screenshot({ path: '../docs/images/nodes.png', fullPage: true })
})

test('settings expose RackNerd and BBR controls', async ({ page }) => {
  await openDashboard(page)
  await page.getByRole('button', { name: '系统设置', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'RackNerd 官方流量' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'BBR 网络优化' })).toBeVisible()
  await expect(page.getByLabel('API Key', { exact: true })).toHaveAttribute('type', 'password')
  await expect(page.getByLabel('API Hash', { exact: true })).toHaveAttribute('type', 'password')
  await page.screenshot({ path: '../docs/images/settings.png', fullPage: true })
})

test('speed test can start and finish', async ({ page }) => {
  await openDashboard(page)
  await page.getByRole('button', { name: '开始测速', exact: true }).click()
  await expect(page.getByRole('button', { name: '测速进行中…' })).toBeDisabled()
  await expect(page.getByRole('button', { name: '开始测速', exact: true })).toBeEnabled({ timeout: 10000 })
})

test('mobile layout does not overflow horizontally', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await openDashboard(page)
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1)
  expect(overflow).toBe(false)
  await page.screenshot({ path: '../docs/images/mobile.png', fullPage: true })
})