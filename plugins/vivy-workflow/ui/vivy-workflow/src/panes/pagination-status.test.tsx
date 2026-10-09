// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { FaceClientRPC, FaceClientStore, FaceStoreState, UITranslator } from '@vivy/ui-sdk';
import { WorkflowClient } from '../client';
import { FaceBridge } from '../face-bridge';
import { RunsPane } from './RunsPane';
import { WorkflowsPane } from './WorkflowsPane';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

type Action = (method: string, params?: unknown) => Promise<unknown>;

function makeHarness(action: Action) {
  const state = {
    activeSessionId: 'sess-1',
    currentRun: { id: 'run-parent', session_id: 'sess-1', status: 'active', created_at: 1 },
  } as unknown as FaceStoreState;
  const notifications = new Map<string, (params: unknown) => void>();
  const rpc = {
    capabilities: { protocol_version: 'vivy.rpc.v1', capabilities: [] },
    call: vi.fn(action),
    onNotification: vi.fn((method: string, listener: (params: unknown) => void) => {
      notifications.set(method, listener);
      return () => { notifications.delete(method); };
    }),
    onClose: vi.fn(() => () => undefined),
    close: vi.fn(),
  } as unknown as FaceClientRPC;
  const store = {
    getState: () => state,
    getInitialState: () => state,
    setState: vi.fn(),
    subscribe: vi.fn(() => () => undefined),
  } as unknown as FaceClientStore<FaceStoreState>;
  return { client: new WorkflowClient(new FaceBridge(rpc, store)), rpc, notifications, state };
}

let container: HTMLDivElement;
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

const t = ((key: string, params?: Record<string, unknown>) => `${key}${params?.status ? ` ${params.status}` : ''}`) as UITranslator;

async function clickButton(needle: string, settle?: () => Promise<unknown>) {
  const button = await vi.waitFor(() => {
    const found = [...container.querySelectorAll('button')].find((item) =>
      item.textContent?.includes(needle) || item.getAttribute('title') === needle,
    );
    expect(found).toBeDefined();
    return found!;
  });
  await act(async () => {
    button.click();
    await new Promise((resolve) => setTimeout(resolve, 0));
    await settle?.();
  });
}

async function mountWorkflows(client: WorkflowClient, sessionId = 'sess-a') {
  await act(async () => root.render(
    <WorkflowsPane
      client={client}
      sessionId={sessionId}
      t={t}
      canRun
      onOpenDraft={vi.fn()}
      onOpenRevision={vi.fn()}
      onRunStarted={vi.fn()}
    />,
  ));
  await act(async () => { await vi.waitFor(() => expect(container.textContent).toContain('wf-')); });
}

async function mountRuns(client: WorkflowClient, sessionId = 'sess-a') {
  await act(async () => root.render(
    <RunsPane client={client} sessionId={sessionId} t={t} focusRunId={null} onFocusHandled={vi.fn()} />,
  ));
  await act(async () => { await vi.waitFor(() => expect(container.textContent).toContain('run-')); });
}

function rpcError(message: string): Error {
  const error = new Error(message);
  return error;
}

