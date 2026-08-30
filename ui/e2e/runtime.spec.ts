import { expect, test } from '@playwright/test';

test('real control plane conversation, reload, review, settings and demos', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByText('Vivy', { exact: true }).first()).toBeVisible();
  await expect(page.getByText('还没有会话')).toHaveCount(0);
  await expect(page.getByPlaceholder('输入消息... (Enter 发送)')).toBeVisible();
  await expect(page.getByRole('button', { name: '附件' })).toBeVisible();
  await expect(page.getByRole('button', { name: '新建会话' })).toBeVisible();
  // 语音与桌面伙伴已删除
  await expect(page.getByRole('button', { name: '语音' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: '打开伙伴' })).toHaveCount(0);
  await expect(page.getByRole('progressbar', { name: '上下文占用' })).toBeVisible();
  // 全新上下文会自动弹出欢迎向导，跳过后才不影响后续点击
  const welcomeDialog = page.getByRole('dialog', { name: '欢迎使用 Vivy' });
  await expect(welcomeDialog).toBeVisible();
  await welcomeDialog.getByRole('button', { name: '跳过向导' }).click();
  await expect(welcomeDialog).toBeHidden();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByRole('button', { name: '打开导航' })).toBeVisible();
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.setViewportSize({ width: 1280, height: 720 });
  await expect(page.getByRole('button', { name: '收起导航' })).toBeVisible();
  const composer = page.getByPlaceholder('输入消息... (Enter 发送)');
  await composer.fill('hello vivy');
  await page.getByTitle('发送').click();
  const continueButton = page.getByRole('button', { name: '继续' });
  const continueIfNeeded = async () => {
    if (await continueButton.waitFor({ state: 'visible', timeout: 2_000 }).then(() => true).catch(() => false)) await continueButton.click();
  };
  await continueIfNeeded();
  await expect(page.getByText('mock reply to: hello vivy')).toBeVisible({ timeout: 15_000 });
  // 消息功能栏（对照 Agent-DIVA 移植）：助手消息有复制/重新生成，回退与分叉为占位
  const assistantArticle = page.locator('article').filter({ hasText: 'mock reply to: hello vivy' }).last();
  await expect(assistantArticle.getByRole('button', { name: '复制' })).toBeVisible();
  await expect(assistantArticle.getByRole('button', { name: '重新生成' })).toBeEnabled();
  await expect(assistantArticle.getByRole('button', { name: '回到这里' })).toBeDisabled();
  await expect(assistantArticle.getByRole('button', { name: '从此分叉' })).toBeDisabled();
  const userArticle = page.locator('article').filter({ hasText: 'hello vivy' }).first();
  await expect(userArticle.getByRole('button', { name: '编辑' })).toBeDisabled();
  // 用户消息操作栏（参考 ChatGPT）：复制 + 编辑，悬停浮现、平时隐藏；无回退 / 分叉
  const userActions = userArticle.getByRole('button', { name: '复制' }).locator('..');
  await expect(userActions).toHaveCSS('opacity', '0');
  await userArticle.hover();
  await expect(userActions).toHaveCSS('opacity', '1');
  await expect(userArticle.getByRole('button', { name: '回到这里' })).toHaveCount(0);
  await expect(userArticle.getByRole('button', { name: '从此分叉' })).toHaveCount(0);
  // 复制：点击后按钮切换为「已复制」，剪贴板内容为该消息文本
  await page.context().grantPermissions(['clipboard-read', 'clipboard-write']);
  await assistantArticle.getByRole('button', { name: '复制' }).click();
  await expect(assistantArticle.getByRole('button', { name: '已复制' })).toBeVisible();
  expect(await page.evaluate(() => navigator.clipboard.readText())).toContain('mock reply to: hello vivy');
  await page.reload();
  await expect(page.getByText('mock reply to: hello vivy')).toBeVisible({ timeout: 15_000 });

  await page.getByRole('button', { name: '新建会话' }).click();
  await expect(page.getByText('发送消息后，Vivy 会先进行预检。')).toBeVisible();
  await page.getByPlaceholder('输入消息... (Enter 发送)').fill('e2e approval: save a note');
  await page.getByTitle('发送').click();
  await expect(continueButton).toBeVisible({ timeout: 5_000 });
  await continueButton.click();
  await page.getByRole('button', { name: '审批中心' }).click();
  await expect(page.getByRole('dialog').getByText('审批中心', { exact: true })).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  const approvalDetails = page.getByText('审批详情');
  await expect.poll(async () => {
    if (await approvalDetails.isVisible().catch(() => false)) return true;
    await page.getByRole('button', { name: '刷新' }).click();
    return approvalDetails.isVisible().catch(() => false);
  }, { timeout: 15_000 }).toBe(true);
  await page.getByRole('button', { name: '批准' }).click();
  await expect(page.getByText('已批准', { exact: true }).first()).toBeVisible();
  await page.getByRole('button', { name: '关闭' }).click();
  await expect(page.getByRole('dialog')).toBeHidden();
  await page.setViewportSize({ width: 1280, height: 720 });
  await expect(page.getByRole('button', { name: '收起导航' })).toBeVisible();

  await page.getByRole('link', { name: '设置' }).click();
  await page.getByRole('tab', { name: 'Vivy 功能' }).click();
  await page.getByRole('link', { name: '打开生命周期' }).click();
  await expect(page.getByText('当前 Species')).toBeVisible();
  expect(await page.evaluate(() => Object.keys(localStorage).filter((key) => key.startsWith('vivy.demo.')))).toEqual([]);

  await page.getByRole('link', { name: '设置' }).click();
  await page.getByRole('tab', { name: '模型' }).click();
  await expect(page.getByText('密钥只由运行环境管理')).toBeVisible();
  await page.getByRole('link', { name: '人格' }).click();
  await expect(page.getByRole('heading', { name: '人格' })).toBeVisible();
  await expect(page.getByRole('button', { name: /IDENTITY\.MD/ })).toBeVisible();
  await page.getByRole('link', { name: '面具' }).click();
  await expect(page.getByRole('heading', { name: '面具', exact: true })).toBeVisible();
  await expect(page.getByText('面具库')).toBeVisible();
  // 新建会话入口已移入主页聊天框，先回到聊天页再创建
  await page.getByRole('link', { name: '聊天' }).click();
  await page.getByRole('button', { name: '新建会话' }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole('button', { name: '附件' })).toBeVisible();

  await page.getByRole('link', { name: '中控台' }).click();
  await expect(page.getByRole('heading', { name: '中控台' })).toBeVisible();
  await expect(page.getByRole('tab', { name: '概览' })).toBeVisible();
  await page.getByRole('tab', { name: 'Token' }).click();
  await expect(page.getByText('总 Token', { exact: true })).toBeVisible();
  await page.getByRole('link', { name: '记忆' }).click();
  await expect(page.getByText('回答偏好', { exact: true }).first()).toBeVisible();
  await page.getByPlaceholder('搜索记忆').fill('不存在的记忆');
  await expect(page.getByText('没有匹配的记忆')).toBeVisible();
  await page.getByRole('link', { name: 'MCP' }).click();
  await expect(page.getByRole('heading', { name: 'MCP 服务' })).toBeVisible();
  await expect(page.getByText('尚未配置 MCP 服务')).toBeVisible();
  await expect(page.locator('main strong', { hasText: '演示 / 本地模拟' })).toHaveCount(0);
  expect(await page.evaluate(() => Object.keys(localStorage).filter((key) => key.startsWith('vivy.demo.mcp')))).toEqual([]);

  await page.getByRole('link', { name: '记事本' }).click();
  const demoBanner = page.locator('main strong', { hasText: '演示 / 本地模拟' });
  await expect(demoBanner).toBeVisible();
  await page.getByRole('tab', { name: '搜索' }).click();
  await page.getByPlaceholder('搜索会话内容...').fill('你好');
  await page.getByRole('button', { name: '搜索' }).click();
  await expect(page.getByText('你好', { exact: true })).toBeVisible();
  await page.getByRole('tab', { name: '报告' }).click();
  await expect.poll(() => page.evaluate(() => Object.keys(localStorage).some((key) => key.startsWith('vivy.demo.')))).toBe(true);
  const keys = await page.evaluate(() => Object.keys(localStorage));
  expect(keys.some((key) => key.startsWith('vivy.demo.'))).toBe(true);
  expect(keys.every((key) => key === 'vivy.ui.activeSession' || key === 'vivy.ui.welcome.completed' || key.startsWith('vivy.demo.'))).toBe(true);
  await page.getByRole('button', { name: /每日工作摘要/ }).click();
  await expect(page.getByRole('heading', { name: /每日工作摘要/ })).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(demoBanner).toBeVisible();
  await expect(page.getByRole('heading', { name: /每日工作摘要/ })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.getByRole('button', { name: '返回' }).click();
  await expect(page.getByRole('tab', { name: '报告' })).toBeVisible();
  const openNavigation = page.getByRole('button', { name: '打开导航' });
  if (await openNavigation.isVisible().catch(() => false)) await openNavigation.click();
  await page.getByRole('link', { name: '记忆' }).click();
  await page.getByRole('button', { name: /回答偏好/ }).click();
  await expect(page.getByRole('button', { name: '返回' })).toBeVisible();
  await page.getByRole('button', { name: '返回' }).click();
  await expect(page.getByPlaceholder('搜索记忆')).toBeVisible();
  if (await openNavigation.isVisible().catch(() => false)) await openNavigation.click();
  await page.getByRole('link', { name: '设置' }).click();
  await page.getByRole('tab', { name: 'Vivy 功能' }).click();
  await page.getByRole('link', { name: '打开生命周期' }).click();
  await expect(page.getByRole('tab', { name: 'Promotions' })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});
