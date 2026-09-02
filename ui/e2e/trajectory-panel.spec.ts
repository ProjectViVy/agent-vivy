import { expect, test } from '@playwright/test';

// UI-TRAJ / UI-TRAJECTORY-DEMO：轨迹面板接真实 `trajectory/session` RPC。
// Provider 无关：真实控制面（e2e 配置，空 Journal）下新建一个会话，
// 面板应渲染会话选择器，且无 run 的会话投影为账本空态。
test('trajectory tab renders real session panel with empty ledger', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/');

  await page.getByRole('link', { name: '聊天' }).click();
  await page.getByRole('button', { name: '新建会话' }).click();
  await expect(page).toHaveURL(/\/$/);

  await page.getByRole('link', { name: '中控台' }).click();
  await expect(page.getByRole('heading', { name: '中控台' })).toBeVisible();
  await page.getByRole('tab', { name: '轨迹' }).click();

  await expect(page.locator('[data-trajectory-panel]')).toBeVisible();
  await expect(page.locator('[data-trajectory-session-select]')).toBeVisible();
  await expect(page.getByText('暂无轨迹记录').first()).toBeVisible();
});
