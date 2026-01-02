import { test, expect } from '@playwright/test'

test('sync shows human-friendly phase names during progress', async ({ page }) => {
  test.setTimeout(120000) // 2 minute timeout

  // Navigate to sync page
  await page.goto('/sync')
  await page.waitForLoadState('networkidle')

  // Take initial screenshot
  await page.screenshot({ path: 'e2e/screenshots/phase-01-initial.png', fullPage: true })

  // Set provider to Walmart using the select element
  const providerSelect = page.locator('select').first()
  await providerSelect.selectOption('walmart')

  // Set lookback days to 14 - find the number input
  const lookbackInput = page.locator('input[type="number"]').first()
  await lookbackInput.fill('14')

  // Take screenshot before starting
  await page.screenshot({ path: 'e2e/screenshots/phase-02-configured.png', fullPage: true })

  // Click Start Sync button
  await page.click('button[type="submit"]')

  // Wait for job to appear and capture progress phases
  const phasesObserved: string[] = []

  for (let i = 0; i < 20; i++) {
    await page.waitForTimeout(2000)

    // Take screenshot
    await page.screenshot({ path: `e2e/screenshots/phase-${String(i + 3).padStart(2, '0')}-progress.png`, fullPage: true })

    // Look for phase text - should now be human-friendly like "Fetching orders from Walmart..."
    const pageContent = await page.textContent('body')

    // Check for the human-friendly phase names
    const friendlyPhases = [
      'Waiting to start...',
      'Initializing...',
      'Fetching orders from Walmart...',
      'Processing orders...',
      'Completed',
      'Failed'
    ]

    for (const phase of friendlyPhases) {
      if (pageContent?.includes(phase) && !phasesObserved.includes(phase)) {
        phasesObserved.push(phase)
        console.log(`Human-friendly phase observed: "${phase}"`)
      }
    }

    // Also check for machine phase names (should NOT appear in UI)
    const machinePhases = ['fetching_orders', 'processing_orders', 'initializing']
    for (const phase of machinePhases) {
      if (pageContent?.includes(phase)) {
        console.log(`WARNING: Machine phase name visible in UI: "${phase}"`)
      }
    }

    // Check if sync completed
    if (pageContent?.includes('Completed') || pageContent?.includes('Failed')) {
      console.log('Sync finished')
      break
    }
  }

  console.log('\n=== Summary ===')
  console.log('Human-friendly phases observed:', phasesObserved)

  // Take final screenshot
  await page.screenshot({ path: 'e2e/screenshots/phase-final.png', fullPage: true })

  // Verify at least one human-friendly phase was shown
  expect(phasesObserved.length).toBeGreaterThan(0)
})
