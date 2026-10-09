import { describe, expect, it, vi } from 'vitest';
import type { FaceClientRPC, FaceClientStore, FaceStoreState, FullUIHost } from '@vivy/ui-sdk';
import { FaceBridge, normalizeJournalEvent } from './face-bridge';
import { WorkflowClient } from './client';
import { TransportError } from './studio/transport';

function makeRpc(action?: (method: string, params?: unknown) => Promise<unknown>) {
  const notifications = new Map<string, (params: unknown) => void>();
  const closeListeners = new Set<() => void>();
  const rpc: FaceClientRPC = {
    capabilities: { protocol_version: 'vivy.rpc.v1', capabilities: [] },
    call: vi.fn(action ?? (async () => ({}))) as unknown as FaceClientRPC['call'],
    onNotification: vi.fn((method: string, listener: (params: unknown) => void) => {
      notifications.set(method, listener);
      return () => { notifications.delete(method); };
    }),
    onClose: vi.fn((listener: () => void) => {
      closeListeners.add(listener);
      return () => { closeListeners.delete(listener); };
    }),
    close: vi.fn(),
  };
  return { rpc, notifications, closeListeners };
}

function makeStore(overrides: Partial<FaceStoreState> = {}): FaceClientStore<FaceStoreState> {
  const state = {
    activeSessionId: 'sess-1',
    currentRun: { id: 'run-parent', session_id: 'sess-1', status: 'active', created_at: 1 },
    ...overrides,
  } as unknown as FaceStoreState;
  return {
    getState: () => state,
    getInitialState: () => state,
    setState: vi.fn(),
    subscribe: vi.fn(() => () => undefined),
  } as unknown as FaceClientStore<FaceStoreState>;
}

function makeHost(overrides: Partial<FaceStoreState> = {}, action?: (method: string, params?: unknown) => Promise<unknown>) {
  const { rpc, notifications, closeListeners } = makeRpc(action);
  return { rpc, store: makeStore(overrides), notifications, closeListeners };
}

