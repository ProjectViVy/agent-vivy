import { expect, test } from '@playwright/test';

test('settings sandbox tab persists default permission', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/settings?tab=sandbox');

  await expect(page.getByRole('tab', { name: '沙箱' })).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByText(/新会话的默认权限/)).toBeVisible();
  await expect(page.getByRole('tab', { name: '沙箱' })).not.toContainText('预览');

  await page.getByRole('combobox').click();
  await page.getByRole('option', { name: '谨慎' }).click();
  await page.getByRole('button', { name: '保存' }).click();
  await expect(page.getByText('已保存，新会话使用该默认权限')).toBeVisible();

  await page.reload();
  await expect(page.getByRole('tab', { name: '沙箱' })).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByRole('combobox')).toContainText('谨慎');

  await page.getByRole('combobox').click();
  await page.getByRole('option', { name: '智能' }).click();
  await page.getByRole('button', { name: '保存' }).click();
  await expect(page.getByText('已保存，新会话使用该默认权限')).toBeVisible();
});
