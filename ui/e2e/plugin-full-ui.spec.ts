import { expect, test, type Page } from '@playwright/test';

/**
 * This is an opt-in split-pair test. The normal Playwright webServer is the
 * default (unselected) Generation on 8799; VIVY_FULL_UI_URL points at a
 * separately started Vite 3015 process whose backend is the packed fixture.
 */
const selectedUIURL = process.env.VIVY_FULL_UI_URL?.trim();
const fixtureModule = 'fixture/full-ui';
const fixtureAction = 'fixture.full-ui.echo';
const fixtureStyle = 'fixture-full-ui-style';

type RPCReply = {
  readonly result?: Record<string, unknown>;
  readonly error?: { readonly code?: number; readonly message?: string };
};

async function rpc(page: Page, method: string, params: Record<string, unknown>): Promise<RPCReply> {
  return page.evaluate(async ({ method, params }) => {
    const bootstrap = await (await fetch('/rpc/bootstrap', { cache: 'no-store' })).json() as {
      readonly protocol_version: string;
      readonly websocket_path: string;
      readonly token: string;
    };
    const socketURL = new URL(bootstrap.websocket_path, location.origin);
    socketURL.searchParams.set('token', bootstrap.token);
    const socket = await new Promise<WebSocket>((resolve, reject) => {
      const candidate = new WebSocket(socketURL.toString());
      candidate.onopen = () => resolve(candidate);
      candidate.onerror = () => reject(new Error('websocket connect failed'));
    });
    try {
      let nextID = 0;
      const call = <T extends RPCReply>(callMethod: string, callParams: Record<string, unknown>): Promise<T> => new Promise<T>((resolve, reject) => {
        const id = `full-ui-${Date.now()}-${nextID++}`;
        const timeout = window.setTimeout(() => reject(new Error(`rpc ${method} timeout`)), 5_000);
        socket.onmessage = (event) => {
          const reply = JSON.parse(String(event.data)) as RPCReply & { readonly id?: string };
          if (reply.id !== id) return;
          window.clearTimeout(timeout);
          resolve(reply as T);
        };
        socket.send(JSON.stringify({ jsonrpc: '2.0', id, method: callMethod, params: callParams }));
      });
      await call('initialize', { protocol_version: bootstrap.protocol_version });
      return await call<RPCReply>(method, params);
    } finally {
      socket.close();
    }
  }, { method, params });
}

async function assertDefaultUIOmitsFixture(page: Page, baseURL: string | undefined): Promise<void> {
  await page.goto(new URL('/', baseURL ?? 'http://127.0.0.1:8799').toString());
  await expect(page.getByTestId('full-ui-root')).toHaveCount(0);
  await expect(page.locator(`style[data-vivy-ui-style="${fixtureStyle}"]`)).toHaveCount(0);
  await expect(page.getByRole('link', { name: 'Full UI fixture' })).toHaveCount(0);
}

test('default generated UI omits the full UI fixture', async ({ page, baseURL }) => {
  // RED/baseline: this always runs against the ordinary generated Assembly,
  // which has no fixture root, route, style, or navigation contribution.
  await assertDefaultUIOmitsFixture(page, baseURL);
});

test('selected full UI replacement composes live state and one governed action RPC', async ({ page, baseURL }) => {
  test.skip(!selectedUIURL, 'set VIVY_FULL_UI_URL to the selected fixture Vite server to run this split-pair test');

  let nativeDialogs = 0;
  page.on('dialog', (dialog) => {
    nativeDialogs += 1;
    void dialog.dismiss();
  });
  await page.addInitScript(() => {
    localStorage.setItem('vivy.ui.welcome.completed', '1');
  });

  await assertDefaultUIOmitsFixture(page, baseURL);

  // The selected Vite process is still the real application; only its
  // generated Assembly and backend Generation differ from the baseline.
  await page.goto(new URL('/', selectedUIURL!).toString());
  await expect(page.getByTestId('full-ui-root')).toBeVisible();
  await expect(page.getByTestId('full-ui-copy')).toHaveText('Selected full UI module is active');
  await expect(page.getByTestId('full-ui-connection')).toHaveText('connected');
  await expect(page.getByTestId('full-ui-active-session')).not.toHaveText('none');
  await expect(page.getByTestId('full-ui-session-count')).toHaveText(/^[1-9][0-9]*$/);
  await expect(page.locator(`style[data-vivy-ui-style="${fixtureStyle}"]`)).toHaveCount(1);
  await expect(page.locator('html')).toHaveAttribute('data-vivy-full-ui-theme', 'active');
  await expect(page.locator('html')).toHaveClass(/fixture-full-ui-theme/);
  await expect(page.locator('body')).toHaveCSS('background-color', 'rgb(248, 245, 255)');

  // The root component calls the SDK action client, which can only emit the
  // fixed module.action.invoke method. The response came through the packed
  // server's authenticated WebSocket rather than a browser-local mock.
  await page.getByRole('button', { name: 'Invoke fixture action' }).click();
  await expect(page.getByTestId('full-ui-action-result')).toHaveText('accepted: from selected UI');
  await expect(page.getByTestId('full-ui-action-method')).toHaveText('module.action.invoke');

  await page.getByRole('link', { name: 'Full UI fixture' }).click();
  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(page.getByTestId('full-ui-route')).toBeVisible();
  await expect(page.getByTestId('full-ui-route')).toContainText('Selected full UI route');

  // Browser JSON cannot forge approval, Trust, or other authority claims:
  // strict server decoding rejects the extra fields before ActionHost.Invoke.
  const forged = await rpc(page, 'module.action.invoke', {
    module_id: fixtureModule,
    action_id: fixtureAction,
    input: { message: 'forged' },
    approval: true,
    trust: 'T1',
  });
  expect(forged.error?.code).toBe(-32602);
  expect(forged.error?.message).toMatch(/params are invalid/i);

  const inspect = await rpc(page, 'species/inspect', {});
  expect(inspect.error).toBeUndefined();
  const grants = Array.isArray(inspect.result?.grants) ? inspect.result?.grants as unknown[] : [];
  expect(grants).not.toContain('ui.full');
  expect(grants).not.toContain('ui.grant');
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.getByText(/UI Grant|permission request/i)).toHaveCount(0);
  expect(nativeDialogs).toBe(0);

  // A generation transition/reload tears down the selected DOM composition;
  // the unselected app has no leaked style/theme/route state.
  await page.goto(new URL('/', baseURL ?? 'http://127.0.0.1:8799').toString());
  await expect(page.getByTestId('full-ui-root')).toHaveCount(0);
  await expect(page.locator(`style[data-vivy-ui-style="${fixtureStyle}"]`)).toHaveCount(0);
  await expect(page.locator('html')).not.toHaveAttribute('data-vivy-full-ui-theme', 'active');
  await expect(page.getByRole('link', { name: 'Full UI fixture' })).toHaveCount(0);
});
