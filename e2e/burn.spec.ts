import { test, expect } from '@playwright/test';

test.describe('Burn After Reading', () => {
  test('should disable discussion when burn is checked', async ({ page }) => {
    await page.goto('/');

    const burnCheckbox = page.locator('#burn-after-reading');
    const discussionCheckbox = page.locator('#open-discussion');

    // Discussion should be enabled initially
    await expect(discussionCheckbox).toBeEnabled();

    // Check burn-after-reading
    await burnCheckbox.check();

    // Discussion should now be disabled and unchecked
    await expect(discussionCheckbox).toBeDisabled();
    await expect(discussionCheckbox).not.toBeChecked();
  });

  test('should enable discussion when burn is unchecked', async ({ page }) => {
    await page.goto('/');

    const burnCheckbox = page.locator('#burn-after-reading');
    const discussionCheckbox = page.locator('#open-discussion');

    // Check then uncheck burn
    await burnCheckbox.check();
    await burnCheckbox.uncheck();

    // Discussion should be enabled again
    await expect(discussionCheckbox).toBeEnabled();
  });

  test('should create burn paste with dash prefix in URL', async ({ page }) => {
    await page.goto('/');

    await page.fill('#paste-content', 'This is a burn-after-reading paste');
    await page.check('#burn-after-reading');
    await page.click('#create-paste');

    // Wait for navigation
    await page.waitForURL(/\/\?[a-f0-9]{16}#/);

    // URL should have dash prefix in fragment: #-{key}
    const url = page.url();
    expect(url).toMatch(/\/\?[a-f0-9]{16}#-[1-9A-HJ-NP-Za-km-z]+$/);
  });

  test('should show burn warning before viewing', async ({ page, context }) => {
    // Create a burn paste
    await page.goto('/');
    await page.fill('#paste-content', 'Secret burn content');
    await page.check('#burn-after-reading');
    await page.click('#create-paste');
    await page.waitForURL(/\/\?[a-f0-9]{16}#-/);

    const burnUrl = page.url();

    // Open in a new page (simulating different user without sessionStorage)
    const newPage = await context.newPage();
    await newPage.goto(burnUrl);

    // Should show burn warning
    await expect(newPage.locator('#burn-warning')).toBeVisible();

    // Content should NOT be visible yet
    await expect(newPage.locator('#paste-output')).toBeHidden();

    await newPage.close();
  });

  test('should show content after clicking view button', async ({ page, context }) => {
    const testContent = 'Secret burn content to view';

    // Create a burn paste
    await page.goto('/');
    await page.fill('#paste-content', testContent);
    await page.check('#burn-after-reading');
    await page.click('#create-paste');
    await page.waitForURL(/\/\?[a-f0-9]{16}#-/);

    const burnUrl = page.url();

    // Open in a new page (simulating different user)
    const newPage = await context.newPage();
    await newPage.goto(burnUrl);

    // Click view button
    await newPage.click('#view-burn');

    // Warning should be hidden
    await expect(newPage.locator('#burn-warning')).toBeHidden();

    // Content should be visible
    await expect(newPage.locator('#paste-output')).toBeVisible();
    await expect(newPage.locator('#paste-text')).toContainText(testContent);

    await newPage.close();
  });

  test('should return 404 on second access', async ({ page, context }) => {
    // Create a burn paste
    await page.goto('/');
    await page.fill('#paste-content', 'One-time secret');
    await page.check('#burn-after-reading');
    await page.click('#create-paste');
    await page.waitForURL(/\/\?[a-f0-9]{16}#-/);

    const burnUrl = page.url();

    // First viewer - open in a new page
    const firstViewer = await context.newPage();
    await firstViewer.goto(burnUrl);
    await firstViewer.click('#view-burn');

    // Wait for content to load (paste is now deleted on server)
    await expect(firstViewer.locator('#paste-text')).toBeVisible();
    await firstViewer.close();

    // Second viewer tries to access - should fail
    const secondViewer = await context.newPage();
    await secondViewer.goto(burnUrl);

    // Should show not found error
    await expect(secondViewer.locator('#alert')).toContainText('not found');

    await secondViewer.close();
  });
});
