import { expect, it, vi } from 'vitest';
import type { FullUIHost } from '@vivy/ui-sdk';
import { MaskSession } from './mask-session';

it('keeps a catalog failure visible when selection refresh succeeds', async () => {
  const host = { store: { getState: () => ({ activeSessionId: 'A' }) }, rpc: {} } as unknown as FullUIHost;
  const session = new MaskSession(host);
  vi.spyOn(session.client, 'list').mockRejectedValue(new Error('Catalog disconnected'));
  vi.spyOn(session.client, 'getSelection').mockResolvedValue({ session_id: 'A', mask_id: '', revision: 1, available: true, inactive_reason: '' });
  await session.refreshCatalog();
  await session.refreshSelection();
  expect(session.getSnapshot().error?.message).toBe('Catalog disconnected');
});

it('defers same-session refresh until the pending selection write commits', async () => {
  const original = { session_id: 'A', mask_id: '', revision: 1, available: true, inactive_reason: '' };
  const committed = { ...original, mask_id: 'builtin/writer', revision: 2 };
  let server = original;
  let commit!: () => void;
  const read = vi.fn(async () => server);
  const host = { store: { getState: () => ({ activeSessionId: 'A' }) }, rpc: {} } as unknown as FullUIHost;
  const session = new MaskSession(host);
  vi.spyOn(session.client, 'getSelection').mockImplementation(read);
  const set = vi.spyOn(session.client, 'setSelection').mockImplementation(() => new Promise((resolve) => {
    commit = () => { server = committed; resolve(committed); };
  }));
  await session.refreshSelection();
  const writing = session.select('builtin/writer');
  await session.refreshSelection();
  expect(read).toHaveBeenCalledTimes(1);
  expect(session.getSnapshot().selectionPending).toBe(true);
  expect(await session.select('builtin/programmer')).toBe(false);
  expect(set).toHaveBeenCalledTimes(1);
  commit();
  expect(await writing).toBe(true);
  expect(read).toHaveBeenCalledTimes(2);
  expect(session.getSnapshot().selection).toEqual(committed);
  expect(session.getSnapshot().selectionPending).toBe(false);
});

it('pins Toolkit mask selection to the catalog epoch read by the session', async () => {
  const host = { store: { getState: () => ({ activeSessionId: 'A' }) }, rpc: {} } as unknown as FullUIHost;
  const session = new MaskSession(host);
  vi.spyOn(session.client, 'list').mockResolvedValue({ items: [], next_after_id: '', catalog_epoch: 12 });
  vi.spyOn(session.client, 'getSelection').mockResolvedValue({ session_id: 'A', mask_id: '', revision: 2, available: true, inactive_reason: '' });
  const set = vi.spyOn(session.client, 'setSelection').mockResolvedValue({ session_id: 'A', mask_id: 'bundle/install/pm', revision: 3, available: true, inactive_reason: '' });

  await session.refreshCatalog();
  await session.refreshSelection();
  expect(await session.select('bundle/install/pm')).toBe(true);
  expect(set).toHaveBeenCalledWith({ session_id: 'A', mask_id: 'bundle/install/pm', expected_revision: 2, expected_catalog_epoch: 12 });
});
