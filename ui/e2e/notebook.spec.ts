import { expect, test, type Page } from '@playwright/test';
import fs from 'node:fs';
import type { ChildProcess } from 'node:child_process';
import { startBackend, stopBackend } from './notebook-backend';
import { nbConfigPath, notebookBins } from '../playwright.notebook.config';

/**
 * N3 task 3 acceptance: the durable notebook editor runs against a real packed
 * backend + Vite dev pair — create/save/comment/revision/move/delete/restore/
 * export flows, restart persistence, and a two-tab stale-CAS conflict that
 * must keep the local draft.
 */

const SECTION_NAME = 'E2E Section';
const DOC_TITLE = 'E2E doc';
const DOC_BODY = '# E2E doc\n\nfirst body';

async function invokeModule(page: Page, actionId: string, input: unknown): Promise<Record<string, unknown>> {
  return page.evaluate(async ([a, i]) => {
    const boot = await fetch('/rpc/bootstrap', { cache: 'no-store' });
    if (!boot.ok) throw new Error(`bootstrap ${boot.status}`);
    const info = await boot.json();
    const url = new URL(info.websocket_path, location.origin);
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
    url.searchParams.set('token', info.token);
    const socket = new WebSocket(url.toString());
    return await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('rpc timeout')), 15_000);
      socket.onopen = () =>
        socket.send(JSON.stringify({ jsonrpc: '2.0', id: 'init', method: 'initialize', params: { protocol_version: info.protocol_version } }));
      socket.onmessage = (event) => {
        const data = JSON.parse(String(event.data));
        if (data.id === 'init') {
          socket.send(JSON.stringify({ jsonrpc: '2.0', id: 'nb1', method: 'module.action.invoke', params: { module_id: 'vivy/notebook-core', action_id: a, input: i } }));
        } else if (data.id === 'nb1') {
          clearTimeout(timer);
          socket.close();
          if (data.error) reject(new Error(JSON.stringify(data.error)));
          else resolve(data.result ?? {});
        }
      };
      socket.onerror = () => reject(new Error('ws error'));
    });
  }, [actionId, input] as const);
}

async function openNotebook(page: Page): Promise<void> {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/notebook');
  await expect(page.getByTestId('notebook-page')).toBeVisible();
}

async function selectSection(page: Page, id: string): Promise<void> {
  await page.getByTestId('notebook-section-select').selectOption(id);
}

async function openEntry(page: Page, title: string): Promise<void> {
  const row = page.getByTestId('notebook-entry').filter({ hasText: title });
  await expect(row).toBeVisible();
  await row.click();
  await expect(page.getByTestId('notebook-editor')).toBeVisible();
}

async function saveDraft(page: Page, markdown: string): Promise<void> {
  await page.getByTestId('notebook-draft').fill(markdown);
  await page.getByTestId('notebook-save').click();
  await expect(page.getByTestId('notebook-save-state')).toHaveText(/saved|已保存/);
}

