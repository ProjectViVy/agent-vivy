import http from 'node:http';
import { expect, test, type Server } from '@playwright/test';

/**
 * Settings → 模型：模型列表「刷新」从上游 OpenAI 兼容端点 GET /models 拉取
 * 并保存到本地注册表；手动新增的模型在刷新后保留；密钥不出现在界面。
 *
 * 真实路径：页面点击 → 后端（本 e2e 自起的 vivy.exe）→ 本测试进程内的本地
 * /models 服务，全程无外网、无其它会话端口（:8799 独立地址）。
 */
let upstream: Server;
let upstreamURL = '';
let lastAuth = '';

const UPSTREAM_MODELS = ['gpt-4o', 'gpt-4o-mini'];
const ROW = 'E2E Gateway 自定义'; // 供应商行按钮的可访问名（显示名 + 自定义徽标）

test.beforeAll(async () => {
  upstream = http.createServer((req, res) => {
    lastAuth = req.headers.authorization ?? '';
    res.setHeader('Content-Type', 'application/json');
    if (req.url?.endsWith('/models')) {
      res.end(JSON.stringify({ data: UPSTREAM_MODELS.map((id) => ({ id })) }));
      return;
    }
    res.statusCode = 404;
    res.end('{}');
  });
  await new Promise<void>((resolve) => upstream.listen(0, '127.0.0.1', resolve));
  const address = upstream.address();
  if (!address || typeof address === 'string') throw new Error('upstream did not bind');
  upstreamURL = `http://127.0.0.1:${address.port}`;
});

test.afterAll(async () => {
  await new Promise<void>((resolve, reject) => upstream.close((err) => (err ? reject(err) : resolve())));
});

test('model list refresh syncs upstream models and persists them locally', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/settings?tab=model');

  // 新增自定义供应商（base_url 指向本地 /models 服务，key 只在本机测试内）。
  await page.getByRole('button', { name: '新增自定义供应商' }).click();
  await page.getByLabel('显示名').fill('E2E Gateway');
  await page.getByLabel('Base URL').fill(upstreamURL);
  await page.getByLabel('API Key').fill('sk-e2e-secret');
  await page.getByRole('button', { name: '保存' }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.getByRole('button', { name: ROW })).toBeVisible();

  // 选中该供应商：初始列表为空。
  await page.getByRole('button', { name: ROW }).click();
  await expect(page.getByText('该供应商暂无模型，可点击上方「新增」按钮添加。')).toBeVisible();

  // 点击「刷新」：上游模型出现，密钥以 Bearer 上行，成功后给出同步反馈。
  await page.getByRole('button', { name: '刷新模型列表' }).click();
  await expect(page.getByText('已从上游同步 2 个模型')).toBeVisible();
  await expect(page.getByText('gpt-4o', { exact: true })).toBeVisible();
  await expect(page.getByText('gpt-4o-mini', { exact: true })).toBeVisible();
  expect(lastAuth).toBe('Bearer sk-e2e-secret');

  // 手动新增一个模型，再次刷新后仍保留（并集策略）。
  await page.getByRole('button', { name: '新增模型' }).click();
  await page.getByPlaceholder('输入模型 id，回车应用').fill('my-local-model');
  await page.getByRole('button', { name: '加入并应用' }).click();
  await expect(page.getByRole('button', { name: 'my-local-model', exact: true })).toBeVisible();
  await page.getByRole('button', { name: '刷新模型列表' }).click();
  await expect(page.getByText('已从上游同步 3 个模型')).toBeVisible();
  await expect(page.getByRole('button', { name: 'my-local-model', exact: true })).toBeVisible();

  // 刷新落盘：重载后（来自后端注册表）模型列表仍在，密钥不出现。
  await page.reload();
  await expect(page.getByRole('button', { name: ROW })).toBeVisible();
  await expect(page.getByText('gpt-4o', { exact: true })).toBeVisible();
  await expect(page.getByText('gpt-4o-mini', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'my-local-model', exact: true })).toBeVisible();
  await expect(page.getByText('sk-e2e-secret')).toHaveCount(0);
});