describe('FaceBridge.call', () => {
  it('forwards the inofy.* method verbatim with the active session bound', async () => {
    const host = makeHost();
    const bridge = new FaceBridge(host.rpc, host.store);
    await bridge.call('inofy.listWorkflows', { cursor: 'c1' });
    expect(host.rpc.call).toHaveBeenCalledWith('inofy.listWorkflows', { cursor: 'c1', session_id: 'sess-1' });
  });

  it('omits session_id when the host has no active session (fail-closed)', async () => {
    const host = makeHost({ activeSessionId: null });
    const bridge = new FaceBridge(host.rpc, host.store);
    await bridge.call('inofy.listWorkflows', {});
    const [, params] = (host.rpc.call as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(params).not.toHaveProperty('session_id');
  });

  it('injects parent_run_id and a bounded operation_id for inofy.startRun', async () => {
    const host = makeHost();
    const bridge = new FaceBridge(host.rpc, host.store);
    await bridge.call('inofy.startRun', { workflow: 'wf-1', revision: 2 });
    expect(host.rpc.call).toHaveBeenCalledWith('inofy.startRun', {
      workflow: 'wf-1', revision: 2, session_id: 'sess-1',
      parent_run_id: 'run-parent',
      operation_id: expect.stringMatching(/^[0-9a-f-]{36}$/),
    });
  });

  it('leaves non-startRun calls without run binding params', async () => {
    const host = makeHost();
    const bridge = new FaceBridge(host.rpc, host.store);
    await bridge.call('inofy.saveDraft', { workflow: 'wf', artifact: {}, etag: null });
    const [, params] = (host.rpc.call as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(params).not.toHaveProperty('parent_run_id');
    expect(params).not.toHaveProperty('operation_id');
  });
});

describe('WorkflowClient error surface', () => {
  it('maps wire error codes onto App §11.4 statuses', async () => {
    const host = makeHost({}, async () => {
      const err = new Error('workflow not found') as Error & { code?: number; data?: { code?: string } };
      err.code = -32004;
      err.data = { code: 'not_found' };
      throw err;
    });
    const client = new WorkflowClient(new FaceBridge(host.rpc, host.store));
    await expect(client.loadDraft('wf-1')).rejects.toMatchObject({ status: 412, code: 'not_found' });
  });

  it('maps revision_conflict onto 412 for the editor CAS branch', async () => {
    const host = makeHost({}, async () => {
      const err = new Error('stale etag') as Error & { data?: { code?: string } };
      err.data = { code: 'revision_conflict' };
      throw err;
    });
    const client = new WorkflowClient(new FaceBridge(host.rpc, host.store));
    try {
      await client.saveDraft('wf', { definition: { schema_version: 'inofy.workflow/v1', graph: { nodes: [], edges: [], exits: [] } } }, 'e1');
      expect.unreachable();
    } catch (e) {
      expect(e).toBeInstanceOf(TransportError);
      expect((e as TransportError).status).toBe(412);
      expect((e as TransportError).code).toBe('revision_conflict');
    }
  });

  it('sends explicit create intent when saving a missing draft', async () => {
    const host = makeHost({}, async () => ({ workflow: 'wf-new', etag: 'etag-1', artifact: {} }));
    const client = new WorkflowClient(new FaceBridge(host.rpc, host.store));
    const artifact = { definition: { schema_version: 'inofy.workflow/v1', graph: { nodes: [], edges: [], exits: [] } } };

    await client.saveDraft('wf-new', artifact, null);

    expect(host.rpc.call).toHaveBeenCalledWith('inofy.saveDraft', {
      workflow: 'wf-new', artifact, create: true, session_id: 'sess-1',
    });
  });

  it('sends a concrete current etag when editing a draft', async () => {
    const host = makeHost({}, async () => ({ workflow: 'wf', etag: 'etag-2', artifact: {} }));
    const client = new WorkflowClient(new FaceBridge(host.rpc, host.store));
    const artifact = { definition: { schema_version: 'inofy.workflow/v1', graph: { nodes: [], edges: [], exits: [] } } };

    await client.saveDraft('wf', artifact, 'etag-1');

    expect(host.rpc.call).toHaveBeenCalledWith('inofy.saveDraft', {
      workflow: 'wf', artifact, etag: 'etag-1', session_id: 'sess-1',
    });
  });

  it('rejects an empty edit etag before making an rpc call', async () => {
    const host = makeHost();
    const client = new WorkflowClient(new FaceBridge(host.rpc, host.store));
    const artifact = { definition: { schema_version: 'inofy.workflow/v1', graph: { nodes: [], edges: [], exits: [] } } };

    await expect(client.saveDraft('wf', artifact, '')).rejects.toThrow(/etag/i);
    expect(host.rpc.call).not.toHaveBeenCalled();
  });
});

describe('normalizeJournalEvent', () => {
  it('maps journal vocabulary onto the editor RunEvent shape', () => {
    const event = normalizeJournalEvent({
      seq: 7, type: 'workflow.node.attempt', created_at: 100,
      payload: { node_key: '/graph/nodes/a', attempt: 2 },
    });
    expect(event).toEqual({
      seq: 7, kind: 'node_attempt', path: '/graph/nodes/a', attempt: 2,
      data: { node_key: '/graph/nodes/a', attempt: 2 },
    });
  });

  it('maps terminal run events onto engine kinds', () => {
    expect(normalizeJournalEvent({ seq: 9, type: 'run.completed', payload: {} }).kind).toBe('run_succeeded');
    expect(normalizeJournalEvent({ seq: 10, type: 'run.failed', payload: {} }).kind).toBe('run_failed');
    expect(normalizeJournalEvent({ seq: 11, type: 'run.cancelled', payload: {} }).kind).toBe('run_cancelled');
  });

  it('keeps unmapped types verbatim', () => {
    expect(normalizeJournalEvent({ seq: 1, type: 'policy.evaluated', payload: {} }).kind).toBe('policy.evaluated');
  });
});

describe('WorkflowClient.events', () => {
  it('normalizes paged journal rows into RunEvents', async () => {
    const host = makeHost({}, async (method, params) => {
      if (method === 'inofy.events') {
        return {
          events: [
            { seq: 3, type: 'workflow.node.completed', at: 5, data: { node_key: '/graph/nodes/a', result_blob_id: 'wf/x' } },
            { seq: 4, type: 'run.completed', at: 6, data: {} },
          ],
          next_cursor: null,
        };
      }
      return {};
    });
    const client = new WorkflowClient(new FaceBridge(host.rpc, host.store));
    const page = await client.events('run-1');
    expect(page.events).toEqual([
      { seq: 3, kind: 'node_completed', path: '/graph/nodes/a', attempt: undefined, data: { node_key: '/graph/nodes/a', result_blob_id: 'wf/x' } },
      { seq: 4, kind: 'run_succeeded', path: undefined, attempt: undefined, data: {} },
    ]);
    expect(host.rpc.call).toHaveBeenCalledWith('inofy.events', { run_id: 'run-1', after: undefined, session_id: 'sess-1' });
  });
});

describe('FaceBridge.subscribe (inofy.events over run/subscribe)', () => {
  function subscribedRpc() {
    const notifications = new Map<string, (params: unknown) => void>();
    const calls: Array<{ method: string; params: unknown }> = [];
    const rpc: FaceClientRPC = {
      capabilities: { protocol_version: 'vivy.rpc.v1', capabilities: [] },
      call: vi.fn(async (method: string, params?: unknown) => {
        calls.push({ method, params });
        if (method === 'run/subscribe') return { subscription_id: `sub-${calls.filter((c) => c.method === 'run/subscribe').length}` };
        return {};
      }) as unknown as FaceClientRPC['call'],
      onNotification: vi.fn((method: string, listener: (params: unknown) => void) => {
        notifications.set(method, listener);
        return () => { notifications.delete(method); };
      }),
      onClose: vi.fn(() => () => undefined),
      close: vi.fn(),
    };
    return { rpc, notifications, calls };
  }

  it('subscribes with run/subscribe and normalizes run/event notifications', async () => {
    const { rpc, notifications, calls } = subscribedRpc();
    const bridge = new FaceBridge(rpc, makeStore());
    const seen: Array<{ seq: number; kind: string; path?: string }> = [];
    const sub = bridge.subscribe('inofy.events', { run_id: 'run-9', after: 0 }, (e) => seen.push(e as { seq: number; kind: string; path?: string }));
    await vi.waitFor(() => expect(calls.some((c) => c.method === 'run/subscribe')).toBe(true));
    expect(calls[0].params).toEqual({ run_id: 'run-9', after_seq: 0 });

    notifications.get('run/event')?.({
      subscription_id: 'sub-1',
      event: { run_id: 'run-9', seq: 4, type: 'workflow.node.started', created_at: 1, payload_version: 1, payload: { node_key: '/graph/nodes/n1' } },
    });
    notifications.get('run/event')?.({
      subscription_id: 'sub-1',
      event: { run_id: 'run-9', seq: 4, type: 'workflow.node.started', created_at: 1, payload_version: 1, payload: { node_key: '/graph/nodes/n1' } },
    });
    expect(seen).toEqual([{
      seq: 4, kind: 'node_started', path: '/graph/nodes/n1', attempt: undefined,
      data: { node_key: '/graph/nodes/n1' },
    }]);
    sub.close();
  });

  it('drops notifications for other subscriptions and closes on terminal events', async () => {
    const { rpc, notifications } = subscribedRpc();
    const bridge = new FaceBridge(rpc, makeStore());
    const seen: number[] = [];
    bridge.subscribe('inofy.events', { run_id: 'run-1', after: 0 }, (e) => seen.push((e as { seq: number }).seq));
    await vi.waitFor(() => expect((rpc.call as ReturnType<typeof vi.fn>).mock.calls.some(([m]) => m === 'run/subscribe')).toBe(true));
    notifications.get('run/event')?.({ subscription_id: 'other', event: { run_id: 'run-1', seq: 1, type: 'workflow.node.started', payload: {} } });
    notifications.get('run/event')?.({ subscription_id: 'sub-1', event: { run_id: 'run-1', seq: 1, type: 'workflow.node.started', payload: {} } });
    notifications.get('run/event')?.({ subscription_id: 'sub-1', event: { run_id: 'run-1', seq: 2, type: 'run.completed', payload: {} } });
    notifications.get('run/event')?.({ subscription_id: 'sub-1', event: { run_id: 'run-1', seq: 3, type: 'workflow.node.started', payload: {} } });
    expect(seen).toEqual([1, 2]);
    expect((rpc.call as ReturnType<typeof vi.fn>).mock.calls.some(([m]) => m === 'run/unsubscribe')).toBe(true);
  });

  it('resubscribes from the committed cursor after a stream error', async () => {
    const { rpc, notifications, calls } = subscribedRpc();
    const bridge = new FaceBridge(rpc, makeStore());
    const seen: number[] = [];
    bridge.subscribe('inofy.events', { run_id: 'run-2', after: 0 }, (e) => seen.push((e as { seq: number }).seq));
    await vi.waitFor(() => expect(calls.filter((c) => c.method === 'run/subscribe').length).toBe(1));
    notifications.get('run/event')?.({ subscription_id: 'sub-1', event: { run_id: 'run-2', seq: 8, type: 'workflow.node.started', payload: {} } });
    notifications.get('run/stream_error')?.({ subscription_id: 'sub-1', message: 'replay failed' });
    await vi.waitFor(() => expect(calls.filter((c) => c.method === 'run/subscribe').length).toBe(2), { timeout: 3000 });
    expect(calls.filter((c) => c.method === 'run/subscribe')[1].params).toEqual({ run_id: 'run-2', after_seq: 8 });
    notifications.get('run/event')?.({ subscription_id: 'sub-2', event: { run_id: 'run-2', seq: 9, type: 'workflow.node.completed', payload: {} } });
    expect(seen).toEqual([8, 9]);
  });
});
