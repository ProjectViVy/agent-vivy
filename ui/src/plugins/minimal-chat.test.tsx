// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
let root: Root;
let container: HTMLDivElement;
const screen = {
  getByTestId: (id: string) => container.querySelector(`[data-testid="${id}"]`) as HTMLElement,
  getByRole: (role: string) => container.querySelector(`[role="${role}"]`) as HTMLElement,
  getByLabelText: (label: string) => container.querySelector(label === 'message' ? 'textarea' : 'input') as HTMLInputElement,
};
const fireEvent = {
  click: (element: HTMLElement) => element.click(),
  change: (element: HTMLElement, event: {target: {value: string}}) => act(() => {
    const prototype = element.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
    Object.getOwnPropertyDescriptor(prototype, 'value')!.set!.call(element, event.target.value);
    element.dispatchEvent(new Event('input', {bubbles: true}));
  }),
};
import { afterEach, expect, test, vi } from 'vitest';
import type { FaceStoreState, FullUIHost } from '@vivy/ui-sdk';
import { MinimalChat } from '../../../faces/minimal-chat/ui/src/index';

afterEach(() => { act(() => root?.unmount()); container?.remove(); });

function setup(overrides: Partial<FaceStoreState> = {}) {
  const listeners = new Set<() => void>();
  const cancelCurrentRun = vi.fn(async () => {});
  const startRun = vi.fn(async () => {});
  const createSession = vi.fn(async () => {
    state = { ...state, activeSessionId: 'session-1' };
    return { id: 'session-1', title: '', created_at: 0 };
  });
  let state = {
    initialized: true, sessions: [], messages: [], runEvents: [], runLogs: {},
    currentRun: null, runBusy: false, streamingText: '', activeSessionId: null,
    createSession, startRun, cancelCurrentRun, ...overrides,
  } as unknown as FaceStoreState;
  const host = {
    t: (key: string) => key.slice(key.lastIndexOf('.') + 1),
    store: { getState: () => state, subscribe: (listener: () => void) => {
      listeners.add(listener); return () => listeners.delete(listener);
    } },
  } as unknown as FullUIHost;
  container = document.createElement('div'); document.body.append(container);
  root = createRoot(container);
  act(() => root.render(<MinimalChat host={host} />));
  return { createSession, startRun, cancelCurrentRun, update(patch: Partial<FaceStoreState>) {
    act(() => { state = { ...state, ...patch }; listeners.forEach((listener) => listener()); });
  } };
}

test('admits a message through the host with the selected workspace', async () => {
  const { createSession, startRun } = setup();
  fireEvent.change(screen.getByLabelText('workspace'), { target: { value: '/tmp/project' } });
  fireEvent.change(screen.getByLabelText('message'), { target: { value: 'Read nonce.txt' } });
  await act(async () => fireEvent.click(screen.getByTestId('minimal-send')));
  expect(createSession).toHaveBeenCalledWith(undefined, '/tmp/project');
  expect(startRun).toHaveBeenCalledWith('session-1', { text: 'Read nonce.txt' });
  expect(screen.getByLabelText('message').value).toBe('');
});

test('active Run owns cancellation and prevents another submission', async () => {
  const { cancelCurrentRun } = setup({ currentRun: { id: 'run-1', session_id: 's', status: 'active', created_at: 0 } });
  fireEvent.change(screen.getByLabelText('message'), { target: { value: 'second' } });
  expect((screen.getByTestId('minimal-send') as HTMLButtonElement).disabled).toBe(true);
  await act(async () => fireEvent.click(screen.getByTestId('minimal-cancel')));
  expect(cancelCurrentRun).toHaveBeenCalledOnce();
});

test('the host-created empty session can bind a workspace before its first Run', async () => {
  const chooseWorkspace = vi.fn(async () => ({ id: 'empty', title: '', created_at: 0 }));
  const { startRun } = setup({ activeSessionId: 'empty', sessions: [{ id: 'empty', title: '', created_at: 0 }], chooseWorkspace });
  fireEvent.change(screen.getByLabelText('workspace'), { target: { value: '/tmp/project' } });
  fireEvent.change(screen.getByLabelText('message'), { target: { value: 'Read nonce.txt' } });
  await act(async () => fireEvent.click(screen.getByTestId('minimal-send')));
  expect(chooseWorkspace).toHaveBeenCalledWith('/tmp/project');
  expect(startRun).toHaveBeenCalledWith('empty', { text: 'Read nonce.txt' });
});

test('renders committed tool output, terminal status and backend errors', () => {
  const { update } = setup({ currentRun: { id: 'r', session_id: 's', status: 'completed', created_at: 0 },
    runEvents: [{ run_id: 'r', seq: 1, type: 'tool.finished', created_at: 0, payload_version: 1,
      payload: { tool_call_id: 'read-1', tool_name: 'read_file', result: 'nonce-42' } }] });
  expect(screen.getByTestId('minimal-tool-read-1').textContent).toContain('nonce-42');
  expect(screen.getByTestId('minimal-status').textContent).toContain('completed');
  update({ runError: 'Provider unavailable' });
  expect(screen.getByRole('alert').textContent).toContain('Provider unavailable');
});

test('failed admission retains the draft and reports the cause', async () => {
  const startRun = vi.fn(async () => { throw new Error('admission denied'); });
  setup({ activeSessionId: 's', startRun });
  fireEvent.change(screen.getByLabelText('message'), { target: { value: 'keep me' } });
  await act(async () => fireEvent.click(screen.getByTestId('minimal-send')));
  expect(screen.getByLabelText('message').value).toBe('keep me');
  expect(screen.getByRole('alert').textContent).toContain('admission denied');
});

test('the draft is locked only while admission is pending', async () => {
  let resolve!: () => void;
  const startRun = vi.fn(() => new Promise<void>((done) => { resolve = done; }));
  setup({ activeSessionId: 's', startRun });
  fireEvent.change(screen.getByLabelText('message'), { target: { value: 'submitted draft' } });
  await act(async () => fireEvent.click(screen.getByTestId('minimal-send')));
  expect(screen.getByLabelText('message').disabled).toBe(true);
  await act(async () => resolve());
  expect(screen.getByLabelText('message').disabled).toBe(false);
  expect(screen.getByLabelText('message').value).toBe('');
});

test('a finished tool displays both result and failure, and empty completion is not Running', () => {
  setup({ runEvents: [
    { run_id: 'r', seq: 1, type: 'tool.finished', created_at: 0, payload_version: 1,
      payload: { tool_call_id: 'partial', tool_name: 'read_file', result: 'partial output', error: 'recoverable failure' } },
    { run_id: 'r', seq: 2, type: 'tool.finished', created_at: 0, payload_version: 1,
      payload: { tool_call_id: 'empty', tool_name: 'read_file', result: '' } },
  ] });
  expect(screen.getByTestId('minimal-tool-partial').textContent).toContain('partial output');
  expect(screen.getByTestId('minimal-tool-partial').textContent).toContain('recoverable failure');
  expect(screen.getByTestId('minimal-tool-empty').textContent).toContain('completed');
});
