import { expect, test } from '@playwright/test';

// 欢迎向导 e2e：首访自动弹出 → 跳过 → 重跑 → 目录驱动的模型步骤 → deep-link。
// 覆盖 ui/src/components/layout/WelcomeWizard.tsx、welcome-model.ts 与 hooks/use-welcome.ts
// 的真实路径（e2e 后端 providers.active=deepseek，目录来自 internal/provider/data/vendors.yaml）。
// 界面语言用默认的 en：后端 settings.locale 在 initialize 时覆盖 localStorage，
// 所以这里断言英文文案，避免依赖浏览器 locale。
test('welcome wizard first-run, skip, rerun, model step and deep link', async ({ page }) => {
  await page.goto('/');
  const dialog = page.getByRole('dialog', { name: 'Welcome to Vivy' });

  // 全新浏览器上下文：向导在初始化完成后自动弹出
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole('heading', { name: 'Begin your journey with Vivy' })).toBeVisible();

  // 跳过向导：写入完成标记并关闭
  await dialog.getByRole('button', { name: 'Skip wizard' }).click();
  await expect(dialog).toBeHidden();
  expect(await page.evaluate(() => localStorage.getItem('vivy.ui.welcome.completed'))).toBe('1');

  // 已完成后刷新不再自动弹出
  await page.reload();
  await expect(page.getByPlaceholder('Type a message, or / for commands (Enter to send)')).toBeVisible();
  await expect(page.getByRole('dialog', { name: 'Welcome to Vivy' })).toHaveCount(0);

  // 设置页通用分区的重跑入口
  await page.getByRole('link', { name: 'Settings' }).click();
  await page.getByRole('button', { name: 'Run wizard again' }).click();
  await expect(dialog).toBeVisible();

  // 模型步骤：厂商 / 协议端点 / 模型 / 端点地址全部由后端 settings/providers 目录驱动。
  // providers.active=deepseek → 预填 DeepSeek 的 openai-completions 端点与目录默认模型；
  // 不再有自由文本的「协议适配器」「Base URL」输入，也没有跳转到 DeepSeek 平台的按钮。
  await dialog.getByRole('button', { name: 'Next' }).click();
  await expect(dialog.getByRole('heading', { name: 'Configure your model' })).toBeVisible();
  await expect(dialog.getByTestId('welcome-provider')).toContainText('DeepSeek');
  await expect(dialog.getByTestId('welcome-protocol')).toContainText('openai-completions');
  await expect(dialog.getByTestId('welcome-model')).toContainText('deepseek-flash');
  await expect(dialog.getByTestId('welcome-endpoint')).toContainText('https://api.deepseek.com');
  await expect(dialog.getByTestId('welcome-api-key')).toBeVisible();
  await expect(dialog.getByTestId('welcome-api-key')).toHaveValue('');
  await expect(dialog.getByRole('button', { name: 'Open in browser' })).toHaveCount(0);

  // 下拉必须真的能打开并选中：向导遮罩 z-[200] 曾盖住 z-50 的 SelectContent，
  // 选项在 DOM 里但视觉/命中都在遮罩后面。这里点击真实的 option 触发命中测试。
  await dialog.getByTestId('welcome-provider').click();
  await page.getByRole('option', { name: 'MiniMax' }).click();
  await expect(dialog.getByTestId('welcome-provider')).toContainText('MiniMax');
  await dialog.getByTestId('welcome-provider').click();
  await page.getByRole('option', { name: 'DeepSeek' }).click();
  await expect(dialog.getByTestId('welcome-provider')).toContainText('DeepSeek');
  await expect(dialog.getByTestId('welcome-protocol')).toContainText('openai-completions');
  // 切换厂商会清空模型选择；重新选中目录默认模型，顺带覆盖模型下拉。
  await dialog.getByTestId('welcome-model').click();
  await page.getByRole('option', { name: 'deepseek-flash' }).click();
  await expect(dialog.getByTestId('welcome-model')).toContainText('deepseek-flash');

  // 没有已配置密钥、也没有确认运行环境注入时，明确拒绝保存而不是静默写入空配置。
  await dialog.getByRole('button', { name: 'Next' }).click();
  await expect(
    dialog.getByText('Enter an API key, or confirm below that the runtime environment already provides it.'),
  ).toBeVisible();

  // 勾选「运行环境已注入」后允许留空保存；向导不写任何密钥。
  await dialog.getByTestId('welcome-env-key').click();
  await dialog.getByRole('button', { name: 'Next' }).click();
  await expect(dialog.getByRole('heading', { name: 'You are all set!' })).toBeVisible();

  // 完成卡片「模型设置」：写标记、关向导并 deep-link 到设置页模型分区
  await dialog.getByRole('button', { name: 'Model settings' }).click();
  await expect(dialog).toBeHidden();
  await expect(page).toHaveURL(/\/settings\?tab=model/);
  await expect(page.getByRole('tab', { name: 'Model' })).toHaveAttribute('aria-selected', 'true');
  // DeepSeek 自 PROV-P4 起贡献两条协议端点行（openai-completions / anthropic-messages），
  // 可访问名都含 "DeepSeek"，按 testid 精确定位并用 aria-pressed 断言当前选择。
  await expect(page.getByText('Saved models', { exact: true })).toBeVisible();
  await expect(page.locator('[data-testid="provider-row-deepseek-openai-completions"]')).toHaveAttribute('aria-pressed', 'true');

  // 完成后再次刷新：向导保持关闭
  await page.reload();
  await expect(page.getByRole('tab', { name: 'Model' })).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByRole('dialog', { name: 'Welcome to Vivy' })).toHaveCount(0);
});
