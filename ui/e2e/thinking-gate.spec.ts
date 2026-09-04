import { expect, test } from '@playwright/test';

// UI-COMPOSER / UI-CHAT-TOOLBAR 门控与伪操作清理：
// 1. 思考模式 D9 门控：e2e 环境未配置 provider，session/context
//    的 thinking_supported 为 false —— 思考选择器必须整体隐藏（而不是以
//    死控件形式存在），附件等其余工具栏按钮不受影响。
// 2. 伪操作清理：AutoDream 图标彻底移除；执行模式下拉仅保留真实支持的智能体与
//    计划模式，询问模式彻底隐退。
test('thinking selector is hidden without a thinking-capable model, and fake controls are removed', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/');

  await page.getByRole('link', { name: '聊天' }).click();
  await page.getByRole('button', { name: '新建会话' }).click();
  await expect(page).toHaveURL(/\/$/);

  await expect(page.getByRole('button', { name: '思考模式' })).toHaveCount(0);
  await expect(page.getByLabel('附件')).toBeVisible();
  await expect(page.getByRole('textbox', { name: '输入消息... (Enter 发送)' })).toBeVisible();

  // 伪操作清理：无 AutoDream 图标/按钮
  await expect(page.getByTitle('手动触发 AutoDream')).toHaveCount(0);
  await expect(page.getByLabel('手动触发 AutoDream')).toHaveCount(0);

  // 执行模式下拉：点击展开，仅见智能体模式与计划模式，不见询问模式
  const modeTrigger = page.getByRole('button', { name: '智能体模式' });
  await expect(modeTrigger).toBeVisible();
  await modeTrigger.click();
  await expect(page.getByRole('menuitem', { name: '智能体模式' })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: '计划模式' })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: '询问模式' })).toHaveCount(0);
});
