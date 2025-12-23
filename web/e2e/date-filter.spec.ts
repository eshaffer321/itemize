import { test, expect } from '@playwright/test'

test.describe('Date Range Filter', () => {
  test('date range dropdown is visible on orders page', async ({ page }) => {
    await page.goto('/orders')

    // Verify date range select is present
    const dateSelect = page.locator('select[name="days"]')
    await expect(dateSelect).toBeVisible()

    // Verify all options are present
    await expect(dateSelect.locator('option[value=""]')).toHaveText('All Time')
    await expect(dateSelect.locator('option[value="7"]')).toHaveText('Last 7 Days')
    await expect(dateSelect.locator('option[value="30"]')).toHaveText('Last 30 Days')
    await expect(dateSelect.locator('option[value="90"]')).toHaveText('Last 90 Days')
    await expect(dateSelect.locator('option[value="365"]')).toHaveText('Last Year')

    // Take screenshot of the filter bar
    await page.screenshot({ path: 'screenshots/date-filter-dropdown.png', fullPage: false })
  })

  test('date range filter reduces results when applied', async ({ page }) => {
    await page.goto('/orders')

    // Get initial count from "Showing X-Y of Z orders" text
    const countText = await page.locator('text=/Showing.*of.*orders/').textContent()
    const initialTotal = parseInt(countText?.match(/of (\d+) orders/)?.[1] || '0')

    // Select "Last 7 Days" and submit
    const dateSelect = page.locator('select[name="days"]')
    await dateSelect.selectOption('7')
    await page.click('button[type="submit"]')

    // Wait for navigation with days param
    await page.waitForURL(/days=7/)

    // Take screenshot showing filtered results
    await page.screenshot({ path: 'screenshots/date-filter-7-days.png', fullPage: true })

    // Get filtered count
    const filteredText = await page.locator('text=/Showing.*of.*orders/').textContent()
    const filteredTotal = parseInt(filteredText?.match(/of (\d+) orders/)?.[1] || '0')

    // Filtered should be <= initial (usually less for 7 days)
    expect(filteredTotal).toBeLessThanOrEqual(initialTotal)
  })

  test('date range persists in pagination links', async ({ page }) => {
    await page.goto('/orders?days=30')

    // If there's a "Next" link, check it includes the days param
    const nextLink = page.locator('a:has-text("Next")')
    if (await nextLink.isVisible()) {
      const href = await nextLink.getAttribute('href')
      expect(href).toContain('days=30')
    }

    // Check the date select shows the correct value
    const dateSelect = page.locator('select[name="days"]')
    await expect(dateSelect).toHaveValue('30')
  })

  test('date range filter combines with other filters', async ({ page }) => {
    // Apply both date range and provider filter
    await page.goto('/orders')

    // Select date range
    const dateSelect = page.locator('select[name="days"]')
    await dateSelect.selectOption('90')

    // Select provider
    const providerSelect = page.locator('select[name="provider"]')
    await providerSelect.selectOption('walmart')

    // Submit
    await page.click('button[type="submit"]')

    // Wait for both params in URL
    await page.waitForURL(/days=90/)
    expect(page.url()).toContain('provider=walmart')

    // Take screenshot of combined filters
    await page.screenshot({ path: 'screenshots/date-filter-combined.png', fullPage: true })
  })

  test('screenshot of orders page with all filter options', async ({ page }) => {
    await page.goto('/orders')

    // Take a comprehensive screenshot showing all filter controls
    await page.screenshot({ path: 'screenshots/orders-page-filters.png', fullPage: true })
  })
})
