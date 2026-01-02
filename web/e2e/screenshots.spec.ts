import { test } from '@playwright/test'

test.describe('Screenshot capture', () => {
  test('capture dashboard', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('networkidle')
    await page.screenshot({ path: 'screenshots/dashboard.png', fullPage: true })
  })

  test('capture orders page', async ({ page }) => {
    await page.goto('/orders')
    await page.waitForLoadState('networkidle')
    await page.screenshot({ path: 'screenshots/orders.png', fullPage: true })
  })

  test('capture sync runs page', async ({ page }) => {
    await page.goto('/runs')
    await page.waitForLoadState('networkidle')
    await page.screenshot({ path: 'screenshots/runs.png', fullPage: true })
  })

  test('capture order detail page', async ({ page }) => {
    // Navigate directly to order detail page
    await page.goto('/orders/200014334161019')
    await page.waitForLoadState('networkidle')
    await page.screenshot({ path: 'screenshots/order-detail.png', fullPage: true })
  })

  test('capture settings page', async ({ page }) => {
    await page.goto('/settings')
    await page.waitForLoadState('networkidle')
    await page.screenshot({ path: 'screenshots/settings.png', fullPage: true })
  })

  test('capture sync page', async ({ page }) => {
    await page.goto('/sync')
    await page.waitForLoadState('networkidle')
    await page.screenshot({ path: 'screenshots/sync.png', fullPage: true })
  })

  test('capture transactions page (synced merchants)', async ({ page }) => {
    await page.goto('/transactions')
    await page.waitForLoadState('networkidle')
    await page.screenshot({ path: 'screenshots/transactions-synced.png', fullPage: true })
  })

  test('capture transactions page (all merchants)', async ({ page }) => {
    await page.goto('/transactions?merchant=all')
    await page.waitForLoadState('networkidle')
    await page.screenshot({ path: 'screenshots/transactions-all.png', fullPage: true })
  })
})
