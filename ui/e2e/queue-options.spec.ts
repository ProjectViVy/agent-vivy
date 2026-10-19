import http from 'node:http';
import { expect, test, type Page } from '@playwright/test';

async function rpc(page: Page, method: string, params: Record<string, unknown>) {
  return page.evaluate(async ([method, params]) => {
    const boot = await (await fetch('/rpc/bootstrap')).json();
    const url = new URL(boot.websocket_path, location.origin);
    url.protocol = 'ws:'; url.searchParams.set('token', boot.token);
    const ws = new WebSocket(url);
    return new Promise<Record<string, any>>((resolve, reject) => {
      const timer = setTimeout(() => { ws.close(); reject(new Error('RPC timeout')); }, 15_000);
      ws.onopen = () => ws.send(JSON.stringify({ jsonrpc: '2.0', id: 'init', method: 'initialize', params: { protocol_version: boot.protocol_version } }));
      ws.onmessage = (event) => {
        const response = JSON.parse(String(event.data));
        const session = params.session_id ?? (params.input as Record<string, unknown> | undefined)?.session_id;
        if (response.id === 'init' && session) ws.send(JSON.stringify({ jsonrpc: '2.0', id: 'bind', method: 'session/get', params: { session_id: session } }));
        if ((response.id === 'init' && !session) || response.id === 'bind') ws.send(JSON.stringify({ jsonrpc: '2.0', id: 'call', method, params }));
        if (response.id === 'call') {
          clearTimeout(timer); ws.close();
          if (response.error) reject(new Error(JSON.stringify(response.error))); else resolve(response.result);
        }
      };
      ws.onerror = () => { clearTimeout(timer); reject(new Error('RPC connection failed')); };
    });
  }, [method, params] as const);
}

test('busy image and thinking options survive browser reload and reach the next model call', async ({ page }) => {
  const requests: Record<string, any>[] = [];
  let release: (() => void) | undefined;
  const model = http.createServer((req, res) => {
    let raw = '';
    req.on('data', (chunk) => { raw += chunk; });
    req.on('end', () => {
      const body = JSON.parse(raw);
      if (!body.stream) {
        res.setHeader('Content-Type', 'application/json');
        res.end(JSON.stringify({ id: 'queue-title', object: 'chat.completion', created: 1, model: 'deepseek-flash',
          choices: [{ index: 0, message: { role: 'assistant', content: 'Queue smoke' }, finish_reason: 'stop' }],
          usage: { prompt_tokens: 10, completion_tokens: 2, total_tokens: 12 } }));
        return;
      }
      requests.push(body);
      const n = requests.length;
      const answer = () => {
        if (res.writableEnded || res.destroyed) return;
        res.setHeader('Content-Type', 'text/event-stream');
        const base = { id: `queue-${n}`, object: 'chat.completion.chunk', created: 1, model: 'deepseek-flash' };
        res.write(`data: ${JSON.stringify({ ...base, choices: [{ index: 0, delta: { role: 'assistant', content: `Queue reply ${n}` }, finish_reason: null }] })}\n\n`);
        res.write(`data: ${JSON.stringify({ ...base, choices: [{ index: 0, delta: {}, finish_reason: 'stop' }], usage: { prompt_tokens: 20, completion_tokens: 4, total_tokens: 24 } })}\n\n`);
        res.end('data: [DONE]\n\n');
      };
      if (n === 1) release = answer; else answer();
    });
  });
  await new Promise<void>((resolve) => model.listen(8797, '127.0.0.1', resolve));
  try {
    await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
    const browserErrors: string[] = [];
    page.on('pageerror', (error) => browserErrors.push(error.message));
    await page.goto('/');
    await expect(page.getByText('Start a new conversation')).toBeVisible();
    const previousSession = await page.evaluate(() => localStorage.getItem('vivy.ui.activeSession'));
    await page.getByRole('button', { name: 'New session' }).first().click();
    await expect.poll(() => page.evaluate(() => localStorage.getItem('vivy.ui.activeSession'))).not.toBe(previousSession);
    const input = page.getByRole('combobox', { name: 'Message' });
    await expect(page.getByText('Start a new conversation')).toBeVisible();
    await expect(input).toBeEnabled();
    const session = await page.evaluate(() => localStorage.getItem('vivy.ui.activeSession'));
    expect(session).toBeTruthy();
    await rpc(page, 'module.action.invoke', { module_id: 'vivy/diva-cognitive',
      action_id: 'diva.cognitive.persona.initialize', input: { session_id: session,
        initialization: { identity: 'Queue smoke persona', relationship: 'test partner',
          redline: 'Synthetic loopback test only', user: 'Queue test preferences', world: 'Queue test world' } } });
    await input.fill('Hold the first call'); await input.press('Enter');
    await expect.poll(() => requests.length, { timeout: 15_000 }).toBe(1);
    await page.getByRole('button', { name: 'Thinking mode', exact: true }).click();
    await page.getByRole('menuitem', { name: 'high', exact: true }).click();
    await page.locator('input[type=file]').setInputFiles({ name: 'queue.png', mimeType: 'image/png',
      buffer: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jvV0AAAAASUVORK5CYII=', 'base64') });
    await input.fill('Durable image follow-up'); await input.press('Shift+Alt+Enter');
    const queue = () => rpc(page, 'queue/state', { session_id: session });
    await expect.poll(async () => (await queue()).follow_up?.length).toBe(1);
    const before = (await queue()).follow_up[0];
    expect(before.thinking).toBe('high'); expect(before.mode).toBe('normal');
    expect(before.attachments).toHaveLength(1);
    await page.reload();
    await expect(page.getByText('Durable image follow-up', { exact: true }).first()).toBeVisible();
    expect((await queue()).follow_up[0]).toEqual(before);
    // Recall after reload must restore the captured image and preference,
    // rather than consuming durable work and returning only its text.
    const restoredInput = page.getByRole('combobox', { name: 'Message' });
    await restoredInput.press('Alt+ArrowUp');
    await expect(restoredInput).toHaveValue('Durable image follow-up');
    await expect.poll(async () => (await queue()).follow_up?.length).toBe(0);
    await restoredInput.press('Shift+Alt+Enter');
    await expect.poll(async () => (await queue()).follow_up?.length).toBe(1);
    const restored = (await queue()).follow_up[0];
    expect(restored.thinking).toBe(before.thinking);
    expect(restored.mode).toBe(before.mode);
    expect(restored.attachments).toEqual(before.attachments);
    release!();
    await expect.poll(() => requests.length, { timeout: 30_000 }).toBe(2);
    await expect(page.getByText('Queue reply 2', { exact: true })).toBeVisible();
    expect(JSON.stringify(requests[1].messages)).toContain('data:image/png;base64,');
    expect(requests[1].thinking).toEqual({ type: 'enabled' });
    await expect.poll(async () => (await queue()).follow_up?.length).toBe(0);
    expect(browserErrors).toEqual([]);
    expect(await page.locator('vite-error-overlay').count()).toBe(0);
  } finally {
    release?.();
    model.closeAllConnections();
    await new Promise<void>((resolve) => model.close(() => resolve()));
  }
});
