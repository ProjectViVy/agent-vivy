import { expect, test } from '@playwright/test';

test('welcome wizard first-run, skip, rerun, save and deep link', async ({ page }) => {
  await page.goto('/');
  const dialog = page.getByRole('dialog', { name: '欢迎使用 Vivy' });

  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole('heading', { name: '开启你的 Vivy 之旅' })).toBeVisible();

  await dialog.getByRole('button', { name: '跳过向导' }).click();
  await expect(dialog).toBeHidden();
  expect(await page.evaluate(() => localStorage.getItem('vivy.ui.welcome.completed'))).toBe('1');

  await page.reload();
  await expect(page.getByPlaceholder('输入消息... (Enter 发送)')).toBeVisible();
  await expect(page.getByRole('dialog', { name: '欢迎使用 Vivy' })).toHaveCount(0);

  await page.getByRole('link', { name: '设置' }).click();
  await page.getByRole('button', { name: '重新运行向导' }).click();
  await expect(dialog).toBeVisible();

  await dialog.getByRole('button', { name: '下一步' }).click();
  await expect(dialog.getByRole('heading', { name: '登记第一个供应商' })).toBeVisible();
  await expect(dialog.getByText('API Key 只写入本机用户工作区')).toBeVisible();

  await dialog.getByLabel('显示名').fill('Wizard Gateway');
  await dialog.getByLabel('Base URL').fill('https://wizard.example.com/v1');
  await dialog.getByLabel('默认模型').fill('wizard-model');
  await dialog.getByLabel('API Key').fill('sk-wizard-e2e');
  await dialog.getByRole('button', { name: '下一步' }).click();
  await expect(dialog.getByRole('heading', { name: '准备就绪！' })).toBeVisible();

  await dialog.getByRole('button', { name: '模型设置' }).click();
  await expect(dialog).toBeHidden();
  await expect(page).toHaveURL(/\/settings\?tab=model/);
  await expect(page.getByRole('tab', { name: '模型' })).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByText('Wizard Gateway')).toBeVisible();

  await page.reload();
  await expect(page.getByRole('tab', { name: '模型' })).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByRole('dialog', { name: '欢迎使用 Vivy' })).toHaveCount(0);
});
