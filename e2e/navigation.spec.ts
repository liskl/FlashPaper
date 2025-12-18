import { test, expect } from '@playwright/test';

test.describe('Navigation Links', () => {
  test('should navigate to How It Works page', async ({ page }) => {
    await page.goto('/');

    // Click the "How It Works" link
    await page.click('a[href="/implementation"]');

    // Verify navigation
    await expect(page).toHaveURL('/implementation');

    // Verify page content loaded (first h2 is "Abstract")
    await expect(page.locator('h2').first()).toBeVisible();
    await expect(page.locator('.tagline')).toContainText('Implementation');
  });

  test('should navigate to Documentation page from home', async ({ page }) => {
    await page.goto('/');

    // Click the "Documentation" link
    await page.click('a[href="/docs"]');

    // Verify navigation
    await expect(page).toHaveURL('/docs');

    // Verify page content loaded
    await expect(page.locator('h2').first()).toContainText('Documentation');
  });

  test('should navigate to Documentation from How It Works page', async ({ page }) => {
    await page.goto('/implementation');

    // Click the "Documentation" link
    await page.click('a[href="/docs"]');

    // Verify navigation
    await expect(page).toHaveURL('/docs');
    await expect(page.locator('h2').first()).toContainText('Documentation');
  });

  test('should navigate to How It Works from Documentation page', async ({ page }) => {
    await page.goto('/docs');

    // Click the "How It Works" link
    await page.click('a[href="/implementation"]');

    // Verify navigation
    await expect(page).toHaveURL('/implementation');
    await expect(page.locator('.tagline')).toContainText('Implementation');
  });

  test('should navigate back to home from How It Works page', async ({ page }) => {
    await page.goto('/implementation');

    // Click the site name/logo link to go home
    await page.click('a[href="/"]');

    // Verify navigation back to home
    await expect(page).toHaveURL('/');
    await expect(page.locator('#new-paste')).toBeVisible();
  });

  test('should navigate back to home from Documentation page', async ({ page }) => {
    await page.goto('/docs');

    // Click the site name/logo link to go home
    await page.click('a[href="/"]');

    // Verify navigation back to home
    await expect(page).toHaveURL('/');
    await expect(page.locator('#new-paste')).toBeVisible();
  });
});
