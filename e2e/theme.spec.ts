import { test, expect } from '@playwright/test';

test.describe('Theme Toggle', () => {
  test.beforeEach(async ({ page }) => {
    // Clear localStorage to start fresh
    await page.goto('/');
    await page.evaluate(() => localStorage.clear());
    await page.reload();
  });

  test('should default to light mode', async ({ page }) => {
    // Light mode: no data-theme attribute
    const html = page.locator('html');
    await expect(html).not.toHaveAttribute('data-theme', 'dark');

    // Button should show "Dark" (to switch to dark)
    await expect(page.locator('#theme-text')).toContainText('Dark');
  });

  test('should toggle to dark mode', async ({ page }) => {
    await page.click('#theme-toggle');

    // Should have data-theme="dark"
    const html = page.locator('html');
    await expect(html).toHaveAttribute('data-theme', 'dark');

    // Button should now show "Light" (to switch back)
    await expect(page.locator('#theme-text')).toContainText('Light');
  });

  test('should toggle back to light mode', async ({ page }) => {
    // Toggle to dark
    await page.click('#theme-toggle');
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');

    // Toggle back to light
    await page.click('#theme-toggle');
    await expect(page.locator('html')).not.toHaveAttribute('data-theme', 'dark');
    await expect(page.locator('#theme-text')).toContainText('Dark');
  });

  test('should persist theme in localStorage', async ({ page }) => {
    // Toggle to dark
    await page.click('#theme-toggle');

    // Check localStorage
    const theme = await page.evaluate(() => localStorage.getItem('flashpaper-theme'));
    expect(theme).toBe('dark');
  });

  test('should load saved theme on page refresh', async ({ page }) => {
    // Toggle to dark mode
    await page.click('#theme-toggle');
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');

    // Reload page
    await page.reload();

    // Should still be dark mode
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
    await expect(page.locator('#theme-text')).toContainText('Light');
  });

  test('should load saved theme on new navigation', async ({ page }) => {
    // Toggle to dark mode
    await page.click('#theme-toggle');

    // Navigate away and back
    await page.goto('about:blank');
    await page.goto('/');

    // Should still be dark mode
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  });

  test('theme icon should update', async ({ page }) => {
    // Light mode shows moon icon (click to go dark)
    const iconLight = await page.locator('#theme-icon').textContent();
    expect(iconLight).toContain('\u263E'); // Moon symbol (☾)

    // Toggle to dark
    await page.click('#theme-toggle');

    // Dark mode shows sun icon (click to go light)
    const iconDark = await page.locator('#theme-icon').textContent();
    expect(iconDark).toContain('\u2600'); // Sun symbol (☀)
  });
});
