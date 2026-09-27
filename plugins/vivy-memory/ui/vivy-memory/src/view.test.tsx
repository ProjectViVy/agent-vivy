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
    expect(calls).toContainEqual(['module.action.invoke', {
      module_id: 'vivy/memory-bml',
      action_id: 'vivy.memory.list',
      input: {},
    }]);
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

  it('fetches status once for the status strip', async () => {
    const host = makeHost(async (_m, params) => {
      const actionId = (params as { action_id?: string }).action_id;
      return actionId === 'vivy.memory.status'
        ? { status: 'listed', available: true, startup_revision: 7, database_present: true }
        : { status: 'listed', entries: [ENTRY] };
    });
    await act(async () => root.render(<PluginHostProvider host={host}><MemoryView /></PluginHostProvider>));

    const calls = (host.rpc.call as ReturnType<typeof vi.fn>).mock.calls;
    expect(calls).toContainEqual(['module.action.invoke', {
      module_id: 'vivy/memory-bml', action_id: 'vivy.memory.status', input: {},
    }]);
  });

  it('adds a record through vivy.memory.add with kind long_term', async () => {
    const added = { ...ENTRY, id: 'mem-2', revision: 1 };
    const host = makeHost(async (_m, params) => {
      const actionId = (params as { action_id?: string }).action_id;
      if (actionId === 'vivy.memory.add') return { status: 'applied', entry: added };
      if (actionId === 'vivy.memory.status') return { status: 'listed', available: true };
      return { status: 'listed', entries: [ENTRY, added] };
    });
    await act(async () => root.render(<PluginHostProvider host={host}><MemoryView /></PluginHostProvider>));

    const newButton = [...container.querySelectorAll('button')].find((b) => b.textContent === 'plugin.vivy/memory.addTitle');
    await act(async () => (newButton as HTMLElement).click());

    const textarea = document.body.querySelector('textarea');
    expect(textarea).not.toBeNull();
    const setValue = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!;
    await act(async () => {
      setValue.call(textarea, 'Remember to ship it');
      textarea!.dispatchEvent(new Event('input', { bubbles: true }));
    });
    const submit = [...document.body.querySelectorAll('[role="dialog"] button')].find((b) => b.textContent === 'plugin.vivy/memory.addTitle');
    await act(async () => (submit as HTMLElement).click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));

    const calls = (host.rpc.call as ReturnType<typeof vi.fn>).mock.calls;
    expect(calls).toContainEqual(['module.action.invoke', {
      module_id: 'vivy/memory-bml',
      action_id: 'vivy.memory.add',
      input: { kind: 'long_term', content: 'Remember to ship it' },
    }]);
  });

  it('updates under revision CAS and surfaces a conflict with a rebase affordance', async () => {
    let updateCalls = 0;
    const host = makeHost(async (_m, params) => {
      const actionId = (params as { action_id?: string }).action_id;
      if (actionId === 'vivy.memory.update') {
        updateCalls += 1;
        return { status: 'failed', reason: 'memory_revision_conflict' };
      }
      if (actionId === 'vivy.memory.get') return { status: 'listed', entries: [{ ...ENTRY, revision: 4 }] };
      if (actionId === 'vivy.memory.status') return { status: 'listed', available: true };
      return { status: 'listed', entries: [ENTRY] };
    });
    await act(async () => root.render(<PluginHostProvider host={host}><MemoryView /></PluginHostProvider>));

    await act(async () => {
      const row = [...container.querySelectorAll('button')].find((b) => b.textContent?.includes(ENTRY.content));
      (row as HTMLElement).click();
    });
    const editButton = [...container.querySelectorAll('button')].find((b) => b.textContent === 'common.edit');
    await act(async () => (editButton as HTMLElement).click());

    const textarea = document.body.querySelector('textarea');
    const setValue = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!;
    await act(async () => {
      setValue.call(textarea, 'changed text');
      textarea!.dispatchEvent(new Event('input', { bubbles: true }));
    });
    const save = [...document.body.querySelectorAll('[role="dialog"] button')].find((b) => b.textContent === 'common.save');
    await act(async () => (save as HTMLElement).click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));

    const calls = (host.rpc.call as ReturnType<typeof vi.fn>).mock.calls;
    expect(calls).toContainEqual(['module.action.invoke', {
      module_id: 'vivy/memory-bml',
      action_id: 'vivy.memory.update',
      input: { id: 'mem-1', content: 'changed text', base_revision: 3 },
    }]);
    expect(updateCalls).toBe(1);
    expect(document.body.textContent).toContain('plugin.vivy/memory.conflict');

    const reload = [...document.body.querySelectorAll('[role="dialog"] button')].find((b) => b.textContent === 'common.refresh');
    await act(async () => (reload as HTMLElement).click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    expect(calls).toContainEqual(['module.action.invoke', {
      module_id: 'vivy/memory-bml', action_id: 'vivy.memory.get', input: { id: 'mem-1' },
    }]);
  });

  it('deletes only with a reason and the base revision', async () => {
    const host = makeHost(async (_m, params) => {
      const actionId = (params as { action_id?: string }).action_id;
      if (actionId === 'vivy.memory.remove') return { status: 'applied', entry: null };
      if (actionId === 'vivy.memory.status') return { status: 'listed', available: true };
      return { status: 'listed', entries: [ENTRY] };
    });
    await act(async () => root.render(<PluginHostProvider host={host}><MemoryView /></PluginHostProvider>));

    await act(async () => {
      const row = [...container.querySelectorAll('button')].find((b) => b.textContent?.includes(ENTRY.content));
      (row as HTMLElement).click();
    });
    const deleteButton = [...container.querySelectorAll('button')].find((b) => b.querySelector('svg.lucide-trash-2, svg[class*="trash"]'));
    await act(async () => (deleteButton as HTMLElement).click());

    const dialogButtons = () => [...document.body.querySelectorAll('[role="dialog"] button, [data-radix-popper-content-wrapper] button')];
    let confirm = dialogButtons().find((b) => b.textContent === 'common.delete');
    expect(confirm).not.toBeNull();
    expect((confirm as HTMLButtonElement).disabled).toBe(true);

    const reason = document.body.querySelector('input[aria-label]') as HTMLInputElement;
    const setValue = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!;
    await act(async () => {
      setValue.call(reason, 'stale');
      reason.dispatchEvent(new Event('input', { bubbles: true }));
    });
    confirm = dialogButtons().find((b) => b.textContent === 'common.delete');
    await act(async () => (confirm as HTMLElement).click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));

    const calls = (host.rpc.call as ReturnType<typeof vi.fn>).mock.calls;
    expect(calls).toContainEqual(['module.action.invoke', {
      module_id: 'vivy/memory-bml',
      action_id: 'vivy.memory.remove',
      input: { id: 'mem-1', reason: 'stale', base_revision: 3 },
    }]);
  });

  it('reads and writes MEMRULES under the digest CAS token', async () => {
    const host = makeHost(async (_m, params) => {
      const actionId = (params as { action_id?: string }).action_id;
      if (actionId === 'vivy.memory.rules.read') return { status: 'listed', content: 'rule one', source: 'file', revision: 'deadbeef' };
      if (actionId === 'vivy.memory.rules.write') return { status: 'applied', entry: null };
      if (actionId === 'vivy.memory.status') return { status: 'listed', available: true };
      return { status: 'listed', entries: [ENTRY] };
    });
    await act(async () => root.render(<PluginHostProvider host={host}><MemoryView /></PluginHostProvider>));

    const rulesTab = [...container.querySelectorAll('button[role="tab"]')].find((b) => b.textContent === 'plugin.vivy/memory.tabRules');
    await act(async () => {
      rulesTab!.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
      (rulesTab as HTMLElement).click();
    });
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));

    expect(container.textContent).toContain('rule one');
    const calls = (host.rpc.call as ReturnType<typeof vi.fn>).mock.calls;
    expect(calls).toContainEqual(['module.action.invoke', {
      module_id: 'vivy/memory-bml', action_id: 'vivy.memory.rules.read', input: {},
    }]);

    const editButton = [...container.querySelectorAll('button')].find((b) => b.textContent?.includes('common.edit'));
    await act(async () => (editButton as HTMLElement).click());
    const textarea = container.querySelector('textarea');
    const setValue = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!;
    await act(async () => {
      setValue.call(textarea, 'rule one\nrule two');
      textarea!.dispatchEvent(new Event('input', { bubbles: true }));
    });
    const save = [...container.querySelectorAll('button')].find((b) => b.textContent === 'common.save');
    await act(async () => (save as HTMLElement).click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));

    expect(calls).toContainEqual(['module.action.invoke', {
      module_id: 'vivy/memory-bml',
      action_id: 'vivy.memory.rules.write',
      input: { content: 'rule one\nrule two', base_revision: 'deadbeef' },
    }]);
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
