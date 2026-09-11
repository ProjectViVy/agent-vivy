import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test } from '@playwright/test';

async function startMcpFixture() {
  let requestCount = 0;
  let sessionNumber = 0;
  let rejectToolsList = false;
  const server = createServer((request, response) => {
    if (request.method === 'DELETE') {
      requestCount += 1;
      response.writeHead(200).end();
      return;
    }
    if (request.method !== 'POST') {
      response.writeHead(405).end();
      return;
    }
    let body = '';
    request.setEncoding('utf8');
    request.on('data', (chunk) => { body += chunk; });
    request.on('end', () => {
      requestCount += 1;
      let message: { id?: unknown; method?: string } = {};
      try { message = JSON.parse(body) as { id?: unknown; method?: string }; } catch {
        response.writeHead(400).end();
        return;
      }
      if (message.method === 'tools/list' && rejectToolsList) {
        response.writeHead(404).end();
        return;
      }
      if (message.method === 'notifications/initialized') {
        response.writeHead(202).end();
        return;
      }
      if (message.method === 'initialize') {
        sessionNumber += 1;
        response.setHeader('Mcp-Session-Id', `browser-session-${sessionNumber}`);
        response.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify({
          jsonrpc: '2.0', id: message.id,
          result: { protocolVersion: '2024-11-05', capabilities: { tools: {} }, serverInfo: { name: 'browser-fixture', version: '1' } },
        }));
        return;
      }
      if (message.method === 'tools/list') {
        response.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify({
          jsonrpc: '2.0', id: message.id,
          result: { tools: [{ name: 'browser_fixture', description: 'browser fixture', inputSchema: { type: 'object' } }] },
        }));
        return;
      }
      response.writeHead(202).end();
    });
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => resolve());
  });
  const address = server.address() as AddressInfo;
  return {
    endpoint: `http://127.0.0.1:${address.port}/mcp`,
    requests: () => requestCount,
    rejectToolsList: () => { rejectToolsList = true; },
    close: () => new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve())),
  };
}

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

test('mcp snapshot reads do not connect and Probe is explicit activation', async ({ page }) => {
  const fixture = await startMcpFixture();
  let configured = false;
  try {
    await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
    await page.goto('/mcp');
    expect(fixture.requests()).toBe(0);

    await page.getByRole('button', { name: '添加服务' }).first().click();
    await page.getByLabel('服务名称').fill('snapshot-docs');
    await page.getByLabel('服务地址').fill(fixture.endpoint);
    await page.getByRole('button', { name: '添加服务' }).last().click();
    configured = true;
    await expect(page.getByText('snapshot-docs', { exact: true })).toBeVisible();
    await expect(page.getByText('就绪', { exact: true })).toBeVisible();

    const afterSave = fixture.requests();
    expect(afterSave).toBeGreaterThan(0);
    fixture.rejectToolsList();
    await page.reload();
    await expect(page.getByText('snapshot-docs', { exact: true })).toBeVisible();
    await expect(page.getByText('就绪', { exact: true })).toBeVisible();
    expect(fixture.requests()).toBe(afterSave);

    await page.getByRole('button', { name: '探测 snapshot-docs' }).click();
    await expect(page.getByText('不可用', { exact: true })).toBeVisible();
    expect(fixture.requests()).toBeGreaterThan(afterSave);
  } finally {
    if (configured && await page.getByRole('button', { name: '删除 snapshot-docs' }).count()) {
      await page.getByRole('button', { name: '删除 snapshot-docs' }).click();
      await page.getByRole('button', { name: '删除服务' }).click();
    }
    await fixture.close();
  }
});
