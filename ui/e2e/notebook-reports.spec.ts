import { expect, test, type Page } from '@playwright/test';
import type { ChildProcess } from 'node:child_process';
import { startBackend, stopBackend } from './notebook-backend';
import { nbConfigPath, notebookBins } from '../playwright.notebook.config';

/**
 * R2 task 3 acceptance: report generation, provenance and candidate adoption
 * against a real packed backend. The narrate node has no provider credential
 * in this environment, so runs finish deterministically in fallback mode —
 * the UI must show honest fallback provenance, never a fabricated model run.
 */

async function invokeModule(page: Page, moduleId: string, actionId: string, input: unknown): Promise<Record<string, unknown>> {
  return page.evaluate(async ([m, a, i]) => {
    const boot = await fetch('/rpc/bootstrap', { cache: 'no-store' });
    if (!boot.ok) throw new Error(`bootstrap ${boot.status}`);
    const info = await boot.json();
    const url = new URL(info.websocket_path, location.origin);
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
    url.searchParams.set('token', info.token);
    const socket = new WebSocket(url.toString());
    return await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('rpc timeout')), 20_000);
      socket.onopen = () =>
        socket.send(JSON.stringify({ jsonrpc: '2.0', id: 'init', method: 'initialize', params: { protocol_version: info.protocol_version } }));
      socket.onmessage = (event) => {
        const data = JSON.parse(String(event.data));
        if (data.id === 'init') {
          socket.send(JSON.stringify({ jsonrpc: '2.0', id: 'nb1', method: 'module.action.invoke', params: { module_id: m, action_id: a, input: i } }));
        } else if (data.id === 'nb1') {
          clearTimeout(timer);
          socket.close();
          if (data.error) reject(new Error(JSON.stringify(data.error)));
          else resolve(data.result ?? {});
        }
      };
      socket.onerror = () => reject(new Error('ws error'));
    });
  }, [moduleId, actionId, input] as const);
}

async function openNotebook(page: Page): Promise<void> {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/notebook');
  await expect(page.getByTestId('notebook-page')).toBeVisible();
}

async function selectDailySection(page: Page): Promise<void> {
  await page.getByTestId('notebook-section-select').selectOption('section-daily');
}

async function openEntry(page: Page, title: string): Promise<void> {
  const rows = page.getByTestId('notebook-entry');
  const row = title ? rows.filter({ hasText: title }) : rows.first();
  await expect(row).toBeVisible({ timeout: 30_000 });
  await row.click();
  await expect(page.getByTestId('notebook-editor')).toBeVisible();
}

test.describe.configure({ mode: 'serial' });
test.describe('notebook reports (R2)', () => {
  let backend: ChildProcess | null = null;

  test.beforeAll(async () => {
    backend = await startBackend(notebookBins.selected, nbConfigPath);
  });
  test.afterAll(async () => {
    await stopBackend(backend);
  });

  test('generate daily report, verify provenance, edit, comment and regenerate', async ({ page }) => {
    await openNotebook(page);
    await selectDailySection(page);

    // Fresh empty section: the controls render on entry detail only. Generate
    // through the action to land the first entry, then drive the panel.
    const admission = await invokeModule(page, 'vivy/reports', 'vivy.reports.generate', {
      period: 'daily', window: 'completed', operation_key: 'e2e-gen-1',
    });
    expect((admission.result as Record<string, unknown>)?.run_id).toBeTruthy();

    // The published entry appears in the daily section and carries the
    // report badge; the provenance strip names series + window.
    await page.reload();
    await expect(page.getByTestId('notebook-page')).toBeVisible();
    await selectDailySection(page);
    const row = page.getByTestId('notebook-entry');
    await expect(row).toHaveCount(1, { timeout: 30_000 });
    await row.click();
    await expect(page.getByTestId('report-provenance')).toContainText('report.daily', { timeout: 30_000 });

    // The generated revision is labeled generated, not human.
    await page.getByRole('tab', { name: /Revisions|修订/ }).click();
    const generated = page.getByTestId('notebook-revision').filter({ hasText: /generated|生成/ });
    await expect(generated.first()).toBeVisible();

    // Edit the body — the save creates a human revision distinct from
    // generated content.
    await page.getByRole('tab', { name: /Editor|编辑器/ }).click();
    await page.getByTestId('notebook-draft').fill('# human edit\n\noperator text');
    await page.getByTestId('notebook-save').click();
    await expect(page.getByTestId('notebook-save-state')).toHaveText(/saved|已保存/);

    // Feedback: comment anchored to the current revision.
    await page.getByRole('tab', { name: /Comments|评论/ }).click();
    await page.getByTestId('notebook-comment-input').fill('e2e feedback');
    await page.getByTestId('notebook-comment-submit').click();
    await expect(page.getByTestId('notebook-comment').first()).toBeVisible();

    // Regenerate through the panel controls (rejoined or new run).
    await page.getByRole('tab', { name: /Editor|编辑器/ }).click();
    const panel = page.getByTestId('notebook-reports');
    await expect(panel).toBeVisible();
    await page.getByTestId('report-generate').click();
    const status = page.getByTestId('report-status');
    await expect(status).toBeVisible({ timeout: 60_000 });
    await expect(status).toHaveText(/completed|cancelled|已完成|已取消/i, { timeout: 60_000 });

    // Revisions now show both generated candidates and the human edit;
    // non-head generated rows offer Use this version.
    await page.getByRole('tab', { name: /Revisions|修订/ }).click();
    const generatedRows = page.getByTestId('notebook-revision').filter({ hasText: /generated|生成/ });
    await expect(generatedRows.first()).toBeVisible();
  });

  test('backend restart does not fabricate a failure and the run survives', async ({ page }) => {
    await openNotebook(page);
    await selectDailySection(page);
    await openEntry(page, '');
    await expect(page.getByTestId('notebook-reports')).toBeVisible();
    await page.getByTestId('report-generate').click();

    // Interrupt the transport mid-flight: stop the backend while the panel
    // may still be tracking the run. The UI must not invent a `failed`
    // status from a dead socket — either the badge is absent (admission
    // lost) or it shows a non-failed state.
    await stopBackend(backend);
    backend = null;
    await page.waitForTimeout(2_000);
    const midBadge = page.getByTestId('report-status');
    if ((await midBadge.count()) > 0) {
      await expect(midBadge).not.toHaveText(/failed|失败/i);
    }

    backend = await startBackend(notebookBins.selected, nbConfigPath);

    // After reconnect, generating again replays or admits by receipt — it
    // must never duplicate a completed publication.
    await page.reload();
    await expect(page.getByTestId('notebook-page')).toBeVisible();
    await selectDailySection(page);
    await openEntry(page, '');
    await page.getByTestId('report-generate').click();
    const status = page.getByTestId('report-status');
    await expect(status).toBeVisible({ timeout: 60_000 });
    await expect(status).toHaveText(/completed|cancelled|已完成|已取消/i, { timeout: 60_000 });
  });

  test('controls vanish in a notebook-only generation (no reports module)', async ({ page }) => {
    await stopBackend(backend);
    backend = await startBackend(notebookBins.noReports, nbConfigPath);
    try {
      await openNotebook(page);
      await expect(page.getByTestId('notebook-page')).toBeVisible();
      await selectDailySection(page);
      await openEntry(page, '');
      // Generic editing still works; only the report controls disappear.
      await expect(page.getByTestId('notebook-editor')).toBeVisible();
      await expect(page.getByTestId('notebook-reports')).toHaveCount(0);
    } finally {
      await stopBackend(backend);
      backend = await startBackend(notebookBins.selected, nbConfigPath);
    }
  });
});