test.describe.configure({ mode: 'serial' });
test.describe('notebook durable editor (N3)', () => {
  let backend: ChildProcess | null = null;

  test.beforeAll(async () => {
    backend = await startBackend(notebookBins.selected, nbConfigPath);
  });
  test.afterAll(async () => {
    await stopBackend(backend);
  });

  test('seeded role sections render; no report-generation controls surface', async ({ page }) => {
    await openNotebook(page);
    const select = page.getByTestId('notebook-section-select');
    await expect(select.locator('option[value="section-notes"]')).toHaveCount(1);
    await expect(select.locator('option[value="section-daily"]')).toHaveCount(1);
    await expect(select.locator('option[value="section-weekly"]')).toHaveCount(1);
    await expect(select.locator('option[value="section-monthly"]')).toHaveCount(1);
    // Reports ship in R1/R2; the editor must not expose generate/run controls.
    await expect(page.getByText(/生成报告|新建报告|Generate report|Run report/i)).toHaveCount(0);
  });

  test('create section + document, save, comment, revisions, move, delete/restore, export', async ({ page }) => {
    await openNotebook(page);

    // Section
    await page.getByTestId('notebook-section-create').click();
    await page.getByTestId('notebook-section-title').fill(SECTION_NAME);
    await page.getByTestId('notebook-section-create-submit').click();
    const select = page.getByTestId('notebook-section-select');
    const createdSection = select.locator(`option:has-text("${SECTION_NAME}")`);
    await expect(createdSection).toHaveCount(1);
    await selectSection(page, (await createdSection.getAttribute('value'))!);
    await expect(page.getByTestId('notebook-entry')).toHaveCount(0);

    // Document
    await page.getByTestId('notebook-entry-create').click();
    await expect(page.getByTestId('notebook-entry')).toHaveCount(1);
    await page.getByTestId('notebook-entry').click();
    await expect(page.getByTestId('notebook-editor')).toBeVisible();
    await page.getByTestId('notebook-title').fill(DOC_TITLE);
    await saveDraft(page, DOC_BODY);

    // Comment: create + resolve + status tab
    await page.getByRole('tab', { name: /Comments|评论/ }).click();
    await page.getByTestId('notebook-comment-input').fill('e2e comment');
    await page.getByTestId('notebook-comment-submit').click();
    await expect(page.getByTestId('notebook-comment').filter({ hasText: 'e2e comment' })).toHaveCount(1);
    await page.getByTestId('notebook-comment-resolve').click();
    await page.getByRole('tab', { name: /Resolved|已解决/ }).click();
    await expect(page.getByTestId('notebook-comment').filter({ hasText: 'e2e comment' })).toHaveCount(1);

    // Revisions: two immutable rows; viewing an old one opens a readonly preview
    await page.getByRole('tab', { name: /Revisions|修订/ }).click();
    await expect(page.getByTestId('notebook-revision')).toHaveCount(2);
    await page.getByTestId('notebook-revision-view').first().click();
    await expect(page.getByTestId('notebook-readonly')).toBeVisible();
    await page.getByRole('tab', { name: /Editor|编辑器/ }).click();

    // Move to the seeded Notes section
    await page.getByTestId('notebook-entry-move').selectOption('section-notes');
    await expect(page.getByTestId('notebook-entry')).toHaveCount(0);
    await selectSection(page, 'section-notes');
    await openEntry(page, DOC_TITLE);

    // Delete → hidden until show-deleted → readonly → restore
    await page.getByTestId('notebook-entry-delete').click();
    await expect(page.getByTestId('notebook-entry')).toHaveCount(0);
    await page.getByLabel(/显示已删除|Show deleted/).check();
    const deleted = page.getByTestId('notebook-entry').filter({ hasText: /已删除|Deleted/ });
    await expect(deleted).toHaveCount(1);
    await deleted.click();
    await expect(page.getByTestId('notebook-entry-restore')).toBeVisible();
    await page.getByTestId('notebook-entry-restore').click();
    await page.getByLabel(/显示已删除|Show deleted/).uncheck();
    await openEntry(page, DOC_TITLE);

    // Export: markdown + provenance sidecar, contents match the saved body
    const files = new Map<string, string>();
    page.on('download', (item) => {
      void item.path().then((filePath) => {
        if (filePath) files.set(item.suggestedFilename(), fs.readFileSync(filePath, 'utf8'));
      });
    });
    await page.getByTestId('notebook-export').click();
    await expect.poll(() => files.size, { timeout: 10_000 }).toBe(2);
    const markdown = [...files.entries()].find(([name]) => name.endsWith('.md'));
    const sidecar = [...files.entries()].find(([name]) => name.endsWith('.sidecar.json'));
    expect(markdown?.[1]).toContain('first body');
    const provenance = JSON.parse(sidecar?.[1] ?? '{}') as Record<string, unknown>;
    expect(JSON.stringify(provenance)).toContain('human');
    await page.screenshot({ path: 'test-results/n3-editor-saved.png', fullPage: true });
  });

  test('content survives a real backend restart', async ({ page }) => {
    await openNotebook(page);
    await selectSection(page, 'section-notes');
    await openEntry(page, DOC_TITLE);
    await stopBackend(backend);
    backend = await startBackend(notebookBins.selected, nbConfigPath);
    await page.reload();
    await expect(page.getByTestId('notebook-page')).toBeVisible();
    await selectSection(page, 'section-notes');
    await openEntry(page, DOC_TITLE);
    await expect(page.getByTestId('notebook-draft')).toHaveValue(DOC_BODY);
  });

  test('a stale CAS keeps the local draft and surfaces the conflict affordances', async ({ page, context }) => {
    await openNotebook(page);
    await selectSection(page, 'section-notes');
    await openEntry(page, DOC_TITLE);

    const peer = await context.newPage();
    await openNotebook(peer);
    await selectSection(peer, 'section-notes');
    await openEntry(peer, DOC_TITLE);
    await saveDraft(peer, '# E2E doc\n\npeer body');

    await page.getByTestId('notebook-draft').fill('# E2E doc\n\nmine body');
    await page.getByTestId('notebook-save').click();
    await expect(page.getByTestId('notebook-conflict')).toBeVisible();
    await expect(page.getByTestId('notebook-draft')).toHaveValue('# E2E doc\n\nmine body');
    await expect(page.getByTestId('notebook-view-current')).toBeVisible();
    await expect(page.getByTestId('notebook-reload')).toBeVisible();

    await page.getByTestId('notebook-view-current').click();
    await expect(page.getByTestId('notebook-current-preview')).toContainText('peer body');
    await page.screenshot({ path: 'test-results/n3-conflict.png', fullPage: true });
    await peer.close();
  });

  test('seeded notebook data stays out of chat; the omitted generation has no notebook UI', async ({ page }) => {
    // Explicit read: seeded sections are reachable through the module actions.
    await openNotebook(page);
    const listed = await invokeModule(page, 'vivy.notebook.sections.list', { limit: 100 });
    const sections = (listed.data as { sections?: { id: string }[] } | undefined)?.sections ?? [];
    expect(sections.map((row) => row.id)).toEqual(expect.arrayContaining(['section-notes', 'section-daily', 'section-weekly', 'section-monthly']));

    // Exclusion: notebook ids/content never leak into the chat surface.
    await page.goto('/');
    await expect(page.locator('body')).not.toContainText('section-notes');
    await expect(page.locator('body')).not.toContainText(DOC_TITLE);

    // Omitted generation: the packed no-notebook backend has no module to
    // serve, so the staged route renders its explicit unavailable state.
    await stopBackend(backend);
    backend = await startBackend(notebookBins.omitted, nbConfigPath);
    await page.goto('/notebook');
    await expect(page.getByTestId('notebook-unavailable')).toBeVisible();
    await page.screenshot({ path: 'test-results/n3-unavailable.png', fullPage: true });
    await stopBackend(backend);
    backend = null;
  });
});
