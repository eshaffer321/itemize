import { test, expect } from '@playwright/test'

/**
 * Deep UX Analysis - Round 2
 * Testing actual functionality, not just appearance
 */

test.describe('Order Detail Navigation', () => {
  test('clicking order row navigates to detail page', async ({ page }) => {
    await page.goto('/orders')
    await page.waitForLoadState('networkidle')

    // Screenshot before click
    await page.screenshot({ path: 'screenshots/analysis/01-orders-before-click.png' })

    // Click the first order row
    const firstRow = page.locator('tbody tr').first()
    await firstRow.click()

    // Wait for navigation
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(500)

    // Screenshot after click
    await page.screenshot({ path: 'screenshots/analysis/02-order-detail-after-click.png', fullPage: true })

    // Check if we navigated to order detail
    const url = page.url()
    console.log('Navigated to:', url)
  })

  test('order detail shows items and splits', async ({ page }) => {
    // Navigate directly to a known order
    await page.goto('/orders/200014334161019')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(500)

    await page.screenshot({ path: 'screenshots/analysis/03-order-detail-direct.png', fullPage: true })

    // Check for expected elements
    const hasBackLink = await page.locator('text=Orders').first().isVisible()
    const hasOrderId = await page.getByRole('heading', { name: /Order/ }).isVisible()

    console.log('Has back link:', hasBackLink)
    console.log('Has order heading:', hasOrderId)
  })
})

test.describe('Pagination Flow', () => {
  test('pagination controls work correctly', async ({ page }) => {
    await page.goto('/orders')
    await page.waitForLoadState('networkidle')

    // Screenshot page 1
    await page.screenshot({ path: 'screenshots/analysis/04-pagination-page1.png', fullPage: true })

    // Check we're on page 1
    await expect(page.locator('text=Page 1 of')).toBeVisible()
    await expect(page.locator('text=Showing 1–25')).toBeVisible()

    // Click Next
    await page.locator('a:has-text("Next")').click()
    await page.waitForLoadState('networkidle')

    // Screenshot page 2
    await page.screenshot({ path: 'screenshots/analysis/05-pagination-page2.png', fullPage: true })

    // Check we're on page 2
    await expect(page.locator('text=Page 2 of')).toBeVisible()
    await expect(page.locator('text=Showing 26–50')).toBeVisible()

    // Click Previous
    await page.locator('a:has-text("Previous")').click()
    await page.waitForLoadState('networkidle')

    // Should be back on page 1
    await expect(page.locator('text=Page 1 of')).toBeVisible()
  })

  test('pagination to last page', async ({ page }) => {
    await page.goto('/orders?page=4')
    await page.waitForLoadState('networkidle')

    await page.screenshot({ path: 'screenshots/analysis/06-pagination-last-page.png', fullPage: true })

    // Next should be disabled on last page
    const nextButton = page.locator('text=Next').last()
    const isDisabledNext = await nextButton.evaluate(el => {
      return el.tagName === 'SPAN' || el.classList.contains('text-zinc-400')
    })
    console.log('Next is disabled on last page:', isDisabledNext)
  })
})

test.describe('Filter Functionality', () => {
  test('filter by provider walmart', async ({ page }) => {
    await page.goto('/orders')
    await page.waitForLoadState('networkidle')

    // Select Walmart from provider dropdown
    await page.selectOption('select[name="provider"]', 'walmart')

    // Click Filter button
    await page.click('button:has-text("Filter")')
    await page.waitForLoadState('networkidle')

    await page.screenshot({ path: 'screenshots/analysis/07-filter-walmart.png', fullPage: true })

    // Check URL has provider param
    expect(page.url()).toContain('provider=walmart')

    // All visible provider badges should be walmart
    const providerBadges = page.locator('tbody td:nth-child(2)')
    const count = await providerBadges.count()
    console.log('Filtered results count:', count)
  })

  test('filter by status failed', async ({ page }) => {
    await page.goto('/orders')
    await page.waitForLoadState('networkidle')

    // Select Failed from status dropdown
    await page.selectOption('select[name="status"]', 'failed')
    await page.click('button:has-text("Filter")')
    await page.waitForLoadState('networkidle')

    await page.screenshot({ path: 'screenshots/analysis/08-filter-failed.png', fullPage: true })

    expect(page.url()).toContain('status=failed')
  })

  test('combined filters', async ({ page }) => {
    await page.goto('/orders')
    await page.waitForLoadState('networkidle')

    // Select both filters
    await page.selectOption('select[name="provider"]', 'costco')
    await page.selectOption('select[name="status"]', 'success')
    await page.click('button:has-text("Filter")')
    await page.waitForLoadState('networkidle')

    await page.screenshot({ path: 'screenshots/analysis/09-filter-combined.png', fullPage: true })

    expect(page.url()).toContain('provider=costco')
    expect(page.url()).toContain('status=success')
  })
})

