import { test, expect } from '@playwright/test'

test.describe('Navigation', () => {
  test('should load dashboard page', async ({ page }) => {
    await page.goto('/')
    await expect(page).toHaveTitle(/Retail Sync/)
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible()
  })

  test('should navigate to orders page', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Orders' }).click()
    await expect(page).toHaveURL(/\/orders/)
    await expect(page.getByRole('heading', { name: 'Orders' })).toBeVisible()
  })

  test('should navigate to sync runs page', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Sync Runs' }).click()
    await expect(page).toHaveURL(/\/runs/)
    await expect(page.getByRole('heading', { name: 'Sync Runs' })).toBeVisible()
  })

  test('should navigate to quick start page', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Quick Start' }).click()
    await expect(page).toHaveURL(/\/settings/)
    await expect(page.getByRole('heading', { name: 'Quick Start' })).toBeVisible()
  })

  test('should show sidebar with navigation items', async ({ page }) => {
    await page.goto('/')

    // Check sidebar is visible
    await expect(page.getByText('Retail Sync').first()).toBeVisible()

    // Check all nav items - use first() to handle mobile/desktop duplicate links
    await expect(page.getByRole('link', { name: 'Dashboard' }).first()).toBeVisible()
    await expect(page.getByRole('link', { name: 'Sync', exact: true }).first()).toBeVisible()
    await expect(page.getByRole('link', { name: 'Orders' }).first()).toBeVisible()
    await expect(page.getByRole('link', { name: 'Transactions' }).first()).toBeVisible()
    await expect(page.getByRole('link', { name: 'Sync Runs' }).first()).toBeVisible()
    await expect(page.getByRole('link', { name: 'Quick Start' }).first()).toBeVisible()
  })
})

test.describe('Dashboard', () => {
  test('should show stats section', async ({ page }) => {
    await page.goto('/')

    // Check for stat titles - use exact match to avoid matching status badges
    await expect(page.getByText('Total Orders', { exact: true })).toBeVisible()
    await expect(page.getByText('Successful', { exact: true })).toBeVisible()
    await expect(page.getByText('Failed', { exact: true })).toBeVisible()
    await expect(page.getByText('Total Synced', { exact: true })).toBeVisible()
  })

  test('should show recent orders section', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByRole('heading', { name: 'Recent Orders' })).toBeVisible()
  })

  test('should show recent sync runs section', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByRole('heading', { name: 'Recent Sync Runs' })).toBeVisible()
  })
})

test.describe('Orders Page', () => {
  test('should show filter dropdowns', async ({ page }) => {
    await page.goto('/orders')

    // Check for filter selects (native HTML select elements)
    await expect(page.locator('select[name="provider"]')).toBeVisible()
    await expect(page.locator('select[name="status"]')).toBeVisible()
  })

  test('should have table headers', async ({ page }) => {
    await page.goto('/orders')

    await expect(page.getByRole('columnheader', { name: 'Order ID' })).toBeVisible()
    await expect(page.getByRole('columnheader', { name: 'Provider' })).toBeVisible()
    await expect(page.getByRole('columnheader', { name: 'Date' })).toBeVisible()
    await expect(page.getByRole('columnheader', { name: 'Status' })).toBeVisible()
    await expect(page.getByRole('columnheader', { name: 'Total' })).toBeVisible()
  })
})

test.describe('Sync Runs Page', () => {
  test('should load sync runs page', async ({ page }) => {
    await page.goto('/runs')
    await expect(page.getByRole('heading', { name: 'Sync Runs' })).toBeVisible()
  })

  test('should have table headers', async ({ page }) => {
    await page.goto('/runs')

    await expect(page.getByRole('columnheader', { name: 'Run ID' })).toBeVisible()
    await expect(page.getByRole('columnheader', { name: 'Provider' })).toBeVisible()
    await expect(page.getByRole('columnheader', { name: 'Started' })).toBeVisible()
    await expect(page.getByRole('columnheader', { name: 'Status' })).toBeVisible()
  })
})

test.describe('Quick Start Page', () => {
  test('should show quick start sections', async ({ page }) => {
    await page.goto('/settings')

    await expect(page.getByRole('heading', { name: 'Quick Start' })).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Common Options' })).toBeVisible()
    await expect(page.getByRole('heading', { name: 'How It Works' })).toBeVisible()
  })

  test('should show CLI commands', async ({ page }) => {
    await page.goto('/settings')

    await expect(page.getByText('./monarch-sync serve -port 8085')).toBeVisible()
    await expect(page.getByText('./monarch-sync walmart -days 14 -dry-run')).toBeVisible()
    await expect(page.getByText('./monarch-sync costco -days 7 -dry-run')).toBeVisible()
  })
})
