import { test, expect } from '@playwright/test'

test.describe('Order Search', () => {
  test('search input is visible on orders page', async ({ page }) => {
    await page.goto('/orders')

    // Verify search input is present
    const searchInput = page.locator('input[name="search"]')
    await expect(searchInput).toBeVisible()
    await expect(searchInput).toHaveAttribute('placeholder', 'Search order ID...')
  })

  test('search filters orders by order ID', async ({ page }) => {
    await page.goto('/orders')

    // Get initial count
    const initialCount = await page.locator('tbody tr').count()
    expect(initialCount).toBeGreaterThan(0)

    // Type a search term and submit
    const searchInput = page.locator('input[name="search"]')
    await searchInput.fill('200014')
    await page.click('button[type="submit"]')

    // Wait for navigation with search param
    await page.waitForURL(/search=200014/)

    // Verify results are filtered
    const filteredCount = await page.locator('tbody tr').count()
    expect(filteredCount).toBeLessThanOrEqual(initialCount)

    // Verify all visible order IDs contain the search term
    const orderIds = await page.locator('tbody tr td:first-child').allTextContents()
    for (const orderId of orderIds) {
      expect(orderId).toContain('200014')
    }
  })

  test('search with no results shows empty state', async ({ page }) => {
    await page.goto('/orders?search=XXXNONEXISTENT')

    // Should show "No orders found" message
    await expect(page.locator('text=No orders found')).toBeVisible()
    await expect(page.locator('text=Try adjusting your filters or search query')).toBeVisible()
  })

  test('search persists in pagination links', async ({ page }) => {
    await page.goto('/orders?search=200014')

    // If there's a "Next" link, check it includes the search param
    const nextLink = page.locator('a:has-text("Next")')
    if (await nextLink.isVisible()) {
      const href = await nextLink.getAttribute('href')
      expect(href).toContain('search=200014')
    }
  })
})