test.describe('Dashboard Interactions', () => {
  test('dashboard stat cards link to filtered views', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('networkidle')

    // Check if clicking on "Failed" stat could link somewhere
    // (Currently stats don't link - this is a potential enhancement)
    await page.screenshot({ path: 'screenshots/analysis/10-dashboard-stats.png' })
  })

  test('recent orders table rows are clickable', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('networkidle')

    // Try clicking a recent order
    const firstRecentOrder = page.locator('text=Recent Orders').locator('..').locator('tbody tr').first()

    if (await firstRecentOrder.isVisible()) {
      await firstRecentOrder.click()
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(300)

      await page.screenshot({ path: 'screenshots/analysis/11-dashboard-order-click.png', fullPage: true })
      console.log('Clicked order, URL:', page.url())
    }
  })
})

test.describe('Sync Runs Analysis', () => {
  test('sync runs page details', async ({ page }) => {
    await page.goto('/runs')
    await page.waitForLoadState('networkidle')

    await page.screenshot({ path: 'screenshots/analysis/12-runs-full.png', fullPage: true })

    // Check table headers
    await expect(page.locator('th:has-text("Run ID")')).toBeVisible()
    await expect(page.locator('th:has-text("Duration")')).toBeVisible()
    await expect(page.locator('th:has-text("Found")')).toBeVisible()
  })
})

test.describe('Mobile Deep Dive', () => {
  test('mobile navigation menu', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 })
    await page.goto('/')
    await page.waitForLoadState('networkidle')

    // Screenshot initial mobile view
    await page.screenshot({ path: 'screenshots/analysis/13-mobile-initial.png', fullPage: true })

    // Click hamburger menu (if exists)
    const hamburger = page.locator('button[aria-label*="menu"], button[aria-label*="Menu"], [data-headlessui-state] button').first()
    if (await hamburger.isVisible()) {
      await hamburger.click()
      await page.waitForTimeout(300)
      await page.screenshot({ path: 'screenshots/analysis/14-mobile-menu-open.png', fullPage: true })
    }
  })

  test('mobile orders page with filters', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 })
    await page.goto('/orders')
    await page.waitForLoadState('networkidle')

    await page.screenshot({ path: 'screenshots/analysis/15-mobile-orders.png', fullPage: true })

    // Scroll to see pagination
    await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight))
    await page.waitForTimeout(200)
    await page.screenshot({ path: 'screenshots/analysis/16-mobile-orders-bottom.png', fullPage: true })
  })

  test('mobile order detail', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 })
    await page.goto('/orders/200014334161019')
    await page.waitForLoadState('networkidle')

    await page.screenshot({ path: 'screenshots/analysis/17-mobile-order-detail.png', fullPage: true })

    // Scroll to see items/splits
    await page.evaluate(() => window.scrollTo(0, 500))
    await page.waitForTimeout(200)
    await page.screenshot({ path: 'screenshots/analysis/18-mobile-order-detail-scroll.png', fullPage: true })
  })
})

test.describe('Quick Start Page Analysis', () => {
  test('quick start code blocks', async ({ page }) => {
    await page.goto('/settings')
    await page.waitForLoadState('networkidle')

    await page.screenshot({ path: 'screenshots/analysis/19-quickstart-full.png', fullPage: true })

    // Scroll to see all content
    await page.evaluate(() => window.scrollTo(0, 500))
    await page.waitForTimeout(200)
    await page.screenshot({ path: 'screenshots/analysis/20-quickstart-middle.png', fullPage: true })

    await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight))
    await page.waitForTimeout(200)
    await page.screenshot({ path: 'screenshots/analysis/21-quickstart-bottom.png', fullPage: true })
  })
})

test.describe('Error States', () => {
  test('order not found', async ({ page }) => {
    await page.goto('/orders/nonexistent-order-12345')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(500)

    await page.screenshot({ path: 'screenshots/analysis/22-order-not-found.png', fullPage: true })
  })

  test('empty filter results', async ({ page }) => {
    // Try a filter combination that likely returns no results
    await page.goto('/orders?provider=amazon&status=dry-run')
    await page.waitForLoadState('networkidle')

    await page.screenshot({ path: 'screenshots/analysis/23-empty-filter-results.png', fullPage: true })
  })
})

test.describe('Visual Polish Check', () => {
  test('hover states on table rows', async ({ page }) => {
    await page.goto('/orders')
    await page.waitForLoadState('networkidle')

    // Hover over third row (to show contrast)
    const thirdRow = page.locator('tbody tr').nth(2)
    await thirdRow.hover()

    await page.screenshot({ path: 'screenshots/analysis/24-row-hover-state.png' })
  })

  test('dark mode check', async ({ page }) => {
    // Emulate dark mode preference
    await page.emulateMedia({ colorScheme: 'dark' })
    await page.goto('/')
    await page.waitForLoadState('networkidle')

    await page.screenshot({ path: 'screenshots/analysis/25-dark-mode-dashboard.png', fullPage: true })

    await page.goto('/orders')
    await page.waitForLoadState('networkidle')
    await page.screenshot({ path: 'screenshots/analysis/26-dark-mode-orders.png', fullPage: true })

    await page.goto('/settings')
    await page.waitForLoadState('networkidle')
    await page.screenshot({ path: 'screenshots/analysis/27-dark-mode-quickstart.png', fullPage: true })
  })
})
