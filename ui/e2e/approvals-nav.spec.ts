import { expect, test } from '@playwright/test';

// 审批中心完整路由需有主导航入口（UI-AUDIT-REVIEW-NAV）：此前只能从聊天
// shield 打开 sheet，跨会话 Review Center 的全页面不够可发现。本 spec 不
// 依赖 provider，在无 key 环境同样执行。
test('审批中心从主导航直达', async ({ page }) => {
  await page.goto('/');
  const welcomeDialog = page.getByRole('dialog', { name: '欢迎使用 Vivy' });
  if (await welcomeDialog.waitFor({ state: 'visible', timeout: 5_000 }).then(() => true).catch(() => false)) {
    await welcomeDialog.getByRole('button', { name: '跳过向导' }).click();
    await expect(welcomeDialog).toBeHidden();
  }
  await page.getByRole('link', { name: '审批中心' }).click();
  await expect(page).toHaveURL(/\/approvals$/);
  await expect(page.getByRole('heading', { name: '审批中心' })).toBeVisible();
  await expect(page.getByText('统一处理工具审批和运行中的问题。')).toBeVisible();
  await expect(page.locator('main strong', { hasText: '演示 / 本地模拟' })).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByRole('button', { name: '打开导航' })).toBeVisible();
  const openNavigation = page.getByRole('button', { name: '打开导航' });
  if (await openNavigation.isVisible().catch(() => false)) await openNavigation.click();
  await expect(page.getByRole('link', { name: '审批中心' })).toBeVisible();
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});
