import { expect, test, type Page } from '@playwright/test';
import type http from 'node:http';
import path from 'node:path';
import { finalText, modelRequestBodies, setModelScript, startLoopbackModel, toolCall } from './continuity-setup';

let model: http.Server;
test.beforeAll(async () => { model = await startLoopbackModel(); });
test.afterAll(() => { model.close(); });
test.describe.configure({ mode: 'serial' });

async function rpc(page: Page, method: string, params: Record<string, unknown>) {
  return page.evaluate(async ([method, params]) => {
    const boot = await (await fetch('/rpc/bootstrap')).json();
    const url = new URL(boot.websocket_path, location.origin);
    url.protocol = 'ws:';
    url.searchParams.set('token', boot.token);
    const ws = new WebSocket(url);
    return new Promise<{ result?: Record<string, unknown>; error?: unknown }>((resolve, reject) => {
      const timer = setTimeout(() => { ws.close(); reject(new Error('RPC timed out')); }, 15000);
      ws.onopen = () => ws.send(JSON.stringify({ jsonrpc: '2.0', id: 'init', method: 'initialize', params: { protocol_version: boot.protocol_version } }));
      ws.onmessage = (event) => {
        const response = JSON.parse(String(event.data));
        if (response.id === 'init') ws.send(JSON.stringify({ jsonrpc: '2.0', id: 'call', method, params }));
        if (response.id === 'call') { clearTimeout(timer); ws.close(); resolve(response); }
      };
      ws.onerror = () => { clearTimeout(timer); reject(new Error('RPC connection failed')); };
    });
  }, [method, params] as const);
}

async function openSession(page: Page) {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await page.goto('/');
  await page.getByRole('button', { name: 'New session' }).first().click();
  await expect(page.getByRole('combobox', { name: 'Message' })).toBeVisible();
  await expect(page.getByText('Start a new conversation')).toBeVisible();
  await expect(page.locator('vite-error-overlay')).toHaveCount(0);
  expect(errors).toEqual([]);
  const id = await page.evaluate(() => localStorage.getItem('vivy.ui.activeSession'));
  if (!id) throw new Error('No active session');
  return id;
}
async function work(page: Page, sessionId: string) {
  const response = await rpc(page, 'session/work/get', { session_id: sessionId });
  expect(response.error).toBeUndefined();
  return response.result as { goal?: { objective: string; max_rounds: number; rounds_started: number }; plan: { active: boolean; review_status: string; submission_id?: string }; activation: string };
}
const screenshot = (name: string) => path.resolve('..', 'docs', 'logs', '2026-10-05-composer-commands', `${name}.png`);

test('empty session, slash discovery and persistent authoritative Plan chip', async ({ page }) => {
  const id = await openSession(page);
  await expect(page.locator('[data-work-dock]')).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Enter Plan', exact: true })).toHaveCount(0);
  const input = page.getByRole('combobox', { name: 'Message' });
  await input.fill('/');
  await expect(page.getByRole('listbox', { name: 'Commands' })).toBeVisible();
  await page.screenshot({ path: screenshot('desktop-command-menu') });
  await input.press('Enter');
  await expect(page.locator('[data-command-tag]')).toContainText('/plan');
  expect((await work(page, id)).plan.active).toBe(false);
  await input.press('Enter');
  await expect(page.locator('[data-plan-mode]')).toContainText('Plan mode');
  expect((await work(page, id)).plan.active).toBe(true);
  await expect(page.locator('[data-work-dock]')).toHaveCount(0);
  await page.reload();
  await expect(page.locator('[data-plan-mode]')).toBeVisible();
  await page.getByRole('button', { name: 'Leave Plan', exact: true }).click();
  await expect(page.locator('[data-plan-mode]')).toHaveCount(0);
  expect((await work(page, id)).plan.active).toBe(false);
});

