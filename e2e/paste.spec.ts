import { test, expect } from '@playwright/test';

test.describe('Paste Creation', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
  });

  test('should show create form on home page', async ({ page }) => {
    await expect(page.locator('#new-paste')).toBeVisible();
    await expect(page.locator('#paste-content')).toBeVisible();
    await expect(page.locator('#create-paste')).toBeVisible();
  });

  test('should create a paste and show URL with key', async ({ page }) => {
    const testContent = 'Hello, FlashPaper! Test paste content.';

    await page.fill('#paste-content', testContent);
    await page.click('#create-paste');

    // Wait for navigation to paste view
    await page.waitForURL(/\/\?[a-f0-9]{16}#/);

    // Verify URL format: /?{16-char-hex-id}#{base58-key}
    const url = page.url();
    expect(url).toMatch(/\/\?[a-f0-9]{16}#[1-9A-HJ-NP-Za-km-z]+$/);

    // Verify paste content is displayed
    await expect(page.locator('#paste-text')).toContainText(testContent);
  });

  test('should have expiration options', async ({ page }) => {
    const expireSelect = page.locator('#expire');
    await expect(expireSelect).toBeVisible();

    // Check that common expiration options exist
    const options = await expireSelect.locator('option').allTextContents();
    expect(options.length).toBeGreaterThan(0);
  });

  test('should show error for empty content', async ({ page }) => {
    await page.click('#create-paste');

    // Should show error alert
    await expect(page.locator('#alert')).toBeVisible();
    await expect(page.locator('#alert')).toHaveClass(/alert-error/);
  });
});

test.describe('Paste Actions', () => {
  let pasteUrl: string;
  const testContent = 'Test content for action buttons';

  test.beforeEach(async ({ page }) => {
    // Create a paste first
    await page.goto('/');
    await page.fill('#paste-content', testContent);
    await page.click('#create-paste');
    await page.waitForURL(/\/\?[a-f0-9]{16}#/);
    pasteUrl = page.url();
  });

  test('should clone paste content to editor', async ({ page }) => {
    // Click clone button
    await page.click('#clone-paste');

    // Should switch to create mode
    await expect(page.locator('#new-paste')).toBeVisible();
    await expect(page.locator('#view-paste')).toBeHidden();

    // Content should be in textarea
    const textarea = page.locator('#paste-content');
    await expect(textarea).toHaveValue(testContent);
  });

  test('should copy URL to clipboard', async ({ page, context }) => {
    // Grant clipboard permissions
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);

    await page.click('#paste-url');

    // Should show success alert
    await expect(page.locator('#alert')).toContainText('URL copied');
    await expect(page.locator('#alert')).toHaveClass(/alert-success/);

    // Verify clipboard content
    const clipboardText = await page.evaluate(() => navigator.clipboard.readText());
    expect(clipboardText).toBe(pasteUrl);
  });

  test('should open raw content in new tab', async ({ page, context }) => {
    // Listen for new page (tab)
    const pagePromise = context.waitForEvent('page');

    await page.click('#raw-paste');

    const newPage = await pagePromise;
    await newPage.waitForLoadState();

    // Should be a blob URL with plain text
    expect(newPage.url()).toMatch(/^blob:/);

    // Verify content
    const content = await newPage.textContent('body');
    expect(content).toBe(testContent);

    await newPage.close();
  });

  test('should delete paste with confirmation', async ({ page }) => {
    // Get paste ID from URL
    const pasteId = page.url().split('?')[1].split('#')[0];

    // Handle confirmation dialog
    page.on('dialog', dialog => dialog.accept());

    await page.click('#delete-paste');

    // Should show success alert
    await expect(page.locator('#alert')).toContainText('deleted');

    // Should redirect to home page
    await page.waitForURL('/');

    // Verify paste no longer exists
    await page.goto(`/?${pasteId}`);
    await expect(page.locator('#alert')).toContainText('not found');
  });

  test('should cancel delete when dialog dismissed', async ({ page }) => {
    // Handle confirmation dialog - dismiss it
    page.on('dialog', dialog => dialog.dismiss());

    await page.click('#delete-paste');

    // Should still be on paste page
    expect(page.url()).toBe(pasteUrl);

    // Content should still be visible
    await expect(page.locator('#paste-text')).toContainText(testContent);
  });
});

test.describe('Paste Viewing', () => {
  test('should show 404 for non-existent paste', async ({ page }) => {
    await page.goto('/?0000000000000000#fakekey');

    await expect(page.locator('#alert')).toContainText('not found');
  });

  test('should show error without decryption key', async ({ page }) => {
    // First create a paste
    await page.goto('/');
    await page.fill('#paste-content', 'Test content');
    await page.click('#create-paste');
    await page.waitForURL(/\/\?[a-f0-9]{16}#/);

    // Get paste ID without key
    const pasteId = page.url().split('?')[1].split('#')[0];

    // Navigate to paste without key fragment
    await page.goto(`/?${pasteId}`);

    // Should show error about missing key
    await expect(page.locator('#alert')).toBeVisible();
  });
});
