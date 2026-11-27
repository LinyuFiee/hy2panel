import { expect, test } from '@playwright/test'

// Each page owns its state and save responses; never reset or write to the shared Vite mock.
async function openDraft(page, hysteria = {}) {
  const state = {
    config: {
      panel: { public_host: 'node.example.com' },
      reality: { enabled: false },
      hysteria2: { enabled: true, port: 443, domain: 'hy2.example.com', password: 'test-auth', up_mbps: 75, down_mbps: 250, ...hysteria }
    },
    status: { sing_box: 'active' },
    links: {}
  }
  const saves = []
  await page.route('**/api/state', route => route.fulfill({ json: state }))
  await page.route('**/api/config', route => {
    const payload = route.request().postDataJSON()
    saves.push(payload)
    state.config.hysteria2 = payload.hysteria2
    return route.fulfill({ json: state })
  })
  await page.goto('/#/nodes')
  const advanced = page.getByRole('button', { name: /高级认证与带宽/ })
  await advanced.click()
  return {
    state, saves, advanced,
    save: page.getByRole('button', { name: '保存并应用' }),
    obfs: page.getByRole('combobox', { name: 'Hysteria2 QUIC 混淆' }),
    password: page.getByLabel('混淆密码', { exact: true }),
    unlimited: page.getByRole('switch', { name: 'Hysteria2 不限速' }),
    up: page.getByRole('spinbutton', { name: '服务器上传 Mbps' }),
    down: page.getByRole('spinbutton', { name: '服务器下载 Mbps' })
  }
}

async function revisitNodes(page, advanced) {
  const nav = page.getByRole('navigation', { name: '控制台分区' })
  await nav.getByRole('link', { name: '概览' }).click()
  await expect(advanced).toHaveCount(0)
  await nav.getByRole('link', { name: '节点' }).click()
  await advanced.click()
}

async function refreshState(page) {
  const refreshed = page.waitForResponse('**/api/state')
  await page.evaluate(() => document.dispatchEvent(new Event('visibilitychange')))
  await refreshed
}

test('restores the password for the previous obfs type, but saves no secret when off', async ({ page }) => {
  const original = 'fixture-salamander-password'
  const edited = 'fixture-gecko-edit'
  const { obfs, password, advanced, save, saves } = await openDraft(page, { obfs_type: 'salamander', obfs_password: original })
  await obfs.selectOption('')
  await expect(password).toHaveCount(0)
  await revisitNodes(page, advanced)
  await obfs.selectOption('salamander')
  await expect(password).toHaveValue(original)
  await expect(save).toBeDisabled()

  await obfs.selectOption('gecko')
  await password.fill(edited)
  await obfs.selectOption('')
  await obfs.selectOption('salamander')
  await expect(password).toHaveValue(original)
  await obfs.selectOption('gecko')
  await expect(password).toHaveValue(edited)
  await obfs.selectOption('')
  const storage = await page.evaluate(() => JSON.stringify([localStorage, sessionStorage]))
  expect(storage).not.toContain(original)
  expect(storage).not.toContain(edited)
  await save.click()
  await expect(save).toBeDisabled()
  expect(saves).toHaveLength(1)
  expect(saves[0].hysteria2.obfs_type).toBe('')
  expect(saves[0].hysteria2.obfs_password ?? '').toBe('')
  expect(JSON.stringify(saves)).not.toContain(edited)
  expect(JSON.stringify(saves)).not.toContain(original)
  await obfs.selectOption('gecko')
  await expect(password).toHaveValue(/.+/)
  await expect(password).not.toHaveValue(edited)
})

test('unlimited remembers asymmetric rates across clean polling and navigation, not after save', async ({ page }) => {
  const { up, down, unlimited, advanced, save, saves } = await openDraft(page, { up_mbps: 0, down_mbps: 0 })
  await unlimited.click()
  await up.fill('640')
  await down.fill('0')
  await unlimited.click()
  await expect(up).toHaveValue('0')
  await expect(down).toBeDisabled()
  await expect(save).toBeDisabled()
  await refreshState(page)
  await revisitNodes(page, advanced)
  await unlimited.click()
  await expect(up).toHaveValue('640')
  await expect(down).toHaveValue('0')
  await save.click()
  await expect(save).toBeDisabled()
  expect(saves[0].hysteria2).toMatchObject({ up_mbps: 640, down_mbps: 0 })

  await unlimited.click()
  await save.press('Enter')
  await expect(save).toBeDisabled()
  expect(saves[1].hysteria2).toMatchObject({ up_mbps: 0, down_mbps: 0 })
  await unlimited.click()
  await expect(up).toHaveValue('100')
  await expect(down).toHaveValue('100')
})

