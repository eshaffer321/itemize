import { test, expect } from '@playwright/test'

test.describe('Navigation', () => {
  test('should load dashboard page', async ({ page }) => {
    await page.goto('/')
    await expect(page).toHaveTitle(/Monarch Sync/)
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

  test('should navigate to settings page', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Settings' }).click()
    await expect(page).toHaveURL(/\/settings/)
    await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible()
  })

  test('should show sidebar with navigation items', async ({ page }) => {
    await page.goto('/')

    // Check sidebar is visible
    await expect(page.getByText('Monarch Sync')).toBeVisible()

    // Check all nav items
    await expect(page.getByRole('link', { name: 'Dashboard' })).toBeVisible()
    await expect(page.getByRole('link', { name: 'Orders' })).toBeVisible()
    await expect(page.getByRole('link', { name: 'Sync Runs' })).toBeVisible()
    await expect(page.getByRole('link', { name: 'Settings' })).toBeVisible()
  })
})

test.describe('Dashboard', () => {
  test('should show stats section', async ({ page }) => {
    await page.goto('/')

    // Check for stat titles
    await expect(page.getByText('Total Orders')).toBeVisible()
    await expect(page.getByText('Successful Orders')).toBeVisible()
    await expect(page.getByText('Failed Orders')).toBeVisible()
    await expect(page.getByText('Total Synced')).toBeVisible()
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

    // Check for filter selects
    await expect(page.getByRole('combobox', { name: 'provider' })).toBeVisible()
    await expect(page.getByRole('combobox', { name: 'status' })).toBeVisible()
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

test.describe('Settings Page', () => {
  test('should show configuration sections', async ({ page }) => {
    await page.goto('/settings')

    await expect(page.getByRole('heading', { name: 'Configuration' })).toBeVisible()
    await expect(page.getByRole('heading', { name: 'API Server' })).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Running Syncs' })).toBeVisible()
  })

  test('should show CLI commands', async ({ page }) => {
    await page.goto('/settings')

    await expect(page.getByText('./monarch-sync serve -port 8080')).toBeVisible()
    await expect(page.getByText('./monarch-sync walmart -days 14')).toBeVisible()
    await expect(page.getByText('./monarch-sync costco -days 7')).toBeVisible()
  })
})
