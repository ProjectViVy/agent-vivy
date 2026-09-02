import { expect, test } from '@playwright/test';

import { hasRealProvider } from './global-setup';

test('cron page schedules real jobs and closes the trigger loop without demo storage', async ({ page }) => {
  test.skip(!hasRealProvider, 'requires a configured real provider; offline e2e does not use a model double');
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/cron-tasks');

  await expect(page.getByRole('heading', { name: '定时任务' })).toBeVisible();
  await expect(page.getByText('演示 / 本地模拟')).toHaveCount(0);

  // 创建：默认 cron 表达式 0 9 * * *，开启启用开关。
  await page.getByRole('button', { name: '新建任务' }).first().click();
  await page.getByLabel('任务名称').fill('e2e 定时任务');
  await page.getByLabel('任务内容').fill('e2e cron ping');
  await page.getByRole('dialog').getByRole('button', { name: '创建任务' }).click();

  await expect(page.getByText('e2e 定时任务').first()).toBeVisible();
  await expect(page.getByText('已计划').first()).toBeVisible();
  // 下次运行由后端计算，运行计划展示 cron 表达式与时区。
  await expect(page.getByText('0 9 * * * · Asia/Shanghai').first()).toBeVisible();

  // 手动触发 → 真实供应商运行 → 终态回写（ok → 已完成）。
  await page.getByRole('button', { name: '立即运行' }).click();
  await expect(page.getByText('已完成').first()).toBeVisible({ timeout: 20_000 });
  // 任务专属会话被后端创建，可以跳转。
  await expect(page.getByRole('button', { name: '查看会话' })).toBeEnabled();

  expect(await page.evaluate(() => Object.keys(localStorage).filter((key) => key.startsWith('vivy.demo.')))).toEqual([]);

  // 刷新后任务仍在（SQLite 持久化）。
  await page.reload();
  await expect(page.getByText('e2e 定时任务').first()).toBeVisible();
  await expect(page.getByText('已完成').first()).toBeVisible();

  // 清理：选中任务 → 删除 → 确认。
  await page.getByRole('button', { name: /e2e 定时任务/ }).first().click();
  await page.getByRole('button', { name: '删除', exact: true }).click();
  await page.getByRole('button', { name: '删除任务' }).click();
  await expect(page.getByText('还没有定时任务')).toBeVisible();
});

// UI-CRON-P2（可行部分）：`at`（定时一次）暴露进表单——离线可测：选项出现、
// 选中后出现触发时间字段、过去时间被未来校验拦下（不发创建请求，不依赖供应商）。
test('cron form exposes one-shot at scheduling with future-time validation', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/cron-tasks');

  await page.getByRole('button', { name: '新建任务' }).first().click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('combobox').click();
  await page.getByRole('option', { name: '定时一次' }).click();

  const atInput = dialog.locator('#cron-at');
  await expect(atInput).toBeVisible();
  await expect(atInput).toHaveAttribute('type', 'datetime-local');

  await page.getByLabel('任务名称').fill('e2e 一次性任务');
  await page.getByLabel('任务内容').fill('e2e at ping');
  await atInput.fill('2020-01-01T00:00');
  await dialog.getByRole('button', { name: '创建任务' }).click();

  await expect(page.getByText('触发时间必须晚于当前时间。')).toBeVisible();
});
