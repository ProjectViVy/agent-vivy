// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { PluginHostProvider, type FaceStoreState, type FullUIHost } from '@vivy/ui-sdk';
import { MaskPage } from './MaskPage';

describe('MaskPage', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
  });

  it('keeps catalog management available and selection disabled without an active session', async () => {
    const host = makeHost(null, async () => ({ items: [], next_after_id: '' }));
    await act(async () => root.render(<PluginHostProvider host={host}><MaskPage /></PluginHostProvider>));

    expect(container.querySelector('[data-testid="mask-page"]')).not.toBeNull();
    expect(container.textContent).toContain('plugin.vivy/masks-ui.noActiveSession');
    expect(container.querySelector('button[data-mask-action="new"]')).not.toBeNull();
    expect(container.querySelector('select')).toHaveProperty('disabled', true);
  });

  it('does not show session A after the active host session changes to B before A resolves', async () => {
    const selectionA = deferred<unknown>();
    const selectionB = deferred<unknown>();
    const catalog = {
      items: [{ id: 'builtin/writer', name: 'Writer', description: '', digest: 'a'.repeat(64), generation_id: 'generation', revision: 1, built_in: true }],
      next_after_id: '',
    };
    let activeSessionId: string | null = 'session-a';
    let liveState = { activeSessionId, currentRun: null, connection: 'connected' } as unknown as FaceStoreState;
    const listeners = new Set<() => void>();
    const state = () => liveState;
    const host = makeHost(activeSessionId, async (_method, params) => {
      if ((params as { action_id?: string } | undefined)?.action_id !== 'vivy.masks.selection.get') return catalog;
      return (params as { input?: { session_id?: string } }).input?.session_id === 'session-a' ? selectionA.promise : selectionB.promise;
    }, state, listeners);

    await act(async () => root.render(<PluginHostProvider host={host}><MaskPage /></PluginHostProvider>));
    await act(async () => {
      activeSessionId = 'session-b';
      liveState = { ...liveState, activeSessionId };
      for (const listener of listeners) listener();
      await Promise.resolve();
    });
    await act(async () => {
      selectionB.resolve({ session_id: 'session-b', mask_id: 'builtin/writer', revision: 7, available: true, inactive_reason: '' });
      await Promise.resolve();
    });
    await act(async () => {
      selectionA.resolve({ session_id: 'session-a', mask_id: 'builtin/programmer', revision: 41, available: true, inactive_reason: '' });
      await Promise.resolve();
    });

    const selectionSessions = (host.rpc.call as ReturnType<typeof vi.fn>).mock.calls
      .filter(([method]) => method === 'module.action.invoke')
      .map(([, params]) => (params as { input?: { session_id?: string } }).input?.session_id)
      .filter((sessionId): sessionId is string => typeof sessionId === 'string');
    expect(selectionSessions).toContain('session-b');
    expect(container.querySelector('select')).toHaveProperty('value', 'builtin/writer');
  });
});

function makeHost(
  activeSessionId: string | null,
  action: (method: string, params?: unknown) => Promise<unknown>,
  stateOverride?: () => FaceStoreState,
  listeners = new Set<() => void>(),
): FullUIHost {
  const stableState = { activeSessionId, currentRun: null, connection: 'connected' } as unknown as FaceStoreState;
  const state = stateOverride ?? (() => stableState);
  const rpc = {
    capabilities: { protocol_version: 'vivy.rpc.v1', capabilities: ['module.action.invoke'] },
    call: vi.fn(action),
    onNotification: vi.fn(() => () => undefined),
    onClose: vi.fn(() => () => undefined),
    close: vi.fn(),
  } as unknown as FullUIHost['rpc'];
  return {
    api: {} as FullUIHost['api'],
    rpc,
    store: {
      getState: state,
      getInitialState: state,
      setState: vi.fn(),
      subscribe: (listener: (state: FaceStoreState, previousState: FaceStoreState) => void) => {
        const notify = () => listener(state(), state());
        listeners.add(notify);
        return () => { listeners.delete(notify); };
      },
    },
    router: { navigate: vi.fn().mockResolvedValue(undefined), invalidate: vi.fn().mockResolvedValue(undefined) },
    composition: {} as FullUIHost['composition'],
    t: (key) => key,
    registerCleanup: () => ({ active: true, dispose: vi.fn() }),
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}
