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
async function rpc(
  page: Page,
  method: string,
  params: Record<string, unknown>,
): Promise<RpcReply> {
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
            socket.send(
              JSON.stringify({
                jsonrpc: '2.0',
                id: 'call',
                method: m,
                params: p,
              }),
            );
          } else if (data.id === 'call') {
            socket.close();
            resolve(data);
          }
        };
        socket.onerror = () => resolve({ error: { message: 'ws error' } });
        socket.onopen = () =>
          socket.send(
            JSON.stringify({
              jsonrpc: '2.0',
              id: 'init',
              method: 'initialize',
              params: { protocol_version: info.protocol_version },
            }),
          );
      });
    },
    [method, params] as const,
  );
}

/** Opens the app with the welcome gate already dismissed (en default locale). */
async function openApp(page: Page): Promise<void> {
  await page.addInitScript(() =>
    localStorage.setItem('vivy.ui.welcome.completed', '1'),
  );
  await page.goto('/');
}

/** Expands the VIVY nav group (when collapsed) and opens the module page. */
async function openMasksPage(page: Page): Promise<void> {
  const link = page.getByRole('link', { name: 'Masks' });
  if (!(await link.isVisible().catch(() => false))) {
    const nav = page.getByRole('button', { name: 'Open navigation' });
    if (await nav.isVisible().catch(() => false)) await nav.click();
  }
  if (!(await link.isVisible().catch(() => false))) {
    await page.getByRole('button', { name: 'VIVY' }).click();
  }
  await link.click();
  await expect(page.getByTestId('mask-page')).toBeVisible();
}

/** Creates a session through the UI so the app peer owns it. */
async function newSession(page: Page): Promise<void> {
  await page.getByRole('button', { name: 'New session' }).first().click();
  await expect(page.getByRole('combobox', { name: 'Message' })).toBeVisible();
}

function selector(page: Page) {
  return page.locator('[data-chat-toolbar] [data-mask-header] button').first();
}
async function chooseMask(page: Page, name: string): Promise<void> {
  await selector(page).click();
  const choice = page
    .getByRole('menuitem')
    .filter({ hasText: name })
    .or(page.getByRole('dialog').getByRole('button').filter({ hasText: name }))
    .first();
  await choice.click();
  await expect(selector(page)).toContainText(name);
}
function editor(page: Page) {
  return page
    .getByRole('dialog')
    .filter({ has: page.locator('[data-mask-editor]') });
}
async function openCustomEditor(page: Page, name: string) {
  await page
    .locator('[data-mask-id]')
    .filter({ hasText: name })
    .locator('button')
    .first()
    .click();
  await page.getByRole('button', { name: 'Edit mask', exact: true }).click();
  await expect(editor(page)).toBeVisible();
}

async function configureHangingProvider(page: Page): Promise<void> {
  const upsert = await rpc(page, 'settings/providers/upsert', {
    display_name: 'Hang Mock',
    bundle: 'deepseek',
    base_url: mockProviderBaseURL,
    default_model: 'deepseek-flash',
    models: ['deepseek-flash'],
    api_key: 'e2e-placeholder',
  });
  expect(upsert.error).toBeUndefined();
  // The narrow picker accepts the existing vendor alias and validates its
  // compiled adapter, preserving vendor ownership for a local test address.
  const update = await rpc(page, 'settings/model/select', {
    provider: 'deepseek',
    base_url: mockProviderBaseURL,
    model: 'deepseek-flash',
  });
  expect(update.error).toBeUndefined();
}

test('default identity, role cards and preview remain distinct from session selection', async ({
  page,
}) => {
  await openApp(page);
  await expect(selector(page)).toContainText('Just me');
  await newSession(page);
  await openMasksPage(page);
  await expect(page.locator('[data-mask-id=""]')).toContainText('Just me');
  for (const id of ['programmer', 'researcher', 'writer'])
    await expect(page.locator(`[data-mask-id="builtin/${id}"]`)).toBeVisible();
  await page
    .locator('[data-mask-id="builtin/programmer"] button')
    .first()
    .click();
  await expect(page.locator('[data-mask-instructions]')).not.toBeEmpty();
  await expect(selector(page)).toContainText('Just me');
  await expect(page.locator('textarea')).toHaveCount(0);
  await page.screenshot({
    path: test.info().outputPath('library-desktop.png'),
    fullPage: true,
  });
});

test('page and toolbar switch immediately and persist across reload', async ({
  page,
}) => {
  await openApp(page);
  await newSession(page);
  await openMasksPage(page);
  await page.locator('[data-mask-id="builtin/writer"] [data-mask-use]').click();
  await expect(selector(page)).toContainText('Writer');
  await chooseMask(page, 'Researcher');
  await expect(
    page.locator('[data-active-mask="builtin/researcher"]'),
  ).toContainText('Researcher');
  await page.reload();
  await expect(selector(page)).toContainText('Researcher');
  await chooseMask(page, 'Just me');
  await expect(page.locator('[data-active-mask=""]')).toContainText('Just me');
});

