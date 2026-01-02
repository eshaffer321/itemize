import { test } from '@playwright/test'

/**
 * Comprehensive UX Investigation
 *
 * Persona: Technical power user who uses Monarch Money for personal finance tracking
 * and wants to sync retail purchases (Walmart, Costco, Amazon) to categorize
 * spending at the item level rather than just the transaction level.
 *
 * Goals:
 * - Monitor sync status and health
 * - Review processed orders and their splits
 * - Troubleshoot failed syncs
 * - Understand spending patterns
 */

test.describe('UX Investigation - Dashboard Flow', () => {
  test('dashboard first impression and information hierarchy', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(500) // Let animations settle
    await page.screenshot({
      path: 'screenshots/ux/01-dashboard-first-impression.png',
      fullPage: true
    })
  })

  test('dashboard stat cards interaction', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('networkidle')

    // Hover over stat cards to check for any hover states
    const statCards = page.locator('[class*="border-t"]').first()
    if (await statCards.isVisible()) {
      await statCards.hover()
      await page.screenshot({
        path: 'screenshots/ux/02-dashboard-stat-hover.png',
        fullPage: false
      })
    }
  })

  test('sidebar navigation states', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('networkidle')

    // Capture sidebar with Dashboard active
    await page.screenshot({
      path: 'screenshots/ux/03-sidebar-dashboard-active.png',
      clip: { x: 0, y: 0, width: 280, height: 800 }
    })

    // Navigate to Orders and capture sidebar state
    await page.click('text=Orders')
    await page.waitForLoadState('networkidle')
    await page.screenshot({
      path: 'screenshots/ux/04-sidebar-orders-active.png',
      clip: { x: 0, y: 0, width: 280, height: 800 }
    })
  })
})

test.describe('UX Investigation - Orders Flow', () => {
  test('orders page initial state', async ({ page }) => {
    await page.goto('/orders')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(500)
    await page.screenshot({
      path: 'screenshots/ux/05-orders-initial.png',
      fullPage: true
    })
  })

  test('orders filter interaction', async ({ page }) => {
    await page.goto('/orders')
    await page.waitForLoadState('networkidle')

    // Check if provider filter exists and click it
    const providerSelect = page.locator('[name="provider"]')
    if (await providerSelect.isVisible()) {
      await providerSelect.click()
      await page.waitForTimeout(200)
      await page.screenshot({
        path: 'screenshots/ux/06-orders-provider-dropdown.png',
        fullPage: true
      })
    }
  })

  test('orders table row hover', async ({ page }) => {
    await page.goto('/orders')
    await page.waitForLoadState('networkidle')

    // Try to hover on first table row
    const firstRow = page.locator('tbody tr').first()
    if (await firstRow.isVisible()) {
      await firstRow.hover()
      await page.screenshot({
        path: 'screenshots/ux/07-orders-row-hover.png',
        fullPage: true
      })
    }
  })
})

test.describe('UX Investigation - Order Detail Flow', () => {
  test('attempt order detail navigation', async ({ page }) => {
    await page.goto('/orders')
    await page.waitForLoadState('networkidle')

    // Try clicking on an order row to navigate to detail
    const orderLink = page.locator('tbody tr a').first()
    if (await orderLink.isVisible()) {
      await orderLink.click()
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(500)
      await page.screenshot({
        path: 'screenshots/ux/08-order-detail.png',
        fullPage: true
      })
    }
  })
})

test.describe('UX Investigation - Sync Runs Flow', () => {
  test('sync runs page initial state', async ({ page }) => {
    await page.goto('/runs')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(500)
    await page.screenshot({
      path: 'screenshots/ux/09-runs-initial.png',
      fullPage: true
    })
  })
})

