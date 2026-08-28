import { expect, test } from '@playwright/test';

test('mcp page manages live HTTP servers without demo storage', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/mcp');

  await expect(page.getByRole('heading', { name: 'MCP 服务' })).toBeVisible();
  await expect(page.getByText('尚未配置 MCP 服务')).toBeVisible();
  await expect(page.locator('main strong', { hasText: '演示 / 本地模拟' })).toHaveCount(0);

  await page.getByRole('button', { name: '添加服务' }).first().click();
  await page.getByLabel('服务名称').fill('docs');
  await page.getByLabel('服务地址').fill('https://docs.example.com/mcp');
  await page.getByLabel('认证环境变量').fill('MCP_DOCS_TOKEN');
  await page.getByRole('button', { name: '添加服务' }).last().click();

  await expect(page.getByText('docs', { exact: true })).toBeVisible();
  await expect(page.getByText('https://docs.example.com/mcp')).toBeVisible();
  await expect(page.getByRole('switch', { name: 'docs 启用状态' })).toBeChecked();
  expect(await page.evaluate(() => Object.keys(localStorage).filter((key) => key.startsWith('vivy.demo.')))).toEqual([]);

  await page.reload();
  await expect(page.getByText('docs', { exact: true })).toBeVisible();
  await expect(page.getByRole('switch', { name: 'docs 启用状态' })).toBeChecked();

  await page.getByRole('switch', { name: 'docs 启用状态' }).click();
  await expect(page.getByRole('switch', { name: 'docs 启用状态' })).not.toBeChecked();

  await page.getByRole('button', { name: '删除 docs' }).click();
  await page.getByRole('button', { name: '删除服务' }).click();
  await expect(page.getByText('尚未配置 MCP 服务')).toBeVisible();
});
