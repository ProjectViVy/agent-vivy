import { expect, test } from '@playwright/test';

// NG-23/NG-28（VIVY-STUDIO.md）：换代权威在 Studio，物种只读 inspect。
// 日常 Vivy 的 /lifecycle 不得再暴露 create/eval/promote 写入口。
test('生命周期页只读：不再暴露创建/评测/提升写入口', async ({ page }) => {
  await page.goto('/');
  const welcomeDialog = page.getByRole('dialog', { name: '欢迎使用 Vivy' });
  if (await welcomeDialog.waitFor({ state: 'visible', timeout: 5_000 }).then(() => true).catch(() => false)) {
    await welcomeDialog.getByRole('button', { name: '跳过向导' }).click();
    await expect(welcomeDialog).toBeHidden();
  }
  await page.getByRole('link', { name: '设置' }).click();
  await page.getByRole('tab', { name: 'Vivy 功能' }).click();
  await page.getByRole('link', { name: '打开生命周期' }).click();
  await expect(page.getByText('当前 Species')).toBeVisible();
  await expect(page.getByText('换代权威在 Vivy Studio；此处仅只读检查，不提供创建/评测/提升入口。')).toBeVisible();
  await expect(page.getByRole('button', { name: '创建 Generation' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: '启动评测' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: '确认提升' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: '拒绝' })).toHaveCount(0);
  for (const tab of ['Generations', 'Evals', 'Promotions']) {
    await page.getByRole('tab', { name: tab }).click();
    await expect(page.getByRole('tab', { name: tab })).toHaveAttribute('aria-selected', 'true');
    await expect(page.getByRole('button', { name: '创建 Generation' })).toHaveCount(0);
  }
});
