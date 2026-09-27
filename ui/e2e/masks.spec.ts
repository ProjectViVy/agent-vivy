import { expect, test, type Page } from '@playwright/test';
import { mockProviderBaseURL } from '../playwright.masks.config';

/**
 * MASK-4 task 3 acceptance: the recipe-selected masks module renders its nav
 * entry, catalog, editor, and chat-header selector against a packed binary
 * (sealed generation) with disposable state.
 *
 * Sessions are created through the app's own UI, never a side-channel RPC:
 * module actions are authorized per WS peer, so only the peer that owns the
 * session may read or write its mask selection.
 */

interface RpcReply {
  result?: Record<string, unknown>;
  error?: { message?: string };
}

/** Session-free control-plane RPC (settings only — mask actions are peer-bound). */
async function rpc(page: Page, method: string, params: Record<string, unknown>): Promise<RpcReply> {
  return page.evaluate(
    async ([m, p]) => {
      const boot = await fetch('/rpc/bootstrap', { cache: 'no-store' });
      if (!boot.ok) return { error: { message: `bootstrap ${boot.status}` } };
      const info = await boot.json();
      const url = new URL(info.websocket_path, location.origin);
      url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
      url.searchParams.set('token', info.token);
      const socket = new WebSocket(url.toString());
      return await new Promise((resolve) => {
        socket.onmessage = (event) => {
          const data = JSON.parse(String(event.data));
          if (data.id === 'init') {
            socket.send(JSON.stringify({ jsonrpc: '2.0', id: 'call', method: m, params: p }));
          } else if (data.id === 'call') {
            socket.close();
            resolve(data);
          }
        };
        socket.onerror = () => resolve({ error: { message: 'ws error' } });
        socket.onopen = () =>
          socket.send(JSON.stringify({ jsonrpc: '2.0', id: 'init', method: 'initialize', params: { protocol_version: info.protocol_version } }));
      });
    },
    [method, params] as const,
  );
}

/** Opens the app with the welcome gate already dismissed (en default locale). */
async function openApp(page: Page): Promise<void> {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/');
}

/** Expands the VIVY nav group (when collapsed) and opens the module page. */
async function openMasksPage(page: Page): Promise<void> {
  const link = page.getByRole('link', { name: 'Masks' });
  if (!(await link.isVisible().catch(() => false))) {
    await page.getByRole('button', { name: 'VIVY' }).click();
  }
  await link.click();
  await expect(page.getByTestId('mask-page')).toBeVisible();
}

/** Creates a session through the UI so the app peer owns it. */
async function newSession(page: Page): Promise<void> {
  await page.getByRole('button', { name: 'New session' }).first().click();
  await expect(page.getByPlaceholder('Type a message... (Enter to send)')).toBeVisible();
}

/** Selects the first (most recent) session row in the sidebar. */
async function selectFirstSession(page: Page): Promise<void> {
  const row = page.locator('div.group.cursor-pointer').first();
  await expect(row).toBeVisible();
  await row.click();
}

async function configureHangingProvider(page: Page): Promise<void> {
  const upsert = await rpc(page, 'settings/providers/upsert', {
    display_name: 'Hang Mock',
    bundle: 'openai-completions',
    base_url: mockProviderBaseURL,
    default_model: 'hang-mock',
    models: ['hang-mock'],
    api_key: 'e2e-placeholder',
  });
  expect(upsert.error).toBeUndefined();
  const settings = await rpc(page, 'settings/get', {});
  const doc = (settings.result ?? {}) as Record<string, unknown>;
  const update = await rpc(page, 'settings/update', {
    ...doc,
    provider: 'openai-completions',
    base_url: mockProviderBaseURL,
    default_model: 'hang-mock',
  });
  expect(update.error).toBeUndefined();
}

test('mask module nav entry, catalog, and builtin definitions render', async ({ page }) => {
  await openApp(page);
  await openMasksPage(page);
  await expect(page.getByRole('button', { name: 'Programmer built-in' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Researcher built-in' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Writer built-in' })).toBeVisible();
});

