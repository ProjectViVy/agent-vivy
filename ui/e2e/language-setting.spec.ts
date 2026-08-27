import { expect, test } from '@playwright/test';

// 设置 → 语言 真实路径：LanguagePicker 切换全局界面语言并持久化到浏览器。
// 覆盖 ui/src/components/settings/LanguagePicker.tsx 与 ui/src/i18n 的模块级 store。
test('settings language tab switches interface language and persists', async ({ page }) => {
  // 跳过首访欢迎向导，避免遮挡设置页
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/settings?tab=language');

  // deep-link 落在语言分区，且显示的是中文文案（默认语言 zh）
  await expect(page.getByRole('tab', { name: '语言' })).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByText(/选择界面语言/)).toBeVisible();
  const zhOption = page.getByRole('button').filter({ hasText: 'CN' });
  const enOption = page.getByRole('button').filter({ hasText: 'EN' });
  await expect(zhOption).toBeVisible();
  await expect(enOption).toBeVisible();

  // 点击 English：界面立即切换为英文并持久化
  await enOption.click();
  await expect(page.getByText(/Pick the interface language/)).toBeVisible();
  await expect(page.getByText('Current language')).toBeVisible();
  expect(await page.evaluate(() => localStorage.getItem('vivy.language'))).toBe('en');
  expect(await page.evaluate(() => document.documentElement.lang)).toBe('en');

  // 刷新后保持英文（localStorage 持久化生效）
  await page.reload();
  await expect(page.getByRole('tab', { name: '语言' })).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByText(/Pick the interface language/)).toBeVisible();

  // 切回简体中文，恢复中文文案
  await page.getByRole('button').filter({ hasText: 'CN' }).click();
  await expect(page.getByText(/选择界面语言/)).toBeVisible();
  expect(await page.evaluate(() => localStorage.getItem('vivy.language'))).toBe('zh');
});