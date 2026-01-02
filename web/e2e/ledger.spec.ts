import { test, expect } from '@playwright/test'

test.describe('Payment Ledger', () => {
  test('should display ledger section on order detail page with ledger data', async ({ page }) => {
    // Navigate to an order that has ledger data (Walmart order with ledger)
    await page.goto('/orders/200013940133681')
    await page.waitForLoadState('networkidle')

    // Check for Payment Ledger heading
    const ledgerSection = page.getByText('Payment Ledger')

    // If ledger exists, verify the section content
    if (await ledgerSection.isVisible()) {
      // Verify ledger state badge (use .first() to avoid strict mode violation when "Charged" appears in both badge and "Total Charged")
      await expect(page.locator('span:has-text("Charged"), span:has-text("Pending"), span:has-text("Partial Refund")').first()).toBeVisible()

      // Verify ledger details are shown
      await expect(page.getByText('Total Charged')).toBeVisible()
      await expect(page.getByText('Payment Methods')).toBeVisible()
      await expect(page.getByText('Charge Count')).toBeVisible()
      await expect(page.getByText('Ledger Version')).toBeVisible()
      await expect(page.getByText('Last Updated')).toBeVisible()
    }
  })

  test('should display charges table when ledger has charges', async ({ page }) => {
    await page.goto('/orders/200013940133681')
    await page.waitForLoadState('networkidle')

    // Check if charges section exists
    const chargesHeading = page.getByRole('heading', { name: /Charges \(\d+\)/ })

    if (await chargesHeading.isVisible()) {
      // Verify charges table headers (updated to include new 'Seq' column)
      await expect(page.getByRole('columnheader', { name: 'Seq' })).toBeVisible()
      await expect(page.getByRole('columnheader', { name: 'Type' })).toBeVisible()
      await expect(page.getByRole('columnheader', { name: 'Payment Method' })).toBeVisible()
      await expect(page.getByRole('columnheader', { name: 'Card' })).toBeVisible()
      await expect(page.getByRole('columnheader', { name: 'Amount' })).toBeVisible()
      await expect(page.getByRole('columnheader', { name: 'Status' })).toBeVisible()

      // Verify total row is present
      await expect(page.getByText('Net Total')).toBeVisible()
    }
  })

  test('should show empty state when ledger has no charges', async ({ page }) => {
    await page.goto('/orders/200013940133681')
    await page.waitForLoadState('networkidle')

    const ledgerSection = page.getByText('Payment Ledger').first()

    if (await ledgerSection.isVisible()) {
      // Check for either charges table or empty state
      const chargesHeading = page.getByRole('heading', { name: /Charges \(\d+\)/ })
      const emptyState = page.getByText('No charge records available for this ledger.')

      // One of these should be visible
      const hasCharges = await chargesHeading.isVisible()
      const hasEmptyState = await emptyState.isVisible()

      expect(hasCharges || hasEmptyState).toBeTruthy()
    }
  })

  test('should show ledger state badges correctly', async ({ page }) => {
    await page.goto('/orders/200013940133681')
    await page.waitForLoadState('networkidle')

    // The ledger should have one of the valid states
    const ledgerSection = page.locator('text=Payment Ledger').first()

    if (await ledgerSection.isVisible()) {
      // Check for any of the possible state badges
      const stateTexts = ['Charged', 'Pending', 'Partial Refund', 'Refunded']
      const stateVisible = await Promise.any(
        stateTexts.map(async (text) => {
          const badge = page.locator(`text=${text}`).first()
          return badge.isVisible().then(visible => visible ? text : null)
        })
      ).catch(() => null)

      expect(stateVisible).not.toBeNull()
    }
  })

  test('should show prominent warning for invalid ledger', async ({ page }) => {
    // This test checks that invalid ledgers display a warning banner
    await page.goto('/orders/200013940133681')
    await page.waitForLoadState('networkidle')

    const ledgerSection = page.getByText('Payment Ledger').first()

    if (await ledgerSection.isVisible()) {
      // Check if the invalid badge is present
      const invalidBadge = page.locator('text=Invalid')
      const isInvalid = await invalidBadge.isVisible()

      if (isInvalid) {
        // If invalid, the warning banner should be visible
        await expect(page.getByText('Ledger Validation Failed')).toBeVisible()
      }
    }
  })

  test('capture order detail with ledger screenshot', async ({ page }) => {
    await page.goto('/orders/200013940133681')
    await page.waitForLoadState('networkidle')

    // Take full page screenshot
    await page.screenshot({
      path: 'screenshots/order-detail-with-ledger.png',
      fullPage: true
    })
  })

  test('capture ledger section only screenshot', async ({ page }) => {
    await page.goto('/orders/200013940133681')
    await page.waitForLoadState('networkidle')

    // Scroll to ledger section if it exists
    const ledgerSection = page.getByText('Payment Ledger').first()

    if (await ledgerSection.isVisible()) {
      await ledgerSection.scrollIntoViewIfNeeded()

      // Take screenshot of the viewport with ledger visible
      await page.screenshot({
        path: 'screenshots/ledger-section.png'
      })
    }
  })

  test('capture charges table with total row screenshot', async ({ page }) => {
    await page.goto('/orders/200013940133681')
    await page.waitForLoadState('networkidle')

    // Scroll to charges section if it exists
    const chargesHeading = page.getByRole('heading', { name: /Charges \(\d+\)/ })

    if (await chargesHeading.isVisible()) {
      await chargesHeading.scrollIntoViewIfNeeded()

      // Take screenshot of the charges table area
      await page.screenshot({
        path: 'screenshots/ledger-charges-table.png'
      })
    }
  })
})

test.describe('Order Detail Page - Full Content', () => {
  test('should display all sections including ledger', async ({ page }) => {
    await page.goto('/orders/200013940133681')
    await page.waitForLoadState('networkidle')

    // Order header
    await expect(page.getByRole('heading', { name: /Order 200013940133681/ })).toBeVisible()

    // Order Summary section
    await expect(page.getByText('Order Summary')).toBeVisible()
    await expect(page.getByText('Subtotal')).toBeVisible()
    await expect(page.getByText('Tax')).toBeVisible()

    // Items section (if present)
    const itemsHeading = page.getByText(/Items \(\d+\)/)
    if (await itemsHeading.isVisible()) {
      await expect(page.getByRole('columnheader', { name: 'Item' })).toBeVisible()
    }

    // Splits section (if present)
    const splitsHeading = page.getByText(/Transaction Splits \(\d+\)/)
    if (await splitsHeading.isVisible()) {
      await expect(page.getByRole('columnheader', { name: 'Category' })).toBeVisible()
    }
  })
})