test('create, edit, revision conflict and delete lifecycle', async ({
  page,
  browser,
}) => {
  await openApp(page);
  await newSession(page);
  await openMasksPage(page);
  await page.locator('[data-mask-action="new"]').click();
  await editor(page).getByLabel('Name', { exact: true }).fill('E2E Mask');
  await editor(page)
    .getByLabel('Instructions', { exact: true })
    .fill('Original instructions.');
  await editor(page).getByRole('button', { name: 'Save', exact: true }).click();
  await expect(editor(page)).toBeHidden();
  await expect(selector(page)).toContainText('Just me');

  const second = await browser.newContext();
  const page2 = await second.newPage();
  await openApp(page2);
  await newSession(page2);
  await openMasksPage(page2);
  await openCustomEditor(page2, 'E2E Mask');
  await openCustomEditor(page, 'E2E Mask');
  await editor(page)
    .getByLabel('Instructions', { exact: true })
    .fill('First committed edit.');
  await editor(page).getByRole('button', { name: 'Save', exact: true }).click();
  await expect(editor(page)).toBeHidden();
  await editor(page2)
    .getByLabel('Instructions', { exact: true })
    .fill('Local stale draft.');
  await editor(page2)
    .getByRole('button', { name: 'Save', exact: true })
    .click();
  await expect(editor(page2).getByRole('alert')).toContainText(
    'changed elsewhere',
  );
  await expect(
    editor(page2).getByLabel('Instructions', { exact: true }),
  ).toHaveValue('Local stale draft.');
  await editor(page2)
    .getByRole('button', { name: 'Load latest version' })
    .click();
  await page2
    .getByRole('button', { name: 'Discard changes', exact: true })
    .click();
  await expect(
    editor(page2).getByLabel('Instructions', { exact: true }),
  ).toHaveValue('First committed edit.');
  await editor(page2)
    .getByRole('button', { name: 'Cancel', exact: true })
    .click();
  await page2
    .locator('[data-mask-id]')
    .filter({ hasText: 'E2E Mask' })
    .locator('[data-mask-use]')
    .click();

  await page.getByRole('button', { name: 'Delete', exact: true }).click();
  const confirm = page.getByRole('dialog');
  await confirm.getByRole('button', { name: 'Delete', exact: true }).click();
  await expect(confirm.getByRole('alert')).toContainText('session');
  await confirm.getByRole('button', { name: 'Cancel', exact: true }).click();
  await chooseMask(page2, 'Just me');
  await second.close();
  await page.getByRole('button', { name: 'Delete', exact: true }).click();
  await page
    .getByRole('dialog')
    .getByRole('button', { name: 'Delete', exact: true })
    .click();
  await expect(
    page.locator('[data-mask-id]').filter({ hasText: 'E2E Mask' }),
  ).toHaveCount(0);
});

