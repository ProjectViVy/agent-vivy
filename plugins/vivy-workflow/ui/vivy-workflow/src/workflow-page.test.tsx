// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import {
  PluginHostProvider,
  type FaceClientRPC,
  type FaceClientStore,
  type FaceStoreState,
  type FullUIHost,
} from '@vivy/ui-sdk';
import { WorkflowPage } from './WorkflowPage';
import { extension } from './index';

function makeHost(action: (method: string, params?: unknown) => Promise<unknown>): FullUIHost {
  const state = {
    activeSessionId: 'sess-1',
    currentRun: { id: 'run-parent', session_id: 'sess-1', status: 'active', created_at: 1 },
  } as unknown as FaceStoreState;
  const rpc = {
    capabilities: { protocol_version: 'vivy.rpc.v1', capabilities: [] },
    call: vi.fn(action),
    onNotification: vi.fn(() => () => undefined),
    onClose: vi.fn(() => () => undefined),
    close: vi.fn(),
  } as unknown as FaceClientRPC;
  return {
    api: {} as FullUIHost['api'],
    rpc,
    store: {
      getState: () => state,
      getInitialState: () => state,
      setState: vi.fn(),
      subscribe: vi.fn(() => () => undefined),
    } as FaceClientStore<FaceStoreState>,
    router: { navigate: vi.fn().mockResolvedValue(undefined), invalidate: vi.fn().mockResolvedValue(undefined) },
    composition: {} as FullUIHost['composition'],
    t: (key: string) => key,
    registerCleanup: () => ({ active: true, dispose: vi.fn() }),
  };
}

const stubAction = async (method: string): Promise<unknown> => {
  switch (method) {
    case 'inofy.capabilities':
      return {
        engine: 'inofy', engine_version: '0.1.0',
        features: ['drafts', 'revisions', 'run_journal', 'connections'],
        run: { supports_wait: false, supports_resume: false, resume_reasons: [] },
      };
    case 'inofy.nodeTypes':
      return [{ type_id: 'vivy.child-task@1', implementation_id: 'vivy.child-task', display: { title: 'Child task' } }];
    case 'inofy.listWorkflows':
      return { items: [{ workflow_id: 'wf-alpha', revision: 1 }], next_cursor: null };
    case 'inofy.listRuns':
      return { items: [], next_cursor: null };
    case 'inofy.listConnections':
      return { items: [], next_cursor: null };
    default:
      return {};
  }
};

async function click(el: Element) {
  await act(async () => {
    el.dispatchEvent(new window.MouseEvent('mousedown', { bubbles: true }));
    el.dispatchEvent(new window.MouseEvent('mouseup', { bubbles: true }));
    el.dispatchEvent(new window.MouseEvent('click', { bubbles: true }));
  });
}

describe('vivy-workflow extension', () => {
  it('registers a /workflows route and a vivy-group sidebar item on install', () => {
    const registered = { routes: [] as Array<{ path?: string }>, nav: [] as Array<{ group?: string; to?: string }> };
    const composition = {
      routes: { register: vi.fn((_id: string, route: { path?: string }) => { registered.routes.push(route); return { dispose: vi.fn() }; }) },
      navigation: { register: vi.fn((_id: string, item: { group?: string; to?: string }) => { registered.nav.push(item); return { dispose: vi.fn() }; }) },
    } as unknown as FullUIHost['composition'];
    const host: FullUIHost = { ...makeHost(async () => ({})), composition };
    const dispose = extension.install(host);
    expect(registered.routes.map((r) => r.path)).toEqual(['/workflows']);
    expect(registered.nav).toEqual([expect.objectContaining({ group: 'vivy', to: '/workflows' })]);
    dispose?.();
  });
});

describe('WorkflowPage', () => {
  let container: HTMLElement;
  let root: Root;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
  });

  it('renders the module tabs and lists published workflows on the Workflows tab', async () => {
    const host = makeHost(stubAction);
    await act(async () => root.render(<PluginHostProvider host={host}><WorkflowPage /></PluginHostProvider>));
    await vi.waitFor(() => expect(container.textContent).toContain('plugin.vivy/workflow-ui.tab.editor'));

    const workflowsTab = [...container.querySelectorAll('button')].find((b) => b.textContent === 'plugin.vivy/workflow-ui.tab.workflows');
    expect(workflowsTab).toBeTruthy();
    await click(workflowsTab!);
    await vi.waitFor(() => expect(container.textContent).toContain('wf-alpha'));

    const calls = (host.rpc.call as ReturnType<typeof vi.fn>).mock.calls.map(([m]) => m);
    expect(calls).toContain('inofy.nodeTypes');
    expect(calls).toContain('inofy.listWorkflows');
    for (const [, params] of (host.rpc.call as ReturnType<typeof vi.fn>).mock.calls) {
      if (typeof params === 'object' && params !== null) {
        expect((params as { session_id?: string }).session_id).toBe('sess-1');
      }
    }
  });

  it('opens a seeded draft for an unknown workflow id', async () => {
    const host = makeHost(async (method: string) => {
      if (method === 'inofy.loadDraft') {
        const err = new Error('not found') as Error & { data?: { code?: string; message?: string } };
        err.data = { code: 'not_found', message: 'draft not found' };
        throw err;
      }
      return stubAction(method);
    });
    await act(async () => root.render(<PluginHostProvider host={host}><WorkflowPage /></PluginHostProvider>));

    const input = container.querySelector<HTMLInputElement>('input[placeholder="plugin.vivy/workflow-ui.editor.idPlaceholder"]');
    expect(input).toBeTruthy();
    await act(async () => {
      const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')!.set!;
      setter.call(input, 'wf-new');
      input!.dispatchEvent(new window.Event('input', { bubbles: true }));
    });
    const openBtn = [...container.querySelectorAll('button')].find((b) => b.textContent === 'common.open');
    expect(openBtn).toBeTruthy();
    await click(openBtn!);
    await vi.waitFor(() => expect(container.textContent).toContain('plugin.vivy/workflow-ui.editor.newDraftNote'));
  });
});
