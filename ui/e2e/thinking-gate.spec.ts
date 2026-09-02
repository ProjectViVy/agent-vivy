import { expect, test } from '@playwright/test';

// UI-COMPOSER 思考模式 D9 门控：e2e 环境未配置 provider，session/context
// 的 thinking_supported 为 false —— 思考选择器必须整体隐藏（而不是以
// 死控件形式存在），附件等其余工具栏按钮不受影响。
test('thinking selector is hidden without a thinking-capable model', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/');

  await page.getByRole('link', { name: '聊天' }).click();
  await page.getByRole('button', { name: '新建会话' }).click();
  await expect(page).toHaveURL(/\/$/);

  await expect(page.getByRole('button', { name: '思考模式' })).toHaveCount(0);
  await expect(page.getByLabel('附件')).toBeVisible();
  await expect(page.getByRole('textbox', { name: '输入消息... (Enter 发送)' })).toBeVisible();
});
