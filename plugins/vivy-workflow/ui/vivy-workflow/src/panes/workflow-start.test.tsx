// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { FaceClientRPC, FaceClientStore, FaceStoreState, UITranslator } from '@vivy/ui-sdk';
import { WorkflowClient } from '../client';
import { FaceBridge } from '../face-bridge';
import { WorkflowsPane } from './WorkflowsPane';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

function makeHarness(action: (method: string, params?: unknown) => Promise<unknown>) {
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
  const store = {
    getState: () => state,
    getInitialState: () => state,
    setState: vi.fn(),
    subscribe: vi.fn(() => () => undefined),
  } as unknown as FaceClientStore<FaceStoreState>;
  return { client: new WorkflowClient(new FaceBridge(rpc, store)), rpc, state };
}

function rpcError(code: string): Error {
  const error = new Error(code) as Error & { data?: { code: string; message: string } };
  error.data = { code, message: code };
  return error;
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

const t = ((key: string) => key) as UITranslator;

async function clickButton(needle: string, settle?: () => Promise<unknown>) {
  const button = [...container.querySelectorAll('button')].find((item) => item.textContent?.includes(needle));
  if (!button) throw new Error(`button not found: ${needle}`);
  await act(async () => {
    button.click();
    await new Promise((resolve) => setTimeout(resolve, 0));
    await settle?.();
  });
}

async function mount(client: WorkflowClient) {
  await act(async () => root.render(
    <WorkflowsPane
      client={client}
      sessionId="sess-a"
      t={t}
      canRun
      onOpenDraft={vi.fn()}
      onOpenRevision={vi.fn()}
      onRunStarted={vi.fn()}
    />,
  ));
  await act(async () => {
    await vi.waitFor(() => expect(container.textContent).toContain('wf-published'));
  });
}

describe('WorkflowsPane start intent', () => {
  it('retries published revision 2 with the identical captured request after a lost reply', async () => {
    let rejectFirst!: (error: Error) => void;
    const firstStart = new Promise<unknown>((_resolve, reject) => { rejectFirst = reject; });
    const startCalls: unknown[] = [];
    const host = makeHarness(async (method, params) => {
      if (method === 'inofy.listWorkflows') return { items: [{ workflow_id: 'wf-published', revision: 2 }], next_cursor: null };
      if (method === 'inofy.capabilities') return {};
      if (method === 'inofy.listConnections') return [];
      if (method === 'inofy.startRun') {
        startCalls.push(params);
        if (startCalls.length === 1) return firstStart;
        return { run_id: 'run-published' };
      }
      return {};
    });
    await mount(host.client);

    await clickButton('plugin.vivy/workflow-ui.editor.run', () => vi.waitFor(() => expect(startCalls).toHaveLength(1)));
    await act(async () => {
      rejectFirst(new Error('socket closed after admission'));
      await new Promise((resolve) => setTimeout(resolve, 0));
      Object.assign(host.state, { currentRun: { id: 'run-other', session_id: 'sess-1', status: 'active', created_at: 2 } });
    });
    await vi.waitFor(() => expect(container.querySelector('[role="alert"]')).not.toBeNull());
    await clickButton('plugin.vivy/workflow-ui.editor.retryStart', () => vi.waitFor(() => expect(startCalls).toHaveLength(2)));
    expect(startCalls[1]).toEqual(startCalls[0]);
    expect(startCalls[0]).toMatchObject({
      workflow: 'wf-published', revision: 2, session_id: 'sess-1', parent_run_id: 'run-parent',
      operation_id: expect.stringMatching(/^[0-9a-f-]{36}$/),
    });
  });

  it('keeps the same published request after source conflict and shows recovery guidance', async () => {
    const startCalls: unknown[] = [];
    const host = makeHarness(async (method, params) => {
      if (method === 'inofy.listWorkflows') return { items: [{ workflow_id: 'wf-published', revision: 2 }], next_cursor: null };
      if (method === 'inofy.capabilities') return {};
      if (method === 'inofy.listConnections') return [];
      if (method === 'inofy.startRun') {
        startCalls.push(params);
        throw rpcError('revision_conflict');
      }
      return {};
    });
    await mount(host.client);

    await clickButton('plugin.vivy/workflow-ui.editor.run', () => vi.waitFor(() => expect(startCalls).toHaveLength(1)));
    await vi.waitFor(() => expect(container.querySelector('[role="alert"]')?.textContent).toContain('plugin.vivy/workflow-ui.editor.startSourceConflict'));
    await clickButton('plugin.vivy/workflow-ui.editor.retryStart', () => vi.waitFor(() => expect(startCalls).toHaveLength(2)));
    expect(startCalls[1]).toEqual(startCalls[0]);
    expect(startCalls[0]).toMatchObject({ workflow: 'wf-published', revision: 2, parent_run_id: 'run-parent' });
  });
});