describe('workflow list pagination', () => {
  it('appends every page and removes the Load more control at an empty terminal cursor', async () => {
    const cursors: Array<unknown> = [];
    const host = makeHarness(async (method, params) => {
      if (method === 'inofy.listWorkflows') {
        const cursor = (params as { cursor?: string }).cursor;
        cursors.push(cursor);
        return cursor === undefined
          ? { items: [{ workflow_id: 'wf-first', revision: 1 }], next_cursor: 'wf-page-2' }
          : { items: [{ workflow_id: 'wf-first', revision: 1 }, { workflow_id: 'wf-second', revision: 2 }], next_cursor: '' };
      }
      if (method === 'inofy.capabilities') return { supports_resume: false };
      if (method === 'inofy.listConnections') return [];
      return {};
    });
    await mountWorkflows(host.client);

    await clickButton('plugin.vivy/workflow-ui.workflows.loadMore', () => vi.waitFor(() => expect(cursors).toHaveLength(2)));
    await vi.waitFor(() => expect(container.textContent).toContain('wf-second'));
    expect(container.textContent).toContain('wf-first');
    expect(container.querySelectorAll('ul li')).toHaveLength(2);
    expect(container.textContent).toContain('"supports_resume": false');
    expect(cursors).toEqual([undefined, 'wf-page-2']);
    expect([...container.querySelectorAll('button')].some((button) => button.textContent?.includes('plugin.vivy/workflow-ui.workflows.loadMore'))).toBe(false);
  });

  it('keeps rows and retries the same cursor when loading another page fails', async () => {
    const cursors: Array<unknown> = [];
    let continuationAttempts = 0;
    const host = makeHarness(async (method, params) => {
      if (method === 'inofy.listWorkflows') {
        const cursor = (params as { cursor?: string }).cursor;
        cursors.push(cursor);
        if (cursor === undefined) return { items: [{ workflow_id: 'wf-retained', revision: 1 }], next_cursor: 'wf-retry-cursor' };
        continuationAttempts += 1;
        if (continuationAttempts === 1) throw rpcError('temporary list failure');
        return { items: [{ workflow_id: 'wf-next', revision: 1 }], next_cursor: null };
      }
      if (method === 'inofy.capabilities') return {};
      if (method === 'inofy.listConnections') return [];
      return {};
    });
    await mountWorkflows(host.client);

    await clickButton('plugin.vivy/workflow-ui.workflows.loadMore');
    await vi.waitFor(() => expect(container.querySelector('[role="alert"]')?.textContent).toContain('temporary list failure'));
    expect(container.textContent).toContain('wf-retained');
    await clickButton('plugin.vivy/workflow-ui.workflows.loadMore', () => vi.waitFor(() => expect(cursors).toHaveLength(3)));
    await vi.waitFor(() => expect(container.textContent).toContain('wf-next'));
    expect(cursors).toEqual([undefined, 'wf-retry-cursor', 'wf-retry-cursor']);
    expect(container.textContent).toContain('wf-retained');
  });

  it('replaces rows on refresh and ignores a late page from the older query', async () => {
    let resolveOldPage!: (value: unknown) => void;
    let firstPageCount = 0;
    const cursors: Array<unknown> = [];
    const host = makeHarness(async (method, params) => {
      if (method === 'inofy.listWorkflows') {
        const cursor = (params as { cursor?: string }).cursor;
        cursors.push(cursor);
        if (cursor === 'old-cursor') return new Promise((resolve) => { resolveOldPage = resolve; });
        firstPageCount += 1;
        return firstPageCount === 1
          ? { items: [{ workflow_id: 'wf-old', revision: 1 }], next_cursor: 'old-cursor' }
          : { items: [{ workflow_id: 'wf-fresh', revision: 3 }], next_cursor: 'fresh-cursor' };
      }
      if (method === 'inofy.capabilities') return {};
      if (method === 'inofy.listConnections') return [];
      return {};
    });
    await mountWorkflows(host.client);

    await clickButton('plugin.vivy/workflow-ui.workflows.loadMore', () => vi.waitFor(() => expect(resolveOldPage).toBeTypeOf('function')));
    const workflowLoadMore = [...container.querySelectorAll('button')].find((button) => button.textContent?.includes('common.loading'));
    expect(workflowLoadMore?.disabled).toBe(true);
    expect(cursors.filter((cursor) => cursor === 'old-cursor')).toHaveLength(1);
    await clickButton('common.refresh', () => vi.waitFor(() => expect(firstPageCount).toBe(2)));
    await vi.waitFor(() => expect(container.textContent).toContain('wf-fresh'));
    await act(async () => { resolveOldPage({ items: [{ workflow_id: 'wf-late', revision: 2 }], next_cursor: null }); });
    await vi.waitFor(() => expect(container.textContent).not.toContain('wf-late'));
    expect(container.textContent).toContain('wf-fresh');
    expect(container.textContent).not.toContain('wf-old');
    await clickButton('plugin.vivy/workflow-ui.workflows.loadMore', () => vi.waitFor(() => expect(cursors.at(-1)).toBe('fresh-cursor')));
    expect(cursors).toEqual([undefined, 'old-cursor', undefined, 'fresh-cursor']);
  });
});

