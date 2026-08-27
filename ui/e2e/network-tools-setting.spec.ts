import { expect, test } from '@playwright/test';

// 设置 → 网络工具 真实路径：NetworkToolsCard 读取 settings/get 的真实 provider
// 名单与可用性（密钥仅存在性），选择首选 provider 后经 settings/update 持久化，
// 刷新后保持。覆盖 ui/src/components/settings/NetworkToolsCard.tsx 与 RPC
// settings/get|update 的 network_search 分区。
test('settings network tools tab renders real roster and persists preference', async ({ page }) => {
  // 跳过首访欢迎向导，避免遮挡设置页
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/settings?tab=network');

  // deep-link 落在网络工具分区，且渲染真实卡片（来自 settings/get，非假数据）
  await expect(page.getByRole('tab', { name: '网络工具' })).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByText(/EINO 原生网络工具配置/)).toBeVisible();

  // 真实 roster：免密钥的 duckduckgo / wikipedia 恒为「已配置」，
  // 需要密钥的 bing/google/searxng 在无环境变量时显示「待配置」。
  await expect(page.getByText('DuckDuckGo')).toBeVisible();
  await expect(page.getByText('Wikipedia')).toBeVisible();
  await expect(page.getByText('已配置')).toHaveCount(2);
  await expect(page.getByText('待配置')).toHaveCount(3);
  await expect(page.getByText(/需要环境变量 BING_SEARCH_API_KEY/)).toBeVisible();

  // 选择首选 provider（wikipedia，免密钥可立即使用）并保存
  await page.getByRole('combobox').click();
  await page.getByRole('option', { name: 'Wikipedia', exact: true }).click();
  await page.getByRole('button', { name: '保存' }).click();
  await expect(page.getByText('已保存，下次启动生效')).toBeVisible();

  // 刷新后保持（settings/update 持久化到 agent-home settings.yaml）
  await page.reload();
  await expect(page.getByRole('tab', { name: '网络工具' })).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByRole('combobox')).toContainText('Wikipedia');

  // 恢复自动（清空首选），保持环境干净
  await page.getByRole('combobox').click();
  await page.getByRole('option', { name: /^自动/ }).click();
  await page.getByRole('button', { name: '保存' }).click();
  await expect(page.getByText('已保存，下次启动生效')).toBeVisible();
});