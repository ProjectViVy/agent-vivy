import { expect, test } from '@playwright/test';

test('browser tab title is VIVY', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/');
  await expect(page).toHaveTitle('VIVY');
});
