import { expect, test, type Page } from '@playwright/test';
import type { ChildProcess } from 'node:child_process';
import { invokeMemory, startBackend, stopBackend } from './memory-backend';
import { memoryBins, memoryRunConfig } from '../playwright.memory.config';

/**
 * E1-BML-04 acceptance: the durable memory page over one real packed backend
 * plus the split Vite origin. Create, restart, search, two-tab stale CAS,
 * tombstone, restart again, and the truthful unavailable state each go through
 * the same action plane the browser uses; independent RPC reads verify what
 * the screen claims.
 */

const RUN = Date.now().toString(36);
const RECORD = `bmluiprobe ${RUN} alpha`;
const CORRECTED = `bmluiprobe ${RUN} beta`;
const CONFLICT_DRAFT = `bmluiprobe ${RUN} stale draft`;

interface MemoryEntry {
  readonly id: string;
  readonly content: string;
  readonly revision: number;
  readonly trust: string;
}

let backend: ChildProcess | null = null;

test.beforeAll(async () => {
  backend = await startBackend(memoryBins.selected, memoryRunConfig);
});

test.afterAll(async () => {
  await stopBackend(backend);
  backend = null;
});

async function restartBackend(): Promise<void> {
  await stopBackend(backend);
  backend = await startBackend(memoryBins.selected, memoryRunConfig);
}

async function openMemory(page: Page): Promise<void> {
  await page.addInitScript(() => {
    localStorage.setItem('vivy.ui.welcome.completed', '1');
    // The first-run persona gate belongs to a fresh uninitialized backend; this
    // spec measures the memory page, so use the product's own "not now" flag.
    localStorage.setItem('vivy.persona.onboarding.dismissed', '1');
  });
  await page.goto('/memory');
  await expect(page.getByRole('button', { name: 'New memory' })).toBeVisible();
}

function row(page: Page, text: string) {
  return page.locator('button').filter({ hasText: text }).first();
}

async function listEntries(page: Page): Promise<MemoryEntry[]> {
  const outcome = await invokeMemory(page, 'vivy.memory.list', { limit: 100 });
  expect(outcome.status).toBe('listed');
  return (outcome.entries ?? []) as MemoryEntry[];
}

async function getOutcome(page: Page, id: string): Promise<Record<string, unknown>> {
  return invokeMemory(page, 'vivy.memory.get', { id });
}

