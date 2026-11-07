import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  workers: 1,
  timeout: 30000,
  use: {
    baseURL: 'http://127.0.0.1:5199',
    viewport: { width: 1440, height: 1000 },
    headless: true,
    channel: process.env.PLAYWRIGHT_CHANNEL || undefined
  },
  webServer: {
    command: 'npm run dev -- --port 5199',
    url: 'http://127.0.0.1:5199',
    reuseExistingServer: true,
    timeout: 30000,
    env: { MOCK_UI: '1' }
  }
})