test('chat header selector applies a session mask and persists across reload', async ({ page }) => {
  await openApp(page);
  await newSession(page);

  const selector = page.getByRole('combobox', { name: 'Session mask' });
  await expect(selector).toBeEnabled();
  await selector.selectOption({ label: 'Programmer' });
  await expect(selector).toHaveValue('builtin/programmer');

  await page.reload();
  await selectFirstSession(page);
  await expect(page.getByRole('combobox', { name: 'Session mask' })).toHaveValue('builtin/programmer');
});

test('custom mask create, stale revision conflict, and in-use delete refusal', async ({ page, browser }) => {
  await openApp(page);
  await newSession(page);

  // Create a custom mask through the catalog editor.
  await openMasksPage(page);
  await page.getByTestId('mask-page').locator('button[data-mask-action="new"]').click();
  const editor = page.locator('section[aria-label="Mask editor"]');
  await editor.locator('input').first().fill('E2E Mask');
  await editor.locator('textarea').fill('You are an e2e mask.');
  await editor.getByRole('button', { name: 'Save' }).click();
  const catalogItem = page.getByRole('button', { name: /E2E Mask/ });
  await expect(catalogItem).toBeVisible();

  // Two browser contexts racing the same definition: the second save must
  // surface the revision-conflict copy instead of silently overwriting.
  const second = await browser.newContext();
  const page2 = await second.newPage();
  await openApp(page2);
  await newSession(page2);
  await openMasksPage(page2);
  await page2.getByRole('button', { name: /E2E Mask/ }).click();
  const editor2 = page2.locator('section[aria-label="Mask editor"]');
  // The second context must hold the pre-write revision before the first
  // context commits, otherwise its "stale" save silently wins.
  await expect(editor2.locator('input').first()).toHaveValue('E2E Mask');

  await catalogItem.click();
  await editor.locator('textarea').fill('first write wins');
  await editor.getByRole('button', { name: 'Save' }).click();
  // Wait until the first write settles so the second one is provably stale.
  await expect(editor.getByRole('button', { name: 'Save' })).toBeEnabled();

  await editor2.locator('textarea').fill('stale write loses');
  await editor2.getByRole('button', { name: 'Save' }).click();
  await expect(page2.getByRole('alert')).toBeVisible();
  await second.close();

  // Bind the mask to the session via the page selector, then prove delete is refused in-use.
  await page.getByTestId('mask-page').getByRole('combobox', { name: 'Session mask' }).selectOption({ label: 'E2E Mask · custom' });
  await catalogItem.click();
  await editor.getByRole('button', { name: 'Delete' }).click();
  await expect(page.getByRole('alert')).toBeVisible();
  await expect(catalogItem).toBeVisible();
});

test('code mode toggles independently of the session mask', async ({ page }) => {
  await openApp(page);
  await newSession(page);

  const codeToggle = page.getByRole('button', { name: 'Enable code mode' });
  test.skip(!(await codeToggle.isVisible()), 'packed generation does not advertise code_mode_available');

  const selector = page.getByRole('combobox', { name: 'Session mask' });
  await selector.selectOption({ label: 'Programmer' });
  await expect(selector).toHaveValue('builtin/programmer');

  await codeToggle.click();
  await expect(page.getByRole('button', { name: 'Code mode on' })).toBeVisible();
  await expect(selector).toHaveValue('builtin/programmer');

  await page.getByRole('button', { name: 'Code mode on' }).click();
  await expect(page.getByRole('button', { name: 'Enable code mode' })).toBeVisible();
  await expect(selector).toHaveValue('builtin/programmer');
});

test('selection during an active run is queued for the next run', async ({ page }) => {
  await openApp(page);
  await configureHangingProvider(page);
  await newSession(page);

  // The mock provider hangs forever, keeping the run active.
  await page.getByPlaceholder('Type a message... (Enter to send)').fill('keep the run active');
  await page.getByRole('button', { name: 'Send' }).click();

  const selector = page.getByRole('combobox', { name: 'Session mask' });
  await expect(page.getByText('Next run choice; the current run already captured its mask')).toBeVisible();
  await selector.selectOption({ label: 'Writer' });
  await expect(selector).toHaveValue('builtin/writer');
});
