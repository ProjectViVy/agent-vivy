import { expect, test, type Page } from '@playwright/test';
import type http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import {
  finalText,
  modelRequestBodies,
  runWorkspaceDirs,
  setModelScript,
  sha256DownloadedFile,
  sha256Text,
  startLoopbackModel,
  toolCall,
} from './continuity-setup';

// T12 end-to-end coding acceptance: the deterministic loopback model drives
// the real backend + Vite split pair; every turn commits through the actual
// Journal and every download verifies real bytes (never a mocked callback).
const SENTINEL = 'SENTINEL-UNRELATED-SESSION';
const REPORT_BYTES = 'REPORT-BYTES\n';
let deliverySessionId = '';

type Json = Record<string, unknown>;

let modelServer: http.Server;
test.beforeAll(async () => {
  modelServer = await startLoopbackModel();
});
test.afterAll(() => {
  modelServer.close();
});
test.describe.configure({ mode: 'serial' });

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
        setTimeout(() => reject(new Error(`rpc ${method} timeout`)), 10_000);
      });
    } finally {
      socket.close();
    }
  }, { method, params });
}

async function newChat(page: Page): Promise<string> {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/');
  await expect(page.locator('article').first().or(page.getByText('Start a new conversation'))).toBeVisible({ timeout: 15_000 });
  await page.getByRole('button', { name: 'New session' }).first().click();
  await expect(page.getByText('Start a new conversation')).toBeVisible();
  const sessionId = await page.evaluate(() => localStorage.getItem('vivy.ui.activeSession'));
  if (!sessionId) throw new Error('no active session after new chat');
  return sessionId;
}

async function send(page: Page, text: string): Promise<void> {
  const composer = page.getByPlaceholder('Type a message... (Enter to send)');
  await composer.fill(text);
  await composer.press('Enter');
}

test('coding delivery loop: card, verify, preview and byte-exact download', async ({ page }) => {
  await setModelScript([
    toolCall('call-1', 'write_file', { path: 'report.txt', content: REPORT_BYTES }),
    toolCall('call-2', 'bash', { command: 'tar -cf report.tar report.txt' }),
    toolCall('call-3', 'present_files', {
      files: [
        { path: 'report.txt', description: 'final report' },
        { path: 'report.tar', description: 'report archive' },
      ],
      title: 'Report bundle',
    }),
    finalText('loop complete'),
  ]);
  deliverySessionId = await newChat(page);
  await send(page, 'make the report');

  const card = page.locator('section[data-delivery-set]');
  await expect(card).toBeVisible({ timeout: 45_000 });
  await expect(page.getByText('2 files delivered')).toBeVisible();
  // Binary entries never offer an in-app preview.
  await expect(page.getByRole('button', { name: 'Preview report.tar' })).toHaveCount(0);

  await page.getByRole('button', { name: 'Verify report.txt' }).click();
  await expect(page.getByText('verified').first()).toBeVisible({ timeout: 15_000 });

  await page.getByRole('button', { name: 'Preview report.txt' }).click();
  await expect(page.locator('[data-delivery-preview]')).toContainText('REPORT-BYTES');

  const downloadReady = page.waitForEvent('download');
  await page.getByRole('button', { name: 'Download report.txt' }).click();
  const download = await downloadReady;
  expect(await sha256DownloadedFile(download)).toBe(sha256Text(REPORT_BYTES));

  const bodies = await modelRequestBodies();
  expect(bodies.join('\n')).not.toContain(SENTINEL);
});

test('partial group, second same-path group and changed-file state', async ({ page }) => {
  // Tamper with the committed workspace bytes, then re-verify through the
  // real read path — the card must explain the file changed since delivery.
  const runDir = runWorkspaceDirs().find((dir) => fs.existsSync(path.join(dir, 'report.txt')));
  if (!runDir) throw new Error('run workspace with report.txt not found');
  fs.appendFileSync(path.join(runDir, 'report.txt'), 'tampered');

  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/');
  await expect(page.locator('section[data-delivery-set]')).toBeVisible({ timeout: 15_000 });
  await page.getByRole('button', { name: 'Verify report.txt' }).click();
  await expect(page.getByText('file changed since delivery').first()).toBeVisible({ timeout: 15_000 });

  // Each run gets a private workspace: write the file again in this run so
  // the group comes out partial (one committed item + one missing path).
  await setModelScript([
    toolCall('call-4', 'write_file', { path: 'report.txt', content: 'SECOND-BYTES\n' }),
    toolCall('call-5', 'present_files', {
      files: [
        { path: 'report.txt', description: 'updated report' },
        { path: 'ghost.txt', description: 'missing file' },
      ],
      title: 'Second bundle',
    }),
    finalText('second delivery done'),
  ]);
  await send(page, 'deliver again');
  const cards = page.locator('section[data-delivery-set]');
  await expect(cards).toHaveCount(2, { timeout: 45_000 });
  await expect(page.getByText('1 failed')).toBeVisible();
  await expect(page.getByText('partial').first()).toBeVisible();
  await expect(page.getByText('ghost.txt')).toBeVisible();

  // Reconnect/restart replay: the immutable groups reload from the journal
  // exactly once — no duplicate cards from the replay/live union.
  await page.reload();
  await expect(cards).toHaveCount(2, { timeout: 15_000 });
  await expect(page.getByText('2 files delivered')).toBeVisible();
});

