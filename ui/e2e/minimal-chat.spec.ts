import fs from 'node:fs';
import path from 'node:path';
import { randomUUID } from 'node:crypto';
import type http from 'node:http';
import { test, expect, type Page } from '@playwright/test';
import { startLoopbackModel, setModelScript, modelRequestBodies, toolCall, finalText } from './continuity-setup';

let model: http.Server;
test.beforeAll(async () => { model = await startLoopbackModel({ titleText: 'Minimal test chat' }); });
test.afterAll(async () => { model.closeAllConnections(); await new Promise<void>((resolve) => model.close(() => resolve())); });

async function expectTerminalAndIdle(page: Page, terminal: string) {
  const runId = await page.getByTestId('minimal-status').getAttribute('data-run-id');
  expect(runId).toBeTruthy();
  const result = await page.evaluate(async (id) => {
    const modulePath = '/src/lib/api.ts';
    const api = await import(/* @vite-ignore */ modulePath);
    const log = await api.getRunLog(id);
    const active = await api.listBackgroundRuns();
    return { terminals: log.events.filter((event: { type: string }) => ['run.completed', 'run.failed', 'run.cancelled'].includes(event.type)).map((event: { type: string }) => event.type), active: active.runs.length };
  }, runId);
  expect(result).toEqual({ terminals: [terminal], active: 0 });
}

test('selected minimal-chat completes protected list/read causality and stays usable on a narrow viewport', async ({ page }) => {
  const workspace = path.join(process.env.VIVY_MINIMAL_E2E_HOME!, 'workspace');
  const nonce = randomUUID();
  fs.writeFileSync(path.join(workspace, 'nonce.txt'), nonce);
  const before = (await modelRequestBodies()).length;
  await setModelScript([toolCall('minimal-list', 'list_dir', { path: '.' }),
    toolCall('minimal-read', 'read_file', { path: 'nonce.txt' }), finalText('Read complete.')]);
  await page.goto('/');
  await expect(page.getByTestId('minimal-status')).toHaveText('Ready');
  await page.getByLabel('Workspace (optional)').fill(workspace);
  await page.getByLabel('Message', { exact: true }).fill('List this workspace, read nonce.txt, then finish.');
  await page.getByTestId('minimal-send').click();
  await expect(page.getByTestId('minimal-status')).toHaveText('Completed', { timeout: 30_000 });
  await expect(page.getByTestId('minimal-tool-minimal-read')).toContainText(nonce);
  await expect(page.getByTestId('minimal-tool-minimal-list')).toContainText('nonce.txt');
  await expect(page.getByTestId('minimal-message').last()).toContainText('Read complete.');
  const requests = (await modelRequestBodies()).slice(before).map((body) => JSON.parse(body)).filter((body) => body.stream === true);
  expect(requests).toHaveLength(3);
  const toolResult = requests[2].messages.find((message: { role: string; tool_call_id?: string }) => message.role === 'tool' && message.tool_call_id === 'minimal-read');
  expect(toolResult.content).toContain(nonce);
  await expectTerminalAndIdle(page, 'run.completed');
  await page.getByTestId('minimal-message').last().scrollIntoViewIfNeeded();
  await page.screenshot({ path: test.info().outputPath('desktop.png') });
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await expect(page.getByTestId('minimal-send')).toBeInViewport();
  await page.screenshot({ path: test.info().outputPath('narrow.png') });
  await page.getByLabel('Message', { exact: true }).fill('Ready for another message');
  await expect(page.getByTestId('minimal-send')).toBeEnabled();
});

test('cancel reaches the active Run owner and displays one terminal', async ({ page }) => {
  await setModelScript([{ kind: 'hang' }]);
  await page.goto('/');
  await page.getByLabel('Message', { exact: true }).fill('Hold this request until cancelled.');
  await page.getByTestId('minimal-send').click();
  await expect(page.getByTestId('minimal-status')).toHaveText('Running');
  await expect(page.getByTestId('minimal-send')).toBeDisabled();
  await page.getByTestId('minimal-cancel').click();
  await expect(page.getByTestId('minimal-status')).toHaveText('Cancelled', { timeout: 20_000 });
  await expect(page.getByTestId('minimal-cancel')).toHaveCount(0);
  await expectTerminalAndIdle(page, 'run.cancelled');
});

test('provider failure is visible and preserves the conversation', async ({ page }) => {
  await setModelScript([{ kind: 'failure' }]);
  await page.goto('/');
  await page.getByLabel('Message', { exact: true }).fill('Exercise the failure path.');
  await page.getByTestId('minimal-send').click();
  await expect(page.getByTestId('minimal-status')).toHaveText('Failed', { timeout: 30_000 });
  await expect(page.getByRole('alert')).not.toBeEmpty();
  await expect(page.getByTestId('minimal-message').filter({ hasText: 'Exercise the failure path.' })).toBeVisible();
  await expectTerminalAndIdle(page, 'run.failed');
});