test.describe('UX Investigation - Settings Flow', () => {
  test('settings page layout', async ({ page }) => {
    await page.goto('/settings')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(500)
    await page.screenshot({
      path: 'screenshots/ux/10-settings-layout.png',
      fullPage: true
    })
  })

  test('settings code block selection', async ({ page }) => {
    await page.goto('/settings')
    await page.waitForLoadState('networkidle')

    // Check if code blocks are selectable/clickable
    const codeBlock = page.locator('pre').first()
    if (await codeBlock.isVisible()) {
      await codeBlock.click()
      await page.screenshot({
        path: 'screenshots/ux/11-settings-code-selected.png',
        fullPage: true
      })
    }
  })
})

test.describe('UX Investigation - Responsive Design', () => {
  test('tablet viewport (768px)', async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 })
    await page.goto('/')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(500)
    await page.screenshot({
      path: 'screenshots/ux/12-responsive-tablet-dashboard.png',
      fullPage: true
    })

    await page.goto('/orders')
    await page.waitForLoadState('networkidle')
    await page.screenshot({
      path: 'screenshots/ux/13-responsive-tablet-orders.png',
      fullPage: true
    })
  })

  test('mobile viewport (375px)', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 })
    await page.goto('/')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(500)
    await page.screenshot({
      path: 'screenshots/ux/14-responsive-mobile-dashboard.png',
      fullPage: true
    })

    await page.goto('/orders')
    await page.waitForLoadState('networkidle')
    await page.screenshot({
      path: 'screenshots/ux/15-responsive-mobile-orders.png',
      fullPage: true
    })

    await page.goto('/settings')
    await page.waitForLoadState('networkidle')
    await page.screenshot({
      path: 'screenshots/ux/16-responsive-mobile-settings.png',
      fullPage: true
    })
  })
})

test.describe('UX Investigation - Navigation Flow', () => {
  test('complete user journey screenshot sequence', async ({ page }) => {
    // Simulate a user checking their sync status

    // 1. Land on dashboard
    await page.goto('/')
    await page.waitForLoadState('networkidle')
    await page.screenshot({ path: 'screenshots/ux/17-journey-01-dashboard.png', fullPage: true })

    // 2. Check recent orders
    await page.click('text=Orders')
    await page.waitForLoadState('networkidle')
    await page.screenshot({ path: 'screenshots/ux/18-journey-02-orders.png', fullPage: true })

    // 3. Review sync runs
    await page.click('text=Sync Runs')
    await page.waitForLoadState('networkidle')
    await page.screenshot({ path: 'screenshots/ux/19-journey-03-runs.png', fullPage: true })

    // 4. Check settings (Quick Start page)
    await page.click('text=Quick Start')
    await page.waitForLoadState('networkidle')
    await page.screenshot({ path: 'screenshots/ux/20-journey-04-settings.png', fullPage: true })

    // 5. Return to dashboard
    await page.click('text=Dashboard')
    await page.waitForLoadState('networkidle')
    await page.screenshot({ path: 'screenshots/ux/21-journey-05-back-to-dashboard.png', fullPage: true })
  })
})

test.describe('UX Investigation - Visual Details', () => {
  test('typography and spacing', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('networkidle')

    // Focus on the main content area
    await page.screenshot({
      path: 'screenshots/ux/22-typography-dashboard.png',
      clip: { x: 256, y: 0, width: 1024, height: 600 }
    })
  })

  test('empty states appearance', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('networkidle')

    // Screenshot the empty tables area
    const recentOrders = page.locator('text=Recent Orders').first()
    if (await recentOrders.isVisible()) {
      const box = await recentOrders.boundingBox()
      if (box) {
        await page.screenshot({
          path: 'screenshots/ux/23-empty-state-orders.png',
          clip: { x: box.x - 20, y: box.y - 10, width: 800, height: 300 }
        })
      }
    }
  })

  test('user profile dropdown', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('networkidle')

    // Try to click on user profile area
    const userProfile = page.locator('text=user@example.com').first()
    if (await userProfile.isVisible()) {
      await userProfile.click()
      await page.waitForTimeout(300)
      await page.screenshot({
        path: 'screenshots/ux/24-user-dropdown.png',
        clip: { x: 0, y: 400, width: 280, height: 400 }
      })
    }
  })
})
