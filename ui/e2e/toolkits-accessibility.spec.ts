import { expect, test } from '@playwright/test';

test('opens Toolkits with the keyboard and fits a narrow viewport', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/');

  const openNavigation = page.getByRole('button', { name: /open navigation/i });
  await expect(openNavigation).toBeVisible();
  await openNavigation.focus();
  await page.keyboard.press('Enter');

  const vivyNavigation = page.getByRole('button', { name: 'VIVY', exact: true });
  await expect(vivyNavigation).toBeVisible();
  await vivyNavigation.focus();
  await page.keyboard.press('Enter');

  const toolkitsLink = page.getByRole('link', { name: 'Toolkits', exact: true });
  await expect(toolkitsLink).toBeVisible();
  await toolkitsLink.focus();
  await page.keyboard.press('Enter');

  const toolkitPage = page.getByTestId('toolkit-page');
  await expect(toolkitPage).toBeVisible();
  await expect(toolkitPage.getByRole('heading', { name: 'Toolkits', exact: true })).toBeVisible();

  const dimensions = await page.evaluate(() => ({
    viewport: document.documentElement.clientWidth,
    document: document.documentElement.scrollWidth,
  }));
  expect(dimensions.document).toBeLessThanOrEqual(dimensions.viewport);
});