describe('run list pagination and lifecycle', () => {
  it('appends Runs pages and stops after an empty terminal cursor', async () => {
    const cursors: Array<unknown> = [];
    const host = makeHarness(async (method, params) => {
      if (method === 'inofy.listRuns') {
        const cursor = (params as { cursor?: string }).cursor;
        cursors.push(cursor);
        return cursor === undefined
          ? { items: [{ run_id: 'run-first', status: 'completed', engine_status: 'succeeded' }], next_cursor: 'run-page-2' }
          : { items: [{ run_id: 'run-first', status: 'completed', engine_status: 'succeeded' }, { run_id: 'run-second', status: 'active', engine_status: 'admitted' }], next_cursor: '' };
      }
      return {};
    });
    await mountRuns(host.client);

    await clickButton('plugin.vivy/workflow-ui.runs.loadMore', () => vi.waitFor(() => expect(cursors).toHaveLength(2)));
    await vi.waitFor(() => expect(container.textContent).toContain('run-second'));
    expect(container.textContent).toContain('run-first');
    expect(container.querySelectorAll('ul li')).toHaveLength(2);
    expect(cursors).toEqual([undefined, 'run-page-2']);
    expect([...container.querySelectorAll('button')].some((button) => button.textContent?.includes('plugin.vivy/workflow-ui.runs.loadMore'))).toBe(false);
  });

  it('preserves run rows and retries the same cursor after continuation failure', async () => {
    const cursors: Array<unknown> = [];
    let attempts = 0;
    const host = makeHarness(async (method, params) => {
      if (method === 'inofy.listRuns') {
        const cursor = (params as { cursor?: string }).cursor;
        cursors.push(cursor);
        if (cursor === undefined) return { items: [{ run_id: 'run-retained', status: 'active' }], next_cursor: 'run-retry-cursor' };
        attempts += 1;
        if (attempts === 1) throw rpcError('temporary run list failure');
        return { items: [{ run_id: 'run-next', status: 'completed' }], next_cursor: null };
      }
      return {};
    });
    await mountRuns(host.client);

    await clickButton('plugin.vivy/workflow-ui.runs.loadMore');
    await vi.waitFor(() => expect(container.querySelector('[role="alert"]')?.textContent).toContain('temporary run list failure'));
    expect(container.textContent).toContain('run-retained');
    await clickButton('plugin.vivy/workflow-ui.runs.loadMore', () => vi.waitFor(() => expect(cursors).toHaveLength(3)));
    await vi.waitFor(() => expect(container.textContent).toContain('run-next'));
    expect(cursors).toEqual([undefined, 'run-retry-cursor', 'run-retry-cursor']);
  });

  it('refreshes the list for a new session and ignores its prior page response', async () => {
    let resolveOldPage!: (value: unknown) => void;
    const cursors: Array<string | undefined> = [];
    const host = makeHarness(async (method, params) => {
      if (method === 'inofy.listRuns') {
        const cursor = (params as { cursor?: string }).cursor;
        cursors.push(cursor);
        if (cursor === 'old-session-cursor') return new Promise((resolve) => { resolveOldPage = resolve; });
        if (cursors.length === 1) return { items: [{ run_id: 'run-old-session', status: 'active' }], next_cursor: 'old-session-cursor' };
        return { items: [{ run_id: 'run-new-session', status: 'completed' }], next_cursor: null };
      }
      return {};
    });
    await mountRuns(host.client, 'session-a');

    await clickButton('plugin.vivy/workflow-ui.runs.loadMore', () => vi.waitFor(() => expect(resolveOldPage).toBeTypeOf('function')));
    const runsLoadMore = [...container.querySelectorAll('button')].find((button) => button.textContent?.includes('common.loading'));
    expect(runsLoadMore?.disabled).toBe(true);
    expect(cursors.filter((cursor) => cursor === 'old-session-cursor')).toHaveLength(1);
    await act(async () => root.render(
      <RunsPane client={host.client} sessionId="session-b" t={t} focusRunId={null} onFocusHandled={vi.fn()} />,
    ));
    await vi.waitFor(() => expect(container.textContent).toContain('run-new-session'));
    await act(async () => { resolveOldPage({ items: [{ run_id: 'run-late-session', status: 'active' }], next_cursor: null }); });
    expect(container.textContent).toContain('run-new-session');
    expect(container.textContent).not.toContain('run-late-session');
    expect(cursors).toEqual([undefined, 'old-session-cursor', undefined]);
  });

  it('shows completed native status and disables cancellation', async () => {
    const host = makeHarness(async (method, params) => {
      if (method === 'inofy.listRuns') return { items: [{ run_id: 'run-completed', status: 'completed', engine_status: 'succeeded' }], next_cursor: null };
      if (method === 'inofy.getRun') return { run_id: 'run-completed', status: 'completed', engine_status: 'succeeded' };
      if (method === 'inofy.events') return { events: [], next_cursor: null };
      if (method === 'run/subscribe') return { subscription_id: 'sub-completed' };
      return {};
    });
    await mountRuns(host.client);
    await clickButton('run-completed');
    await vi.waitFor(() => expect(container.textContent).toContain('plugin.vivy/workflow-ui.runs.engineStatus succeeded'));
    const cancel = [...container.querySelectorAll('button')].find((button) => button.textContent?.includes('plugin.vivy/workflow-ui.runs.cancel'));
    expect(cancel?.disabled).toBe(true);
    expect(container.textContent).toContain('completed');
    expect(container.textContent).toContain('succeeded');
  });

  it('refreshes detail on recovery-required events and shows guidance without Resume', async () => {
    let detailReads = 0;
    const host = makeHarness(async (method, params) => {
      if (method === 'inofy.listRuns') return { items: [{ run_id: 'run-recovery', status: 'active', engine_status: 'admitted' }], next_cursor: null };
      if (method === 'inofy.getRun') {
        detailReads += 1;
        return { run_id: 'run-recovery', status: 'active', engine_status: detailReads === 1 ? 'admitted' : 'recovery_required' };
      }
      if (method === 'inofy.events') return { events: [], next_cursor: null };
      if (method === 'run/subscribe') return { subscription_id: 'sub-recovery' };
      return {};
    });
    await mountRuns(host.client);
    await clickButton('run-recovery');
    await vi.waitFor(() => expect(host.notifications.has('run/event')).toBe(true));
    await vi.waitFor(() => expect(detailReads).toBe(1));
    const notify = host.notifications.get('run/event');
    await act(async () => notify?.({
      subscription_id: 'sub-recovery',
      event: { seq: 3, type: 'workflow.recovery_required', data: {} },
    }));
    await vi.waitFor(() => expect(detailReads).toBe(2));
    await vi.waitFor(() => expect(container.textContent).toContain('plugin.vivy/workflow-ui.runs.recoveryRequired'));
    expect(container.textContent).toContain('recovery_required');
    expect(container.textContent).toContain('active');
    expect([...container.querySelectorAll('button')].some((button) => button.textContent?.toLowerCase().includes('resume'))).toBe(false);
  });
});
