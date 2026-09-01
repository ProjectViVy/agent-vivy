import { expect, test } from '@playwright/test';

// hitl-review-center.md §UI：Run Inspector 与 Review Center 必须用同一张
// 审批卡（ReviewCard）做内联决策。本 spec 只验证 Inspector 的 Review 入口
// 在无 provider 环境可达且空态正确。
test('Run Inspector 提供 Review 内联审批入口', async ({ page }) => {
  await page.goto('/');
  const welcomeDialog = page.getByRole('dialog', { name: '欢迎使用 Vivy' });
  if (await welcomeDialog.waitFor({ state: 'visible', timeout: 5_000 }).then(() => true).catch(() => false)) {
    await welcomeDialog.getByRole('button', { name: '跳过向导' }).click();
    await expect(welcomeDialog).toBeHidden();
  }
  await page.getByRole('link', { name: '设置' }).click();
  await page.getByRole('tab', { name: 'Vivy 功能' }).click();
  const reviewTab = page.getByRole('tab', { name: /^审批/ });
  await expect(reviewTab).toBeVisible();
  await reviewTab.click();
  await expect(page.getByText('此 Run 没有审批或提问')).toBeVisible();
  // 与 Review Center 共用同一张卡：待审批时内联决策按钮来自 approvals 词条
  await expect(page.getByRole('button', { name: '批准' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: '拒绝' })).toHaveCount(0);
});
