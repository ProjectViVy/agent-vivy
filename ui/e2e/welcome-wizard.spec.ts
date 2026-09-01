import { expect, test } from '@playwright/test';

// 欢迎向导 e2e：首访自动弹出 → 跳过 → 重跑 → 模型保存 → deep-link。
// 覆盖 ui/src/components/layout/WelcomeWizard.tsx 与 hooks/use-welcome.ts 的真实路径。
test('welcome wizard first-run, skip, rerun, save and deep link', async ({ page }) => {
  await page.goto('/');
  const dialog = page.getByRole('dialog', { name: '欢迎使用 Vivy' });

  // 全新浏览器上下文：向导在初始化完成后自动弹出
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole('heading', { name: '开启你的 Vivy 之旅' })).toBeVisible();

  // 跳过向导：写入完成标记并关闭
  await dialog.getByRole('button', { name: '跳过向导' }).click();
  await expect(dialog).toBeHidden();
  expect(await page.evaluate(() => localStorage.getItem('vivy.ui.welcome.completed'))).toBe('1');

  // 已完成后刷新不再自动弹出
  await page.reload();
  await expect(page.getByPlaceholder('输入消息... (Enter 发送)')).toBeVisible();
  await expect(page.getByRole('dialog', { name: '欢迎使用 Vivy' })).toHaveCount(0);

  // 设置页通用分区的重跑入口
  await page.getByRole('link', { name: '设置' }).click();
  await page.getByRole('button', { name: '重新运行向导' }).click();
  await expect(dialog).toBeVisible();

  // 模型步骤：预填来自真实配置默认值；没有凭证时仍允许先保存选择，
  // 后续发送由运行时返回明确的供应商连接错误。
  await dialog.getByRole('button', { name: '下一步' }).click();
  await expect(dialog.getByRole('heading', { name: '配置模型' })).toBeVisible();
  await expect(dialog.getByRole('textbox', { name: 'Provider' })).toHaveValue('openai');
  await expect(dialog.getByRole('textbox', { name: '默认模型' })).toHaveValue('gpt-4o-mini');
  await expect(dialog.getByText('API Key 只写入本机用户工作区，不会回传界面。下一条消息即走该供应商。')).toBeVisible();

  // 保留真实 provider 选择并保存，进入完成步骤
  await dialog.getByRole('button', { name: '下一步' }).click();
  await expect(dialog.getByRole('heading', { name: '准备就绪！' })).toBeVisible();

  // 完成卡片「模型设置」：写标记、关向导并 deep-link 到设置页模型分区
  await dialog.getByRole('button', { name: '模型设置' }).click();
  await expect(dialog).toBeHidden();
  await expect(page).toHaveURL(/\/settings\?tab=model/);
  await expect(page.getByRole('tab', { name: '模型' })).toHaveAttribute('aria-selected', 'true');
  // 模型 tab 已重构为供应商注册表 UI（无 Provider 输入框）：断言卡片与当前
  // 供应商行（可访问名 "OpenAI 当前"；顶栏切换按钮的 aria-label 也含 OpenAI，
  // 需用整名匹配避开）
  await expect(page.getByText('已选模型', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'OpenAI 当前' })).toBeVisible();

  // 完成后再次刷新：向导保持关闭
  await page.reload();
  await expect(page.getByRole('tab', { name: '模型' })).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByRole('dialog', { name: '欢迎使用 Vivy' })).toHaveCount(0);
});
