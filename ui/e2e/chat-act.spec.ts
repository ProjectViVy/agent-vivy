import { expect, test, type Page } from '@playwright/test';

// UI-CHAT-ACT R2 离线规格（JOURNAL-REWIND-AND-FORK §6.4 离线回放）：
// - 编辑走完整 UI 流（rewind 截点 + 新文本重开回合）；离线无供应商，回合失败
//   但用户消息已入账（对照 runtime.spec 离线分支）。
// - 回退折叠 / 分叉经 RPC 直驱内核：离线下助手气泡不存在（回退/分叉按钮只在
//   助手气泡上），改由内核 RPC 驱动，再验证折叠渲染与会话列表。

type Json = Record<string, unknown>;

async function rpc(page: Page, method: string, params: Json): Promise<Json> {
  return page.evaluate(async ({ method, params }) => {
    const bootstrap = await (await fetch('/rpc/bootstrap', { cache: 'no-store' })).json() as { websocket_path: string; token: string };
    const wsURL = new URL(bootstrap.websocket_path, location.origin);
    wsURL.searchParams.set('token', bootstrap.token);
    const socket = await new Promise<WebSocket>((resolve, reject) => {
      const candidate = new WebSocket(wsURL.toString());
      candidate.onopen = () => resolve(candidate);
      candidate.onerror = () => reject(new Error('websocket connect failed'));
    });
    try {
      return await new Promise<Json>((resolve, reject) => {
        socket.onmessage = (event) => {
          const reply = JSON.parse(String(event.data)) as { id?: string; result?: Json; error?: { code: number; message: string } };
          if (reply.error) reject(new Error(`rpc ${method}: ${reply.error.code} ${reply.error.message}`));
          else if (reply.id) resolve(reply.result ?? {});
        };
        socket.send(JSON.stringify({ jsonrpc: '2.0', id: 'e2e', method, params }));
        setTimeout(() => reject(new Error(`rpc ${method} timeout`)), 5_000);
      });
    } finally {
      socket.close();
    }
  }, { method, params });
}

test('edit reruns via rewind, rewind folds the view, fork copies history to a new session', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/');
  // 先等 initialize 完全落定（首会话已选中：空态或首条消息出现），再建新
  // 会话——否则 initialize 尾部的自动选中会覆盖新建会话的选择，发送被吞。
  await expect(page.locator('article').first().or(page.getByText('开始新的对话'))).toBeVisible({ timeout: 15_000 });
  await page.getByRole('button', { name: '新建会话' }).click();
  await expect(page.getByText('开始新的对话')).toBeVisible();

  const sessionId = await page.evaluate(() => localStorage.getItem('vivy.ui.activeSession'));
  expect(sessionId).toBeTruthy();

  const composer = page.getByPlaceholder('输入消息... (Enter 发送)');
  await composer.fill('hello vivy');
  await page.getByTitle('发送').click();
  // 乐观本地消息先于失败横幅出现；它缺席说明发送被吞，直接在此失败定位。
  await expect(page.locator('article').filter({ hasText: 'hello vivy' })).toHaveCount(1, { timeout: 10_000 });
  await expect(page.getByText('无法连接！请检查供应商配置！')).toBeVisible({ timeout: 15_000 });
  await expect(page.locator('article').filter({ hasText: 'hello vivy' })).toHaveCount(1);

  // 编辑（UI 全流程）：悬停浮现 → 编辑 → 改文 → 保存并重跑 = 回退到该输入 + 重发。
  // 先等终态刷新把乐观本地消息（local-<run>）换成服务端账本消息（msg_ id），
  // 否则编辑流拿到的截点 id 会被内核拒收（ErrInvalidCutoff）。
  const userArticle = page.locator('article').filter({ hasText: 'hello vivy' }).first();
  await expect(userArticle).toHaveAttribute('data-message-id', /^msg_/, { timeout: 10_000 });
  await userArticle.hover();
  await userArticle.getByRole('button', { name: '编辑' }).click();
  const editor = page.getByRole('textbox', { name: '编辑' });
  await expect(editor).toHaveValue('hello vivy');
  await editor.fill('hello vivy (edited)');
  await userArticle.getByRole('button', { name: '保存并重跑' }).click();
  await expect(page.getByText('hello vivy', { exact: true })).toHaveCount(0);
  await expect(page.locator('article').filter({ hasText: 'hello vivy (edited)' })).toHaveCount(1);
  await expect(page.getByText('无法连接！请检查供应商配置！')).toBeVisible({ timeout: 15_000 });

  // 分叉（RPC 直驱内核）：以当前输入为止的历史复制进新会话，原会话不动。
  const listed = await rpc(page, 'session/messages', { session_id: sessionId }) as { messages: Array<{ id: string; role: string; content: string }> };
  const editedMessage = listed.messages.find((message) => message.content === 'hello vivy (edited)');
  expect(editedMessage).toBeTruthy();
  const fork = await rpc(page, 'session/fork', { session_id: sessionId, message_id: editedMessage!.id, title: '分叉分支' }) as { session_id: string; copied_count: number };
  expect(fork.copied_count).toBe(1);

  // 回退（RPC 直驱内核）：该输入（含）起退出上下文 → 源会话视图为空。
  const rewind = await rpc(page, 'session/rewind', { session_id: sessionId, message_id: editedMessage!.id }) as { cutoff_message_id: string; remaining_count: number };
  expect(rewind.remaining_count).toBe(0);
  await page.reload();
  await expect(page.getByText('开始新的对话')).toBeVisible();
  await expect(page.getByText('hello vivy (edited)')).toHaveCount(0);

  // 分叉子会话：会话列表出现「分叉分支」，进入后能看到复制来的输入。
  // 被回退折出的原文不得在子会话复活（fork 复制的是有效视图）。
  await page.getByRole('button', { name: '历史' }).click();
  await page.getByText('分叉分支').click();
  await expect(page.locator('article').filter({ hasText: 'hello vivy (edited)' })).toHaveCount(1);
  await expect(page.getByText('hello vivy', { exact: true })).toHaveCount(0);
});