test('memory UI keeps every claim the browser can make true across a restart', async ({ page, context }) => {
  // Create through the page, then prove the action plane holds the same row.
  await openMemory(page);
  await expect(page.getByText('No memories yet')).toBeVisible();

  await page.getByRole('button', { name: 'New memory' }).click();
  const addDialog = page.getByRole('dialog');
  await expect(addDialog.getByRole('heading', { name: 'New memory' })).toBeVisible();
  await addDialog.getByLabel('What should Vivy remember?').fill(RECORD);
  await addDialog.getByRole('button', { name: 'New memory' }).click();
  await expect(addDialog).toBeHidden();

  const created = (await listEntries(page)).find((entry) => entry.content === RECORD);
  expect(created, 'created record must be readable through the action plane').toBeTruthy();
  expect(created!.revision).toBe(1);
  await expect(row(page, RECORD)).toBeVisible();
  await expect(page.getByText(created!.id, { exact: true })).toBeVisible();
  await expect(page.getByText(/Revision 1$/)).toBeVisible();

  // Restart the real process; reload the browser and read the same record.
  await restartBackend();
  await page.reload();
  await expect(page.getByRole('button', { name: 'New memory' })).toBeVisible();
  await expect(row(page, RECORD)).toBeVisible();
  await row(page, RECORD).click();
  await expect(page.getByText(created!.id, { exact: true })).toBeVisible();
  await expect(page.getByText(/Revision 1$/)).toBeVisible();
  const reopened = (await getOutcome(page, created!.id)) as {
    status?: string;
    entries?: MemoryEntry[];
  };
  expect(reopened.status).toBe('listed');
  expect(reopened.entries?.[0]?.id).toBe(created!.id);
  expect(reopened.entries?.[0]?.revision).toBe(1);
  expect(reopened.entries?.[0]?.content).toBe(RECORD);

  // Search reaches the same backend record; a miss never fabricates a row.
  const search = page.getByPlaceholder('Search memories');
  await search.fill('alpha');
  await expect(row(page, RECORD)).toBeVisible();
  await search.fill('zzz-no-such-memory-token');
  await expect(page.getByText('No matching memories')).toBeVisible();
  await expect(page.locator('button').filter({ hasText: 'zzz-no-such-memory-token' })).toHaveCount(0);
  await search.fill('');
  await expect(row(page, RECORD)).toBeVisible();

  // Two tabs share the pre-restart revision; the second save must conflict and
  // keep its draft instead of overwriting the first tab's applied body.
  const stalePage = await context.newPage();
  await openMemory(stalePage);
  await expect(row(stalePage, RECORD)).toBeVisible();

  await row(page, RECORD).click();
  await page.getByRole('button', { name: 'Edit', exact: true }).click();
  const editDialog = page.getByRole('dialog');
  await editDialog.getByLabel('What should Vivy remember?').fill(CORRECTED);

  await row(stalePage, RECORD).click();
  await stalePage.getByRole('button', { name: 'Edit', exact: true }).click();
  const staleDialog = stalePage.getByRole('dialog');
  await expect(staleDialog.getByLabel('What should Vivy remember?')).toHaveValue(RECORD);
  await staleDialog.getByLabel('What should Vivy remember?').fill(CONFLICT_DRAFT);

  await editDialog.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(editDialog).toBeHidden();
  await expect(row(page, CORRECTED)).toBeVisible();
  await expect(page.getByText(/Revision 2$/)).toBeVisible();
  const applied = (await getOutcome(page, created!.id)) as {
    status?: string;
    entries?: MemoryEntry[];
  };
  expect(applied.status).toBe('listed');
  expect(applied.entries?.[0]?.content).toBe(CORRECTED);
  expect(applied.entries?.[0]?.revision).toBe(2);

  await staleDialog.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(
    staleDialog.getByText('This record changed since you opened it. Reload the latest version, then retry.'),
  ).toBeVisible();
  await expect(staleDialog.getByLabel('What should Vivy remember?')).toHaveValue(CONFLICT_DRAFT);
  const afterConflict = (await getOutcome(page, created!.id)) as {
    status?: string;
    entries?: MemoryEntry[];
  };
  expect(afterConflict.status).toBe('listed');
  expect(afterConflict.entries?.[0]?.content).toBe(CORRECTED);
  expect(afterConflict.entries?.[0]?.revision).toBe(2);

  // Refresh rebases the stale dialog onto the authoritative revision.
  await staleDialog.getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(staleDialog.getByLabel('What should Vivy remember?')).toHaveValue(CORRECTED);
  await expect(staleDialog.getByRole('alert')).toHaveCount(0);
  await staleDialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(staleDialog).toBeHidden();
  await stalePage.close();

  // Tombstone through the page: the reason gate is real, and the list, the
  // action plane, a reload and a restart all agree the record is gone.
  await expect(row(page, CORRECTED)).toBeVisible();
  await row(page, CORRECTED).click();
  await page.getByRole('button', { name: 'Delete', exact: true }).click();
  const deleteDialog = page.getByRole('dialog');
  await expect(deleteDialog.getByRole('heading', { name: 'Delete memory' })).toBeVisible();
  const deleteButton = deleteDialog.getByRole('button', { name: 'Delete', exact: true });
  await expect(deleteButton).toBeDisabled();
  await deleteDialog.getByLabel('Reason for deletion').fill('bml ui e2e tombstone');
  await expect(deleteButton).toBeEnabled();
  await deleteButton.click();
  await expect(deleteDialog).toBeHidden();
  await expect(row(page, CORRECTED)).toHaveCount(0);
  await expect(page.getByText('No memories yet')).toBeVisible();
  expect(await getOutcome(page, created!.id)).toMatchObject({ status: 'failed', reason: 'memory_not_found' });

  await page.reload();
  await expect(page.getByRole('button', { name: 'New memory' })).toBeVisible();
  await expect(page.getByText('No memories yet')).toBeVisible();
  await expect(page.locator('button').filter({ hasText: CORRECTED })).toHaveCount(0);

  await restartBackend();
  await page.reload();
  await expect(page.getByRole('button', { name: 'New memory' })).toBeVisible();
  await expect(page.getByText('No memories yet')).toBeVisible();
  expect(await getOutcome(page, created!.id)).toMatchObject({ status: 'failed', reason: 'memory_not_found' });

  // Outage: the page must report the failed read, never a successful empty list.
  await stopBackend(backend);
  backend = null;
  await page.getByPlaceholder('Search memories').fill('alpha');
  await expect(page.getByRole('alert')).toBeVisible({ timeout: 20_000 });
  await expect(page.getByText('Failed to load memories', { exact: true })).toBeVisible({ timeout: 20_000 });
  await expect(page.locator('button').filter({ hasText: 'alpha' })).toHaveCount(0);
});
