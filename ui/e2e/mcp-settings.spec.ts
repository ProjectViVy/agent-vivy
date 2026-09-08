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

test('mcp page persists a raw-argv STDIO server and projects missing child env', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/mcp');

  await page.getByRole('button', { name: '添加服务' }).first().click();
  await page.getByLabel('服务名称').fill('local');
  await page.getByLabel('传输方式').click();
  await page.getByRole('option', { name: 'STDIO' }).click();
  await page.getByLabel('可执行命令').fill('node');
  await page.getByRole('button', { name: '添加参数' }).click();
  await page.getByRole('button', { name: '添加参数' }).click();
  await page.getByLabel('参数 1').fill('  --stdio  ');
  await page.getByLabel('参数 2').fill('');
  await page.getByRole('button', { name: '添加映射' }).click();
  await page.getByLabel('子进程变量').fill('MCP_TOKEN');
  await page.getByLabel('宿主变量').fill('MCP_E2E_MISSING_HOST');
  await page.getByLabel('工作目录').fill('tools');
  await expect(page.getByText(/本地进程以 Vivy/)).toBeVisible();
  await page.getByRole('button', { name: '添加服务' }).last().click();

  await expect(page.getByText('local', { exact: true })).toBeVisible();
  await expect(page.getByText('STDIO', { exact: true })).toBeVisible();
  await expect(page.getByText('node', { exact: true })).toBeVisible();
  await expect(page.getByText(/缺少子进程环境变量：MCP_TOKEN/)).toBeVisible();

  await page.getByLabel('搜索 MCP 服务').fill('MCP_E2E_MISSING_HOST');
  await expect(page.getByText('local', { exact: true })).toBeVisible();
  await page.getByLabel('搜索 MCP 服务').fill('');
  await page.getByRole('switch', { name: 'local 启用状态' }).click();
  await expect(page.getByRole('switch', { name: 'local 启用状态' })).not.toBeChecked();
  await page.reload();
  await expect(page.getByRole('switch', { name: 'local 启用状态' })).not.toBeChecked();

  await page.getByRole('button', { name: '编辑 local' }).click();
  await expect(page.getByLabel('参数 1')).toHaveValue('  --stdio  ');
  await expect(page.getByLabel('参数 2')).toHaveValue('');
  await page.getByRole('button', { name: '取消' }).click();
  await page.getByRole('button', { name: '删除 local' }).click();
  await page.getByRole('button', { name: '删除服务' }).click();
  await expect(page.getByText('尚未配置 MCP 服务')).toBeVisible();
});
