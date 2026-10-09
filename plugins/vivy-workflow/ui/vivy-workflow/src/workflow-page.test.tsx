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
import { blankArtifact } from './seed';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

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

  it('does not let two editors overwrite the first missing-draft creator', async () => {
    const stored = new Map<string, { etag: string; artifact: ReturnType<typeof blankArtifact> }>();
    const createHost = () => makeHost(async (method: string, params?: unknown) => {
      const body = params as { workflow?: string; create?: boolean; artifact?: ReturnType<typeof blankArtifact> } | undefined;
      if (method === 'inofy.loadDraft') {
        const existing = stored.get(String(body?.workflow));
        if (existing) return { workflow: body?.workflow, ...existing };
        const err = new Error('draft not found') as Error & { data?: { code?: string; message?: string } };
        err.data = { code: 'not_found', message: 'draft not found' };
        throw err;
      }
      if (method === 'inofy.saveDraft') {
        if (body?.create !== true) throw new Error('missing explicit create intent');
        const workflow = String(body.workflow);
        if (stored.has(workflow)) {
          const err = new Error('draft already exists') as Error & { data?: { code?: string; message?: string } };
          err.data = { code: 'revision_conflict', message: 'draft already exists' };
          throw err;
        }
        const result = { etag: 'etag-created', artifact: body.artifact! };
        stored.set(workflow, result);
        return { workflow, ...result };
      }
      return stubAction(method);
    });

    const secondContainer = document.createElement('div');
    document.body.append(secondContainer);
    const secondRoot = createRoot(secondContainer);
    const firstHost = createHost();
    const secondHost = createHost();
    try {
      await act(async () => {
        root.render(<PluginHostProvider host={firstHost}><WorkflowPage /></PluginHostProvider>);
        secondRoot.render(<PluginHostProvider host={secondHost}><WorkflowPage /></PluginHostProvider>);
      });

      const openMissingDraft = async (target: HTMLElement) => {
        const input = target.querySelector<HTMLInputElement>('input[placeholder="plugin.vivy/workflow-ui.editor.idPlaceholder"]')!;
        await act(async () => {
          const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')!.set!;
          setter.call(input, 'wf-race');
          input.dispatchEvent(new window.Event('input', { bubbles: true }));
        });
        await click([...target.querySelectorAll('button')].find((b) => b.textContent === 'common.open')!);
      };
      await openMissingDraft(container);
      await openMissingDraft(secondContainer);
      await vi.waitFor(() => {
        expect(container.textContent).toContain('plugin.vivy/workflow-ui.editor.newDraftNote');
        expect(secondContainer.textContent).toContain('plugin.vivy/workflow-ui.editor.newDraftNote');
      });

      await click([...container.querySelectorAll('button')].find((b) => b.textContent === 'plugin.vivy/workflow-ui.editor.save')!);
      await vi.waitFor(() => expect(stored.has('wf-race')).toBe(true));
      const winningArtifact = stored.get('wf-race')!.artifact;

      await click([...secondContainer.querySelectorAll('button')].find((b) => b.textContent === 'plugin.vivy/workflow-ui.editor.save')!);
      await vi.waitFor(() => expect(secondContainer.querySelector('[role="alert"]')?.textContent).toContain('revision_conflict'));

      expect(stored.get('wf-race')!.artifact).toBe(winningArtifact);
      expect(stored.get('wf-race')!.artifact.definition.graph.nodes[0]?.config).toEqual({ task: 'wf-race' });
    } finally {
      await act(async () => secondRoot.unmount());
      secondContainer.remove();
    }
  });

  it('edits published revision content using the current draft etag', async () => {
    const revisionArtifact = blankArtifact('published revision task');
    const currentDraft = blankArtifact('newer current draft');
    const host = makeHost(async (method: string) => {
      if (method === 'inofy.listWorkflows') return { items: [{ workflow_id: 'wf-alpha', revision: 2 }], next_cursor: null };
      if (method === 'inofy.getRevision') return { workflow: 'wf-alpha', revision: 2, artifact: revisionArtifact };
      if (method === 'inofy.loadDraft') return { workflow: 'wf-alpha', etag: 'etag-current', artifact: currentDraft };
      if (method === 'inofy.saveDraft') return { workflow: 'wf-alpha', etag: 'etag-next', artifact: revisionArtifact };
      return stubAction(method);
    });
    await act(async () => root.render(<PluginHostProvider host={host}><WorkflowPage /></PluginHostProvider>));
    await click([...container.querySelectorAll('button')].find((b) => b.textContent === 'plugin.vivy/workflow-ui.tab.workflows')!);
    await vi.waitFor(() => expect(container.textContent).toContain('wf-alpha'));
    await click([...container.querySelectorAll('button')].find((b) => b.textContent === 'plugin.vivy/workflow-ui.workflows.openDraft')!);
    await vi.waitFor(() => expect(container.textContent).toContain('plugin.vivy/workflow-ui.editor.forkNote'));
    await click([...container.querySelectorAll('button')].find((b) => b.textContent === 'plugin.vivy/workflow-ui.editor.save')!);

    await vi.waitFor(() => expect((host.rpc.call as ReturnType<typeof vi.fn>).mock.calls.some(([method]) => method === 'inofy.saveDraft')).toBe(true));
    const saveParams = (host.rpc.call as ReturnType<typeof vi.fn>).mock.calls.find(([method]) => method === 'inofy.saveDraft')?.[1] as {
      etag?: string;
      artifact?: ReturnType<typeof blankArtifact>;
    };
    expect(saveParams.etag).toBe('etag-current');
    expect(saveParams.artifact?.definition.graph.nodes[0]?.config).toEqual({ task: 'published revision task' });
  });
});
