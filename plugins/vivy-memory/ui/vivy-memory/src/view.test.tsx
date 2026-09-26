// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { PluginHostProvider, type FaceStoreState, type FullUIHost } from '@vivy/ui-sdk';
import { MemoryView } from './view';

const ENTRY = {
  id: 'mem-1',
  content: 'Prefer pnpm over npm for this repo',
  trust: 'user_asserted',
  provenance: 'user',
  evidence_refs: [],
  revision: 3,
  created_at: '2026-09-20T10:00:00Z',
  updated_at: '2026-09-24T16:30:00Z',
};

describe('MemoryView', () => {
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

  it('lists records through module.action.invoke and renders the real entry fields', async () => {
    const host = makeHost(async () => ({ status: 'listed', entries: [ENTRY] }));
    await act(async () => root.render(<PluginHostProvider host={host}><MemoryView /></PluginHostProvider>));

    const calls = (host.rpc.call as ReturnType<typeof vi.fn>).mock.calls;
    expect(calls).toEqual([['module.action.invoke', {
      module_id: 'vivy/memory-bml',
      action_id: 'vivy.memory.list',
      input: {},
    }]]);
    expect(container.textContent).toContain('Prefer pnpm over npm for this repo');
    expect(container.textContent).toContain('plugin.vivy/memory.trust.user_asserted');
  });

  it('drives vivy.memory.search from the search box', async () => {
    const host = makeHost(async (_method, params) => {
      const actionId = (params as { action_id?: string }).action_id;
      return actionId === 'vivy.memory.search' ? { status: 'listed', entries: [] } : { status: 'listed', entries: [ENTRY] };
    });
    await act(async () => root.render(<PluginHostProvider host={host}><MemoryView /></PluginHostProvider>));

    const input = container.querySelector('input');
    expect(input).not.toBeNull();
    const setValue = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!;
    await act(async () => {
      setValue.call(input, 'deploy');
      input!.dispatchEvent(new Event('input', { bubbles: true }));
      await new Promise((resolve) => setTimeout(resolve, 400));
    });

    const calls = (host.rpc.call as ReturnType<typeof vi.fn>).mock.calls;
    expect(calls).toContainEqual(['module.action.invoke', {
      module_id: 'vivy/memory-bml',
      action_id: 'vivy.memory.search',
      input: { query: 'deploy' },
    }]);
    expect(container.textContent).toContain('plugin.vivy/memory.noMatch');
  });

  it('renders the failed outcome as an error, never fabricated rows', async () => {
    const host = makeHost(async () => ({ status: 'failed', reason: 'bml_unavailable' }));
    await act(async () => root.render(<PluginHostProvider host={host}><MemoryView /></PluginHostProvider>));

    expect(container.textContent).toContain('bml_unavailable');
    expect(container.textContent).not.toContain(ENTRY.content);
  });

  it('renders an error state when the invoke rejects', async () => {
    const host = makeHost(async () => { throw new Error('socket closed'); });
    await act(async () => root.render(<PluginHostProvider host={host}><MemoryView /></PluginHostProvider>));

    expect(container.textContent).toContain('socket closed');
  });
});

function makeHost(action: (method: string, params?: unknown) => Promise<unknown>): FullUIHost {
  const state = () => ({ activeSessionId: null, currentRun: null, connection: 'connected' }) as unknown as FaceStoreState;
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
      subscribe: () => () => undefined,
    },
    router: { navigate: vi.fn().mockResolvedValue(undefined), invalidate: vi.fn().mockResolvedValue(undefined) },
    composition: {} as FullUIHost['composition'],
    t: (key) => key,
    registerCleanup: () => ({ active: true, dispose: vi.fn() }),
  };
}
