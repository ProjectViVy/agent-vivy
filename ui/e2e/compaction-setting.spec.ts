import { expect, test } from '@playwright/test';

// 设置 → 通用 → 上下文压缩卡：所有文案必须来自 i18n（zh/en 双语可切）。
// 锁死缺陷 UI-I18N-COMPACTION：卡片曾把 settings.compaction.* 渲染成原始键，
// 且表单标签/占位/反馈混入硬编码中文（英文界面下也显示中文）。
test('compaction card renders localized labels without raw i18n keys', async ({ page }) => {
  // 跳过首访欢迎向导，避免遮挡设置页
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/settings?tab=general');

  // 中文（默认语言）：卡片标题与三个表单标签齐全（CardTitle 是 div，用文本断言）
  await expect(page.getByText('上下文压缩', { exact: true })).toBeVisible();
  await expect(page.getByText('最大 tokens', { exact: true })).toBeVisible();
  await expect(page.getByText('压缩阈值 (%)', { exact: true })).toBeVisible();
  await expect(page.getByText('保留最近消息', { exact: true })).toBeVisible();

  // 不允许出现任何原始 i18n 键（defect 回归线）
  await expect(page.getByText(/settings\.compaction\./)).toHaveCount(0);

  // 语言持久化切到 English（与 LanguagePicker 同一 localStorage 机制），重载生效
  await page.evaluate(() => localStorage.setItem('vivy.language', 'en'));
  await page.reload();

  await expect(page.getByText('Context compaction', { exact: true })).toBeVisible();
  await expect(page.getByText('Max tokens', { exact: true })).toBeVisible();
  await expect(page.getByText('Compaction threshold (%)', { exact: true })).toBeVisible();
  await expect(page.getByText('Keep recent messages', { exact: true })).toBeVisible();
  await expect(page.getByText('0 = model context window (128000 when unknown)')).toBeVisible();
  // 英文界面下压缩卡不得残留中文硬编码（exact 避开 DivaSettingsPreview
  // 迁移说明段落里的硬编码中文——该预览区整体 i18n 是另一笔欠账）
  await expect(page.getByText('最大 tokens', { exact: true })).toHaveCount(0);
  await expect(page.getByText('保存中…', { exact: true })).toHaveCount(0);
  await expect(page.getByText(/settings\.compaction\./)).toHaveCount(0);
});

// CMP-3：压缩历史面板随卡渲染——无会话时给引导文案，有会话且无记录时给空态；
// 历史条目/空态/引导都必须来自 i18n，不得出现原始键。
test('compaction card shows localized compaction history panel', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/settings?tab=general');

  await expect(page.getByText('压缩历史', { exact: true })).toBeVisible();
  await expect(
    page.getByTestId('compaction-history-empty')
      .or(page.getByText('打开一个会话后查看其压缩历史。'))
  ).toBeVisible();
  await expect(page.getByText(/settings\.compaction\.history/)).toHaveCount(0);

  await page.evaluate(() => localStorage.setItem('vivy.language', 'en'));
  await page.reload();

  await expect(page.getByText('Compaction history', { exact: true })).toBeVisible();
  await expect(
    page.getByTestId('compaction-history-empty')
      .or(page.getByText('Open a session to see its compaction history.'))
  ).toBeVisible();
  await expect(page.getByText(/settings\.compaction\.history/)).toHaveCount(0);
});
