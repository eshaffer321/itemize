import { test, expect } from '@playwright/test'

test.describe('Sync Page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/sync')
    // Wait for page to be ready - don't use networkidle as sync page polls for status
    await page.waitForLoadState('domcontentloaded')
    // Wait for the sync form to appear
    await page.waitForSelector('h1:has-text("Sync")', { timeout: 10000 })
  })

  test('should display the sync page with title', async ({ page }) => {
    // The page has an h1 with text "Sync"
    await expect(page.locator('h1')).toContainText('Sync')
  })

  test('should have provider dropdown with options', async ({ page }) => {
    // Find the provider select/dropdown
    const providerSelect = page.locator('select').first()
    await expect(providerSelect).toBeVisible()

    // Check that Walmart is the default selection
    await expect(providerSelect).toHaveValue('walmart')
  })

  test('should have lookback days input', async ({ page }) => {
    // Find the number input for lookback days (has value 14 by default)
    const lookbackInput = page.locator('input[type="number"]').first()
    await expect(lookbackInput).toBeVisible()
    await expect(lookbackInput).toHaveValue('14')
  })

  test('should have start sync button', async ({ page }) => {
    const startButton = page.getByRole('button', { name: /start sync/i })
    await expect(startButton).toBeVisible()
  })

  test('should have sync configuration section', async ({ page }) => {
    // Check for "Sync Configuration" text
    await expect(page.getByText('Sync Configuration')).toBeVisible()
  })

  test('should have provider options', async ({ page }) => {
    const providerSelect = page.locator('select').first()

    // Check all three options exist
    await expect(providerSelect.locator('option[value="walmart"]')).toBeAttached()
    await expect(providerSelect.locator('option[value="costco"]')).toBeAttached()
    await expect(providerSelect.locator('option[value="amazon"]')).toBeAttached()
  })

  test('should be able to change provider', async ({ page }) => {
    const providerSelect = page.locator('select').first()

    // Change to costco
    await providerSelect.selectOption('costco')
    await expect(providerSelect).toHaveValue('costco')

    // Change to amazon
    await providerSelect.selectOption('amazon')
    await expect(providerSelect).toHaveValue('amazon')
  })

  test('should have sync navigation in sidebar', async ({ page }) => {
    // Check that Sync link exists in sidebar
    const syncLink = page.locator('a[href="/sync"]')
    await expect(syncLink).toBeVisible()
  })

  test('should navigate to sync page from home', async ({ page }) => {
    // Go to home first
    await page.goto('/')
    await page.waitForLoadState('domcontentloaded')
    await page.waitForSelector('h1:has-text("Dashboard")', { timeout: 10000 })

    // Click on Sync in navigation
    await page.locator('a[href="/sync"]').first().click()

    // Should be on sync page
    await expect(page).toHaveURL(/\/sync/)
    await expect(page.locator('h1')).toContainText('Sync')
  })
})