test('builtin duplicate opens editor, dirty cancel is guarded and Save and use synchronizes identity', async ({
  page,
}) => {
  await openApp(page);
  await newSession(page);
  await openMasksPage(page);
  await page
    .locator('[data-mask-id="builtin/programmer"] button')
    .first()
    .click();
  await page.getByRole('button', { name: 'Duplicate and edit' }).click();
  await expect(editor(page).getByLabel('Name', { exact: true })).toHaveValue(
    'Programmer copy',
  );
  await editor(page)
    .getByRole('button', { name: 'Cancel', exact: true })
    .click();
  await expect(
    page.getByRole('dialog', { name: 'Discard unsaved changes?' }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Keep editing' }).click();
  await expect(editor(page)).toBeVisible();
  await page.screenshot({ path: test.info().outputPath('editor-desktop.png') });
  await editor(page)
    .getByRole('button', { name: 'Save and use', exact: true })
    .click();
  await expect(editor(page)).toBeHidden();
  await expect(selector(page)).toContainText('Programmer copy');
});

test('code/life is a small placeholder independent from the actual mask', async ({
  page,
}) => {
  await openApp(page);
  await newSession(page);
  await chooseMask(page, 'Programmer');
  const mode = page.locator('[data-conversation-mode-placeholder]');
  await expect(mode).toHaveText('Code');
  const modeBounds = await mode.boundingBox();
  const maskBounds = await selector(page).boundingBox();
  expect(modeBounds!.x + modeBounds!.width).toBeLessThanOrEqual(maskBounds!.x);
  await mode.click();
  await expect(mode).toHaveText('Life');
  await expect(selector(page)).toContainText('Programmer');
  await mode.click();
  await expect(mode).toHaveText('Code');
  await expect(selector(page)).toContainText('Programmer');
});

test('switching during an active reply updates the session identity immediately', async ({
  page,
}) => {
  await openApp(page);
  await configureHangingProvider(page);
  await page.reload();
  await newSession(page);
  await chooseMask(page, 'Programmer');
  await page
    .getByRole('combobox', { name: 'Message' })
    .fill('keep the run active');
  await page.getByRole('button', { name: 'Send' }).click();
  await expect(selector(page)).toHaveAttribute(
    'title',
    /This reply keeps its original role/,
  );
  await chooseMask(page, 'Writer');
  await expect(selector(page)).toContainText('Writer');
  await openMasksPage(page);
  await expect(
    page.locator('[data-active-mask="builtin/writer"]'),
  ).toContainText('Writer');
  await expect(
    page.locator('[data-active-mask="builtin/writer"]'),
  ).toContainText('This reply keeps its original role');
});

for (const width of [1440, 1024, 390, 320]) {
  test(`responsive identity, library and editor stay in bounds at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    await openApp(page);
    if (width < 768)
      await page.getByRole('button', { name: 'Open navigation' }).click();
    await newSession(page);
    await openMasksPage(page);
    await expect(selector(page)).toContainText('Just me');
    await expect(page.locator('[data-mask-id=""]')).toBeVisible();
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBeLessThanOrEqual(width);
    const headerSize = await page
      .locator('[data-chat-toolbar]')
      .evaluate((node) => ({
        scroll: node.scrollWidth,
        client: node.clientWidth,
      }));
    expect(headerSize.scroll - headerSize.client).toBeLessThanOrEqual(1);
    await page.locator('[data-mask-action="new"]').click();
    await editor(page)
      .getByLabel('Name', { exact: true })
      .fill('Responsive custom mask');
    await editor(page)
      .getByLabel('Instructions', { exact: true })
      .fill('Use the available width.');
    for (const name of ['Cancel', 'Save', 'Save and use']) {
      await expect
        .poll(async () => {
          const bounds = await editor(page)
            .getByRole('button', { name, exact: true })
            .boundingBox();
          return Boolean(
            bounds &&
              bounds.x >= 0 &&
              bounds.x + bounds.width <= width &&
              bounds.y + bounds.height <= 900,
          );
        })
        .toBe(true);
    }
    await page.screenshot({
      path: test.info().outputPath(`editor-${width}.png`),
    });
    await editor(page)
      .getByRole('button', { name: 'Cancel', exact: true })
      .click();
    await page
      .getByRole('button', { name: 'Discard changes', exact: true })
      .click();
    if (width < 768) {
      await chooseMask(page, 'Programmer');
      await page
        .locator('[data-mask-id="builtin/programmer"] button')
        .first()
        .click();
      await expect(page.getByRole('dialog')).toContainText('Programmer');
      await expect
        .poll(async () => {
          const bounds = await page.getByRole('dialog').boundingBox();
          return Boolean(
            bounds &&
              bounds.height > 300 &&
              bounds.y >= 0 &&
              bounds.y + bounds.height <= 900,
          );
        })
        .toBe(true);
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth),
      ).toBeLessThanOrEqual(width);
      await page.screenshot({
        path: test.info().outputPath(`details-${width}.png`),
      });
    }
  });
}

test('Chinese default identity and editor remain readable at 320px', async ({
  page,
}) => {
  await page.setViewportSize({ width: 320, height: 900 });
  await openApp(page);
  const locale = await rpc(page, 'settings/locale', { locale: 'zh' });
  expect(locale.error).toBeUndefined();
  await page.reload();
  await page.getByRole('button', { name: '打开导航' }).click();
  await page
    .getByRole('button', { name: '新建会话', exact: true })
    .first()
    .click();
  await expect(selector(page)).toContainText('我就是我');
  const name = selector(page).locator('span').last();
  expect(
    await name.evaluate((node) => node.scrollWidth - node.clientWidth),
  ).toBeLessThanOrEqual(1);
  await page.getByRole('button', { name: '打开导航' }).click();
  const masks = page.getByRole('link', { name: '面具', exact: true });
  if (!(await masks.isVisible()))
    await page.getByRole('button', { name: 'VIVY', exact: true }).click();
  await masks.click();
  await expect(page.locator('[data-mask-id=""]')).toContainText('我就是我');
  await page.locator('[data-mask-action="new"]').click();
  await expect(editor(page).getByLabel('名称', { exact: true })).toBeVisible();
  await expect
    .poll(async () => {
      const bounds = await editor(page)
        .getByRole('button', { name: '保存并使用', exact: true })
        .boundingBox();
      return Boolean(
        bounds &&
          bounds.y + bounds.height <= 900 &&
          bounds.x + bounds.width <= 320,
      );
    })
    .toBe(true);
  await page.screenshot({ path: test.info().outputPath('editor-zh-320.png') });
  const reset = await rpc(page, 'settings/locale', { locale: 'en' });
  expect(reset.error).toBeUndefined();
});
