import { test, expect } from '@playwright/test'

test.describe('Dark Mode Toggle', () => {
  test.beforeEach(async ({ page }) => {
    // Clear localStorage before each test
    await page.goto('/')
    await page.evaluate(() => localStorage.clear())
  })

  // Helper to open user dropdown menu
  async function openUserMenu(page: any) {
    // Click on the user section in sidebar footer (shows "User" text)
    await page.locator('text=user@example.com').click()
  }

  test('theme toggle is accessible from user menu', async ({ page }) => {
    await page.goto('/')
    await page.setViewportSize({ width: 1280, height: 800 })

    // Open user dropdown
    await openUserMenu(page)

    // Wait for dropdown to appear and verify theme options exist
    await expect(page.getByRole('menuitem', { name: 'Light' })).toBeVisible()
    await expect(page.getByRole('menuitem', { name: 'Dark' })).toBeVisible()
    await expect(page.getByRole('menuitem', { name: 'System' })).toBeVisible()

    // Take screenshot of the dropdown with theme options
    await page.screenshot({ path: 'screenshots/theme-toggle-menu.png' })
  })

  test('can switch to dark mode', async ({ page }) => {
    await page.goto('/')
    await page.setViewportSize({ width: 1280, height: 800 })

    // Take light mode screenshot first
    await page.screenshot({ path: 'screenshots/light-mode.png', fullPage: true })

    // Open user menu and click Dark
    await openUserMenu(page)
    await page.getByRole('menuitem', { name: 'Dark' }).click()

    // Wait for the dark class to be applied
    await expect(page.locator('html')).toHaveClass(/\bdark\b/)

    // Wait for dropdown to close and UI to settle
    await page.waitForTimeout(500)

    // Emulate dark color scheme so CSS media queries also apply
    await page.emulateMedia({ colorScheme: 'dark' })

    // Take dark mode screenshot (without dropdown open)
    await page.screenshot({ path: 'screenshots/dark-mode.png', fullPage: true })
  })

  test('dark mode persists after page reload', async ({ page }) => {
    await page.goto('/')
    await page.setViewportSize({ width: 1280, height: 800 })

    // Switch to dark mode
    await openUserMenu(page)
    await page.getByRole('menuitem', { name: 'Dark' }).click()

    // Verify dark class is applied
    await expect(page.locator('html')).toHaveClass(/dark/)

    // Reload the page
    await page.reload()

    // Verify dark mode persists
    await expect(page.locator('html')).toHaveClass(/dark/)
  })

  test('can switch back to light mode', async ({ page }) => {
    await page.goto('/')
    await page.setViewportSize({ width: 1280, height: 800 })

    // Switch to dark mode first
    await openUserMenu(page)
    await page.getByRole('menuitem', { name: 'Dark' }).click()
    // Check for 'dark' class (space-bounded or at end) vs 'dark:' prefix
    await expect(page.locator('html')).toHaveClass(/\bdark\b/)

    // Switch back to light mode
    await openUserMenu(page)
    await page.getByRole('menuitem', { name: 'Light' }).click()

    // Verify 'dark' class word is removed (not just dark: prefix)
    await expect(page.locator('html')).not.toHaveClass(/\bdark\s|dark$/)
  })

  test('system option respects OS preference', async ({ page }) => {
    // Emulate dark color scheme
    await page.emulateMedia({ colorScheme: 'dark' })
    await page.goto('/')
    await page.setViewportSize({ width: 1280, height: 800 })

    // Open menu and select System
    await openUserMenu(page)
    await page.getByRole('menuitem', { name: 'System' }).click()

    // With dark OS preference, should have dark class
    await expect(page.locator('html')).toHaveClass(/dark/)
  })

  test('active theme shows indicator', async ({ page }) => {
    await page.goto('/')
    await page.setViewportSize({ width: 1280, height: 800 })

    // Switch to dark mode
    await openUserMenu(page)
    await page.getByRole('menuitem', { name: 'Dark' }).click()

    // Reopen menu and verify Dark shows as active
    await openUserMenu(page)

    // Look for the "Active" indicator next to Dark
    const darkMenuItem = page.getByRole('menuitem', { name: 'Dark' })
    await expect(darkMenuItem).toContainText('Active')
  })
})
