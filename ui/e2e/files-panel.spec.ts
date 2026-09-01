import { expect, test } from '@playwright/test';

test('files panel opens and shows the no-run empty state', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/');

  const filesButton = page.getByRole('button', { name: '文件', exact: true });
  await expect(filesButton).toBeVisible();
  await filesButton.click();

  const sheetTitle = page.getByRole('heading', { name: '文件' });
  await expect(sheetTitle).toBeVisible();
  await expect(page.getByText('开始一次运行后可查看其工作区文件')).toBeVisible();

  // 关闭后再开，状态保持干净。
  await page.keyboard.press('Escape');
  await expect(sheetTitle).toBeHidden();
  await filesButton.click();
  await expect(page.getByText('开始一次运行后可查看其工作区文件')).toBeVisible();
});
