// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { FaceClientRPC, FaceClientStore, FaceStoreState, UITranslator } from '@vivy/ui-sdk';
import { WorkflowClient } from '../client';
import { FaceBridge } from '../face-bridge';
import { blankArtifact } from '../seed';
import type { NodeDescriptor } from '../studio/schema';
import { EditorPane } from './EditorPane';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

type Action = (method: string, params?: unknown) => Promise<unknown>;

function makeHarness(action: Action) {
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

function rpcError(code: string, message = code): Error {
  const error = new Error(message) as Error & { data?: { code: string; message: string } };
  error.data = { code, message };
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

async function mountEditor(client: WorkflowClient, catalog: NodeDescriptor[] = []) {
  await act(async () => root.render(
    <EditorPane
      client={client}
      catalog={catalog}
      t={t}
      canRun
      target={{ workflow: 'wf-editor', key: 1 }}
      onRunStarted={vi.fn()}
    />,
  ));
  await act(async () => {
    await vi.waitFor(() => expect(container.textContent).not.toContain('plugin.vivy/workflow-ui.editor.empty'));
  });
}

describe('EditorPane start intent', () => {
  it('retries a lost start reply with the saved snapshot and does not save again', async () => {
    let rejectFirst!: (error: Error) => void;
    const firstStart = new Promise<unknown>((_resolve, reject) => { rejectFirst = reject; });
    const startCalls: unknown[] = [];
    const saveCalls: unknown[] = [];
    const artifact = blankArtifact('wf-editor');
    const host = makeHarness(async (method, params) => {
      if (method === 'inofy.loadDraft') return { workflow: 'wf-editor', etag: 'e1', artifact };
      if (method === 'inofy.saveDraft') {
        saveCalls.push(params);
        return { workflow: 'wf-editor', etag: 'e2', artifact: (params as { artifact: typeof artifact }).artifact };
      }
      if (method === 'inofy.startRun') {
        startCalls.push(params);
        if (startCalls.length === 1) return firstStart;
        return { run_id: 'run-child' };
      }
      return {};
    });
    await mountEditor(host.client);

    await clickButton('plugin.vivy/workflow-ui.editor.run', () => vi.waitFor(() => expect(startCalls).toHaveLength(1)));
    expect(container.textContent).not.toContain('plugin.vivy/workflow-ui.editor.dirty');
    await act(async () => {
      rejectFirst(new Error('socket closed after admission'));
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    await vi.waitFor(() => expect(container.querySelector('[role="alert"]')).not.toBeNull());
    await clickButton('plugin.vivy/workflow-ui.editor.retryStart', () => vi.waitFor(() => expect(startCalls).toHaveLength(2)));
    expect(saveCalls).toHaveLength(1);
    expect(startCalls[1]).toEqual(startCalls[0]);
    expect(startCalls[0]).toMatchObject({ session_id: 'sess-1', parent_run_id: 'run-parent', draft_etag: 'e2' });
  });

  it('keeps save confirmation after validate and publish failures for the next edit', async () => {
    let saveIndex = 0;
    const saveCalls: Array<Record<string, unknown>> = [];
    const followupCalls: Array<[string, Record<string, unknown>]> = [];
    const host = makeHarness(async (method, params) => {
      if (method === 'inofy.loadDraft') return { workflow: 'wf-editor', etag: 'e1', artifact: blankArtifact('wf-editor') };
      if (method === 'inofy.saveDraft') {
        const body = params as Record<string, unknown>;
        saveCalls.push(body);
        saveIndex += 1;
        return { workflow: 'wf-editor', etag: `e${saveIndex + 1}`, artifact: body.artifact };
      }
      if (method === 'inofy.validate' || method === 'inofy.publish') {
        followupCalls.push([method, params as Record<string, unknown>]);
        throw rpcError('validation_failed', method === 'inofy.validate' ? 'invalid workflow' : 'publish rejected');
      }
      return {};
    });
    await mountEditor(host.client);

    await clickButton('plugin.vivy/workflow-ui.editor.validate');
    await vi.waitFor(() => expect(container.querySelector('[role="alert"]')?.textContent).toContain('invalid workflow'));
    expect(followupCalls[0]).toEqual(['inofy.validate', expect.objectContaining({ etag: 'e2' })]);
    await clickButton('plugin.vivy/workflow-ui.editor.save', () => vi.waitFor(() => expect(saveCalls).toHaveLength(2)));
    expect(saveCalls[1]).toMatchObject({ etag: 'e2' });

    await clickButton('plugin.vivy/workflow-ui.editor.publish');
    await vi.waitFor(() => expect(container.querySelector('[role="alert"]')?.textContent).toContain('publish rejected'));
    expect(followupCalls[1]).toEqual(['inofy.publish', expect.objectContaining({ etag: 'e4' })]);
    await clickButton('plugin.vivy/workflow-ui.editor.save', () => vi.waitFor(() => expect(saveCalls).toHaveLength(4)));
    expect(saveCalls[3]).toMatchObject({ etag: 'e4' });
  });

  it('prepares a new start intent after an edit following a failed start', async () => {
    const startCalls: Array<Record<string, unknown>> = [];
    const saveCalls: Array<Record<string, unknown>> = [];
    const host = makeHarness(async (method, params) => {
      if (method === 'inofy.loadDraft') return { workflow: 'wf-editor', etag: 'e1', artifact: blankArtifact('wf-editor') };
      if (method === 'inofy.saveDraft') {
        const body = params as Record<string, unknown>;
        saveCalls.push(body);
        return { workflow: 'wf-editor', etag: `e${saveCalls.length + 1}`, artifact: body.artifact };
      }
      if (method === 'inofy.startRun') {
        startCalls.push(params as Record<string, unknown>);
        if (startCalls.length === 1) throw new Error('reply lost');
        return { run_id: 'run-child' };
      }
      return {};
    });
    const catalog: NodeDescriptor[] = [{ type_id: 'test.task', implementation_id: 'test.task' }];
    await mountEditor(host.client, catalog);

    await clickButton('plugin.vivy/workflow-ui.editor.run', () => vi.waitFor(() => expect(startCalls).toHaveLength(1)));
    await vi.waitFor(() => expect(container.querySelector('[role="alert"]')).not.toBeNull());
    await clickButton('test.task');
    await clickButton('plugin.vivy/workflow-ui.editor.run', () => vi.waitFor(() => expect(startCalls).toHaveLength(2)));
    expect(saveCalls).toHaveLength(2);
    expect(saveCalls[1]).toMatchObject({ etag: 'e2' });
    expect(startCalls[1]?.operation_id).not.toBe(startCalls[0]?.operation_id);
  });

  it('retains an ambiguous intent after a source conflict and retries without saving or preparing again', async () => {
    const startCalls: Array<Record<string, unknown>> = [];
    const saveCalls: unknown[] = [];
    const host = makeHarness(async (method, params) => {
      if (method === 'inofy.loadDraft') return { workflow: 'wf-editor', etag: 'e1', artifact: blankArtifact('wf-editor') };
      if (method === 'inofy.saveDraft') {
        saveCalls.push(params);
        return { workflow: 'wf-editor', etag: 'e2', artifact: (params as { artifact: unknown }).artifact };
      }
      if (method === 'inofy.startRun') {
        startCalls.push(params as Record<string, unknown>);
        if (startCalls.length === 1) throw new Error('reply lost');
        throw rpcError('revision_conflict', 'source changed');
      }
      return {};
    });
    await mountEditor(host.client);

    await clickButton('plugin.vivy/workflow-ui.editor.run', () => vi.waitFor(() => expect(startCalls).toHaveLength(1)));
    await vi.waitFor(() => expect(container.querySelector('[role="alert"]')).not.toBeNull());
    await clickButton('plugin.vivy/workflow-ui.editor.retryStart', () => vi.waitFor(() => expect(startCalls).toHaveLength(2)));
    expect(container.querySelector('[role="alert"]')?.textContent).toContain('plugin.vivy/workflow-ui.editor.startSourceConflict');
    expect(saveCalls).toHaveLength(1);
    expect(startCalls).toHaveLength(2);
    expect(startCalls[1]).toEqual(startCalls[0]);
  });
});
