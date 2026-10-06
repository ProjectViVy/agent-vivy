import { expect, it, vi } from 'vitest';
import type { FullUIHost } from '@vivy/ui-sdk';
import { MaskSession } from './mask-session';

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