test('history reference: attach via picker, source deletion keeps saved excerpt', async ({ page }) => {
  // Source session with a committed rationale message.
  const srcId = await newChat(page);
  await setModelScript([finalText('PLAN-ALPHA rationale: refactor the parser then verify.')]);
  await send(page, 'explain the plan');
  await expect(page.locator('article').filter({ hasText: 'PLAN-ALPHA rationale' })).toBeVisible({ timeout: 30_000 });
  await rpc(page, 'session/rename', { session_id: srcId, title: 'SRC-SESSION' });

  // Destination session attaches one source record through the picker.
  const dstId = await newChat(page);
  await page.getByRole('button', { name: 'Attach history' }).click();
  const picker = page.getByRole('dialog');
  await picker.getByText('SRC-SESSION', { exact: true }).click();
  const record = picker.getByRole('checkbox', { name: 'Select assistant message' });
  await expect(record).toBeVisible({ timeout: 15_000 });
  await record.click();
  await picker.getByRole('button', { name: 'Preview', exact: true }).click();
  await expect(picker.getByText(/records · .* · ok/)).toBeVisible({ timeout: 15_000 });
  await picker.getByRole('button', { name: 'Attach', exact: true }).click();

  await setModelScript([finalText('ack')]);
  await send(page, 'use the plan');
  const refCard = page.locator('[data-reference-card]');
  await expect(refCard).toBeVisible({ timeout: 30_000 });
  await refCard.getByRole('button').first().click();
  await expect(page.getByText('Saved excerpt')).toBeVisible();
  await expect(page.getByText('PLAN-ALPHA rationale').first()).toBeVisible();
  await expect(page.locator('[data-source-jump]')).toBeEnabled();

  // Replay produces no duplicate reference card.
  await page.reload();
  await expect(page.locator('[data-reference-card]')).toHaveCount(1, { timeout: 15_000 });

  // Delete the source session: the saved excerpt stays readable while the
  // live source jump is disabled.
  await rpc(page, 'session/delete', { session_id: srcId });
  await page.evaluate((id) => localStorage.setItem('vivy.ui.activeSession', id), dstId);
  await page.reload();
  const refCardAfter = page.locator('[data-reference-card]');
  await expect(refCardAfter).toBeVisible({ timeout: 15_000 });
  await refCardAfter.getByRole('button').first().click();
  await expect(page.getByText('source_unavailable').first()).toBeVisible({ timeout: 15_000 });
  await expect(page.locator('[data-source-jump]')).toBeDisabled();
  await expect(page.getByText('Saved excerpt')).toBeVisible();
  await expect(page.getByText('PLAN-ALPHA rationale').first()).toBeVisible();
});

test('responsive widths and keyboard delivery actions', async ({ page }, testInfo) => {
  await page.addInitScript((sid) => {
    localStorage.setItem('vivy.ui.welcome.completed', '1');
    localStorage.setItem('vivy.ui.activeSession', sid);
  }, deliverySessionId);
  await page.goto('/');
  const cards = page.locator('section[data-delivery-set]');
  await expect(cards.first()).toBeVisible({ timeout: 15_000 });

  for (const width of [320, 768, 1280]) {
    await page.setViewportSize({ width, height: 800 });
    await expect(cards.first()).toBeVisible();
    const shot = await page.screenshot({ fullPage: false });
    await testInfo.attach(`delivery-${width}px.png`, { body: shot, contentType: 'image/png' });
  }
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow).toBeLessThanOrEqual(1);

  // Keyboard: tab-focus lands on a card action and Enter drives the real
  // download (no pointer required).
  await page.setViewportSize({ width: 1280, height: 800 });
  // The first card's report.txt now reads 'changed' on disk; the second
  // group fingerprinted the tampered bytes at commit, so its download
  // serves real bytes — drive it with the keyboard.
  const downloadButton = page.getByRole('button', { name: 'Download report.txt' }).last();
  await downloadButton.focus();
  const downloadReady = page.waitForEvent('download');
  await page.keyboard.press('Enter');
  const download = await downloadReady;
  expect(await sha256DownloadedFile(download)).toBe(sha256Text('SECOND-BYTES\n'));
});
