import { test, expect } from '@playwright/test';

test.describe('Password Protection', () => {
  const testPassword = 'secretpassword123';
  const testContent = 'This is password-protected content';

  test('should create password-protected paste', async ({ page }) => {
    await page.goto('/');

    await page.fill('#paste-content', testContent);
    await page.fill('#password', testPassword);
    await page.click('#create-paste');

    // Wait for paste creation
    await page.waitForURL(/\/\?[a-f0-9]{16}#/);

    // Paste should be created (content visible because same session)
    const url = page.url();
    expect(url).toMatch(/\/\?[a-f0-9]{16}#[1-9A-HJ-NP-Za-km-z]+$/);
  });

  test('should show password prompt for protected paste', async ({ page, context }) => {
    // Create a password-protected paste
    await page.goto('/');
    await page.fill('#paste-content', testContent);
    await page.fill('#password', testPassword);
    await page.click('#create-paste');
    await page.waitForURL(/\/\?[a-f0-9]{16}#/);

    const pasteUrl = page.url();

    // Open in a new page (new context simulates different user)
    const newPage = await context.newPage();
    await newPage.goto(pasteUrl);

    // Should show password prompt (decryption fails without password)
    await expect(newPage.locator('#password-prompt')).toBeVisible();
    await expect(newPage.locator('#paste-output')).toBeHidden();

    await newPage.close();
  });

  test('should decrypt with correct password', async ({ page, context }) => {
    // Create a password-protected paste
    await page.goto('/');
    await page.fill('#paste-content', testContent);
    await page.fill('#password', testPassword);
    await page.click('#create-paste');
    await page.waitForURL(/\/\?[a-f0-9]{16}#/);

    const pasteUrl = page.url();

    // Open in new page
    const newPage = await context.newPage();
    await newPage.goto(pasteUrl);

    // Wait for password prompt
    await expect(newPage.locator('#password-prompt')).toBeVisible();

    // Enter password and decrypt
    await newPage.fill('#decrypt-password', testPassword);
    await newPage.click('#decrypt-btn');

    // Content should be visible
    await expect(newPage.locator('#paste-output')).toBeVisible();
    await expect(newPage.locator('#paste-text')).toContainText(testContent);

    await newPage.close();
  });

  test('should show error for wrong password', async ({ page, context }) => {
    // Create a password-protected paste
    await page.goto('/');
    await page.fill('#paste-content', testContent);
    await page.fill('#password', testPassword);
    await page.click('#create-paste');
    await page.waitForURL(/\/\?[a-f0-9]{16}#/);

    const pasteUrl = page.url();

    // Open in new page
    const newPage = await context.newPage();
    await newPage.goto(pasteUrl);

    // Enter wrong password
    await newPage.fill('#decrypt-password', 'wrongpassword');
    await newPage.click('#decrypt-btn');

    // Should show error
    await expect(newPage.locator('#alert')).toBeVisible();
    await expect(newPage.locator('#alert')).toContainText('wrong password', { ignoreCase: true });

    // Content should still be hidden
    await expect(newPage.locator('#paste-output')).toBeHidden();

    await newPage.close();
  });

  test('should decrypt on Enter key in password field', async ({ page, context }) => {
    // Create a password-protected paste
    await page.goto('/');
    await page.fill('#paste-content', testContent);
    await page.fill('#password', testPassword);
    await page.click('#create-paste');
    await page.waitForURL(/\/\?[a-f0-9]{16}#/);

    const pasteUrl = page.url();

    // Open in new page
    const newPage = await context.newPage();
    await newPage.goto(pasteUrl);

    // Enter password and press Enter
    await newPage.fill('#decrypt-password', testPassword);
    await newPage.press('#decrypt-password', 'Enter');

    // Content should be visible
    await expect(newPage.locator('#paste-output')).toBeVisible();
    await expect(newPage.locator('#paste-text')).toContainText(testContent);

    await newPage.close();
  });

  test('password field should be optional', async ({ page }) => {
    await page.goto('/');

    // Create paste without password
    await page.fill('#paste-content', 'No password needed');
    // Leave password field empty
    await page.click('#create-paste');

    // Should create successfully
    await page.waitForURL(/\/\?[a-f0-9]{16}#/);

    // Content should be immediately visible (no password prompt)
    await expect(page.locator('#paste-output')).toBeVisible();
    await expect(page.locator('#password-prompt')).toBeHidden();
  });
});