test('Goal command creates bounded real work above the composer and clear removes the dock', async ({ page }) => {
  const id = await openSession(page);
  await setModelScript(Array.from({ length: 12 }, () => finalText('Goal round complete')));
  const input = page.getByRole('combobox', { name: 'Message' });
  await input.fill('/goal Ship the composer change');
  await expect(page.locator('[data-command-tag]')).toContainText('/goal');
  await input.press('Enter');
  const dock = page.locator('[data-work-dock]');
  await expect(dock).toContainText('Ship the composer change');
  const state = await work(page, id);
  expect(state.goal?.objective).toBe('Ship the composer change');
  expect(state.goal?.max_rounds).toBe(3);
  const a = await dock.boundingBox();
  const b = await input.boundingBox();
  expect(a && b && a.y + a.height <= b.y).toBeTruthy();
  await expect(page.getByRole('button', { name: 'Resume Goal', exact: true })).toHaveCount(0);
  await expect(dock).toContainText('Blocked');
  await page.screenshot({ path: screenshot('desktop-goal-dock') });
  await page.getByRole('button', { name: 'Clear Goal', exact: true }).click();
  await expect(dock).toHaveCount(0);
  expect((await work(page, id)).goal).toBeUndefined();
});

test('submitted Plan stays above input and /goal reviews the exact submission', async ({ page }) => {
  const id = await openSession(page);
  await setModelScript([toolCall('plan-1', 'submit_plan', { markdown: '# Composer plan\n1. Move work into the input dock.\n2. Use command tags.' }), ...Array.from({ length: 12 }, () => finalText('Plan accepted and Goal round complete'))]);
  const input = page.getByRole('combobox', { name: 'Message' });
  await input.fill('/plan Design the composer interaction');
  await input.press('Enter');
  await expect(page.locator('[data-work-dock]')).toContainText('# Composer plan');
  await expect(page.getByRole('button', { name: 'Execute plan once', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Start Goal', exact: true })).toHaveCount(0);
  const pending = await work(page, id);
  expect(pending.plan.review_status).toBe('pending');
  expect(pending.plan.submission_id).toBeTruthy();
  await page.screenshot({ path: screenshot('desktop-plan-review') });
  await input.fill('/goal Deliver the approved composer plan');
  await input.press('Enter');
  await expect(page.locator('[data-work-dock]')).toContainText('Deliver the approved composer plan');
  const accepted = await work(page, id);
  expect(accepted.goal?.objective).toBe('Deliver the approved composer plan');
  expect(accepted.plan.review_status).toBe('accepted');
  expect(accepted.plan.submission_id).toBe(pending.plan.submission_id);
  await expect(page.locator('[data-work-dock]')).toContainText('Blocked');
  await page.getByRole('button', { name: 'Clear Goal', exact: true }).click();
});

test('enabled skill tag uses the existing native skill tool and mobile commands fit the viewport', async ({ page }) => {
  await openSession(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await setModelScript([toolCall('skill-1', 'skill', { skill: 'composer-review' }), finalText('Composer review complete')]);
  const input = page.getByRole('combobox', { name: 'Message' });
  await input.fill('/skill');
  await input.press('Enter');
  await page.getByRole('option').filter({ hasText: 'composer-review' }).click();
  await expect(page.locator('[data-command-tag]')).toContainText('composer-review');
  await input.fill('Review the composer changes');
  await page.screenshot({ path: screenshot('mobile-skill-tag') });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await input.press('Enter');
  await expect(page.getByText('Composer review complete', { exact: true })).toBeVisible();
  const requests = (await modelRequestBodies()).map((body) => JSON.parse(body));
  expect(requests.some((request) => request.messages.some((message: { role: string; content?: string }) => message.role === 'user' && message.content?.includes('Use the skill "composer-review" for this request.')))).toBe(true);
  expect(requests.some((request) => request.messages.some((message: { role: string; content?: string }) => message.role === 'tool' && message.content?.includes('Check the composer request and report the result.')))).toBe(true);
});