for (const [direction, key] of [['up', 'up_mbps'], ['down', 'down_mbps']]) {
  test(`collapsed blank ${direction} rate blocks save and focuses the field; explicit zero saves`, async ({ page }) => {
    if (direction === 'down') await page.setViewportSize({ width: 390, height: 844 })
    const ui = await openDraft(page)
    const rate = ui[direction]
    await rate.fill('')
    await ui.advanced.click()
    await expect(rate).not.toBeVisible()
    await ui.save.click()
    await expect(ui.advanced).toHaveAttribute('aria-expanded', 'true')
    await expect(rate).toBeFocused()
    await expect(rate).toHaveValue('')
    await expect(rate).toHaveAttribute('aria-invalid', 'true')
    await expect(rate).toHaveAccessibleDescription(/不限速请填 0/)
    expect(ui.saves).toHaveLength(0)
    await rate.fill('0')
    await ui.advanced.click()
    await ui.save.click()
    await expect(ui.save).toBeDisabled()
    expect(ui.saves).toHaveLength(1)
    expect(ui.saves[0].hysteria2[key]).toBe(0)
  })
}

test('a remote conflict discards toggle history even while the nodes view is closed', async ({ page }) => {
  const secret = 'fixture-conflict-password'
  const ui = await openDraft(page, { obfs_type: 'gecko', obfs_password: secret })
  await ui.obfs.selectOption('')
  await ui.unlimited.click()
  const nav = page.getByRole('navigation', { name: '控制台分区' })
  await nav.getByRole('link', { name: '概览' }).click()
  ui.state.config.hysteria2.port = 8443
  await refreshState(page)
  await nav.getByRole('link', { name: '节点' }).click()
  await expect(page.getByRole('button', { name: '存在冲突' })).toBeDisabled()
  ui.state.config.hysteria2.port = 443
  await refreshState(page)
  await expect(ui.save).toBeEnabled()
  await ui.advanced.click()
  await ui.obfs.selectOption('gecko')
  await expect(ui.password).toHaveValue(/.+/)
  await expect(ui.password).not.toHaveValue(secret)
  await ui.unlimited.click()
  await expect(ui.up).toHaveValue('100')
  await expect(ui.down).toHaveValue('100')
  expect(ui.saves).toHaveLength(0)
})

for (const boundary of ['replacement', 'logout']) {
  test(`a clean draft drops remembered secrets and rates on ${boundary}`, async ({ page }) => {
    const ui = await openDraft(page, { up_mbps: 0, down_mbps: 0 })
    await ui.obfs.selectOption('salamander')
    await ui.password.fill('discard-this-draft')
    await ui.obfs.selectOption('')
    await ui.unlimited.click()
    await ui.down.fill('810')
    await ui.unlimited.click()
    await expect(ui.save).toBeDisabled()

    if (boundary === 'logout') {
      await page.route('**/api/logout', route => route.fulfill({ json: {} }))
      await page.route('**/api/login', route => route.fulfill({ json: {} }))
      await page.getByRole('button', { name: '退出登录' }).click()
      await page.getByLabel('管理员密码').fill('test-login')
      await page.getByRole('button', { name: '登录' }).click()
      await ui.advanced.click()
    } else {
      ui.state.config.hysteria2.port = 9443
      await refreshState(page)
      await expect(page.getByLabel('UDP 端口')).toHaveValue('9443')
    }
    await ui.obfs.selectOption('salamander')
    await expect(ui.password).toHaveValue(/.+/)
    await expect(ui.password).not.toHaveValue('discard-this-draft')
    await ui.unlimited.click()
    await expect(ui.down).toHaveValue('100')
    expect(ui.saves).toHaveLength(0)
  })
}
