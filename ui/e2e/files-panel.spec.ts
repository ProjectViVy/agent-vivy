import { expect, test } from '@playwright/test';

test('files panel opens and shows the no-run empty state', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/');

  // 套件内共享同一个后端库：自动选中的首个会话可能带着既往 run（openRun 会
  // 恢复 currentRun），先建一个全新会话保证 no-run 语义属于本会话。先等
  // initialize 完全落定再建会话，避免其尾部自动选中覆盖新会话的选择。
  await expect(page.locator('article').first().or(page.getByText('开始新的对话'))).toBeVisible({ timeout: 15_000 });
  await page.getByRole('button', { name: '新建会话' }).click();
  await expect(page.getByText('开始新的对话')).toBeVisible();

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
