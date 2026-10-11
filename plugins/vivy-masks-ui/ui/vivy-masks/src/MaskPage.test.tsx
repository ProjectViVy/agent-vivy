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
    expect(container.querySelector('select')).toBeNull();
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
    expect(container.querySelector('[data-active-mask]')?.getAttribute('data-active-mask')).toBe('builtin/writer');
  });

  it('copies a bundled mask into a custom definition without changing session selection', async () => {
    const source = {
      id: 'bundle/install-id/pm', name: 'Project Manager', description: 'Plans work',
      digest: 'a'.repeat(64), generation_id: 'generation', revision: 1, built_in: false,
      origin: 'bundle', available: true, catalog_epoch: 8,
      bundle: { release: { install_id: 'install-id', bundle_id: 'bmad/experts', release_digest: 'b'.repeat(64), release_sequence: 2 }, local_id: 'pm' },
      recommended_skills: [{ ref: { install_id: 'install-id', kind: 'skill', local_id: 'pm' }, skill_id: 'b_installid_pm', name: 'PM Skill', available: true }],
    };
    const copied = {
      ...source, id: 'custom/copied-pm', origin: 'custom', body: 'Plan the work carefully.', built_in: false,
    };
    const initialCatalog = { items: [source], next_after_id: '', catalog_epoch: 8 };
    const call = vi.fn(async (_method: string, params?: unknown) => {
      const action = (params as { action_id?: string; input?: { id?: string } } | undefined)?.action_id;
      if (action === 'vivy.masks.catalog.list') return initialCatalog;
      if (action === 'vivy.masks.selection.get') return { session_id: 'session-a', mask_id: '', revision: 1, available: true, inactive_reason: '' };
      if (action === 'vivy.masks.catalog.get') return { ...source, body: 'Plan the work carefully.' };
      if (action === 'vivy.masks.catalog.copy') return copied;
      return {};
    });
    const host = makeHost('session-a', call);
    await act(async () => root.render(<PluginHostProvider host={host}><MaskPage /></PluginHostProvider>));
    await act(async () => {
      container.querySelector<HTMLButtonElement>('[data-mask-id="bundle/install-id/pm"] button')!.click();
      await Promise.resolve();
    });
    expect(container.querySelector('a[href="/toolkits"]')).not.toBeNull();
    await act(async () => {
      [...container.querySelectorAll('button')].find((button) => button.textContent?.includes('copyToCustom'))!.click();
      await Promise.resolve();
    });

    const copyCall = call.mock.calls.find(([, params]) => (params as { action_id?: string }).action_id === 'vivy.masks.catalog.copy');
    expect(copyCall?.[1]).toEqual(expect.objectContaining({
      module_id: 'vivy/masks',
      input: expect.objectContaining({ source_id: source.id, expected_catalog_epoch: 8, operation_id: expect.any(String) }),
    }));
    expect(call.mock.calls.some(([, params]) => (params as { action_id?: string }).action_id === 'vivy.masks.selection.set')).toBe(false);
    expect(container.textContent).toContain('bundleCopyCreated');
    expect(document.body.querySelector('[data-mask-editor]')).not.toBeNull();
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
