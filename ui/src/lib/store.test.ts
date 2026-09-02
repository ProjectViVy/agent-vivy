import { beforeEach, describe, expect, it, vi } from 'vitest';

const api = vi.hoisted(() => ({
  initialize: vi.fn(), recoverBackgroundRuns: vi.fn(), listSessions: vi.fn(), listBackgroundRuns: vi.fn(), getSettings: vi.fn(), listMessages: vi.fn(), listTodos: vi.fn(), getRun: vi.fn(), getRunLog: vi.fn(), listChildren: vi.fn(), listReviews: vi.fn(),
  createSession: vi.fn(), renameSession: vi.fn(), deleteSession: vi.fn(), startTurn: vi.fn(), cancelRun: vi.fn(), attachBackgroundRun: vi.fn(), startChild: vi.fn(), getChild: vi.fn(), waitChild: vi.fn(), cancelChild: vi.fn(), respondReview: vi.fn(), updateSettings: vi.fn(), inspectSpecies: vi.fn(), listGenerations: vi.fn(), listEvals: vi.fn(), listPromotions: vi.fn(), createGeneration: vi.fn(), rejectGeneration: vi.fn(), startEval: vi.fn(), recordEval: vi.fn(), promoteGeneration: vi.fn(),
  listProviders: vi.fn(), upsertProvider: vi.fn(), deleteProvider: vi.fn(),
}));
const subscription = vi.hoisted(() => ({ onEvent: undefined as undefined | ((event: { run_id: string; seq: number; type: string; created_at: number; payload_version: number; payload: Record<string, unknown> }) => void) }));
vi.mock('./api', () => api);
vi.mock('./rpc', () => ({ resetRpcClient: vi.fn() }));
vi.mock('./run-subscription', () => ({ subscribeRun: vi.fn((_id: string, _seq: number, onEvent: typeof subscription.onEvent) => { subscription.onEvent = onEvent; return { close: vi.fn(), lastSeq: () => 0 }; }) }));

import { resetStoreForTests, useVivyStore } from './store';

function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>((done) => { resolve = done; }); return { promise, resolve }; }

describe('Vivy store integrity', () => {
  beforeEach(() => {
    const values = new Map<string, string>();
    vi.stubGlobal('localStorage', { getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => values.set(key, value), removeItem: (key: string) => values.delete(key) });
    resetStoreForTests();
    useVivyStore.setState(useVivyStore.getInitialState(), true);
    vi.clearAllMocks();
    subscription.onEvent = undefined;
    api.initialize.mockResolvedValue({ protocol_version: 'vivy.rpc.v1', capabilities: ['session', 'run.subscribe'] });
    api.recoverBackgroundRuns.mockResolvedValue({ recovered: true });
    api.listBackgroundRuns.mockResolvedValue({ runs: [] });
    api.getSettings.mockResolvedValue({ provider: 'openai', default_model: 'gpt-4o-mini', base_url: '', execute_max_timeout_seconds: 0, read_only: false, config_provider: '', config_model: '', config_execute_max_timeout_seconds: 30 });
    api.listProviders.mockResolvedValue({ entries: [], active_provider: '', active_model: '', active_base_url: '', read_only: false, config_provider: '', config_model: '' });
    api.listReviews.mockResolvedValue({ reviews: [] });
    api.listTodos.mockResolvedValue({ todos: [] });
  });

  it('initializes one authoritative active session and restores its messages', async () => {
    api.listSessions.mockResolvedValue({ sessions: [{ id: 's1', title: 'One', created_at: 1 }] });
    api.listMessages.mockResolvedValue({ messages: [{ id: 'm1', role: 'user', content: 'hello', created_at: 2 }] });
    await useVivyStore.getState().initialize();
    expect(useVivyStore.getState()).toMatchObject({ initialized: true, activeSessionId: 's1', messagesPhase: 'ready' });
    expect(useVivyStore.getState().messages[0].content).toBe('hello');
  });

  it('creates and selects the first session during initialization when none exist', async () => {
    api.listSessions.mockResolvedValue({ sessions: [] });
    api.createSession.mockResolvedValue({ id: 's-new', title: '', created_at: 1 });
    api.listMessages.mockResolvedValue({ messages: [] });
    await useVivyStore.getState().initialize();
    expect(api.createSession).toHaveBeenCalledTimes(1);
    expect(api.createSession).toHaveBeenCalledWith('');
    expect(useVivyStore.getState()).toMatchObject({ initialized: true, activeSessionId: 's-new', sessionsPhase: 'ready', messagesPhase: 'empty' });
  });

  it('does not let a late session response overwrite the newly selected session', async () => {
    const first = deferred<{ messages: Array<{ id: string; role: 'user'; content: string; created_at: number }> }>();
    const second = deferred<{ messages: Array<{ id: string; role: 'user'; content: string; created_at: number }> }>();
    api.listMessages.mockImplementation((id: string) => id === 's1' ? first.promise : second.promise);
    const p1 = useVivyStore.getState().selectSession('s1');
    const p2 = useVivyStore.getState().selectSession('s2');
    second.resolve({ messages: [{ id: 'm2', role: 'user', content: 'new', created_at: 2 }] });
    await p2;
    first.resolve({ messages: [{ id: 'm1', role: 'user', content: 'stale', created_at: 1 }] });
    await p1;
    expect(useVivyStore.getState().activeSessionId).toBe('s2');
    expect(useVivyStore.getState().messages.map((item) => item.content)).toEqual(['new']);
  });

  it('retries initialization after a control-plane failure without a full page reload', async () => {
    api.initialize.mockRejectedValueOnce(new Error('Failed to fetch'));
    await useVivyStore.getState().initialize();
    expect(useVivyStore.getState()).toMatchObject({ initialized: true, connection: 'error' });
    expect(useVivyStore.getState().initializationError).toContain('Failed to fetch');
    api.listSessions.mockResolvedValue({ sessions: [{ id: 's1', title: 'One', created_at: 1 }] });
    api.listMessages.mockResolvedValue({ messages: [] });
    await useVivyStore.getState().retryInitialize();
    expect(useVivyStore.getState()).toMatchObject({ initialized: true, connection: 'connected', activeSessionId: 's1', initializationError: null });
  });

  it('keeps the run.failed payload as the conversation error', async () => {
    api.listMessages.mockResolvedValue({ messages: [] });
    api.getRun.mockResolvedValue({ id: 'r1', session_id: 's1', status: 'active', created_at: 1 });
    api.getRunLog.mockResolvedValue({ events: [] });
    api.listChildren.mockResolvedValue({ children: [] });
    await useVivyStore.getState().selectSession('s1');
    await useVivyStore.getState().openRun('r1', 's1');
    subscription.onEvent?.({
      run_id: 'r1',
      seq: 1,
      type: 'run.failed',
      created_at: 2,
      payload_version: 1,
      payload: { cause_category: 'provider_error', message: 'provider openai: API key missing' },
    });
    expect(useVivyStore.getState().currentRun?.status).toBe('failed');
    expect(useVivyStore.getState().runError).toBe('无法连接！请检查供应商配置！');
  });

  it('restores a historical run.failed message when reopening the run', async () => {
    api.listMessages.mockResolvedValue({ messages: [{ id: 'm1', run_id: 'r1', role: 'user', content: 'hi', created_at: 1 }] });
    api.getRun.mockResolvedValue({ id: 'r1', session_id: 's1', status: 'failed', created_at: 1 });
    api.getRunLog.mockResolvedValue({
      events: [{ run_id: 'r1', seq: 1, type: 'run.failed', created_at: 2, payload_version: 1, payload: { cause_category: 'internal_error', message: 'The model run could not be completed. Please try again.' } }],
    });
    api.listChildren.mockResolvedValue({ children: [] });
    await useVivyStore.getState().selectSession('s1');
    expect(useVivyStore.getState().runError).toBe('The model run could not be completed. Please try again.');
  });

  it('keeps streamed output visible and reports an error when terminal message refresh fails', async () => {
    useVivyStore.setState({ activeSessionId: 's1', streamingText: 'complete answer', streamingReasoning: 'reasoning' });
    api.getRun.mockResolvedValue({ id: 'r1', session_id: 's1', status: 'active', created_at: 1 });
    api.getRunLog.mockResolvedValue({ events: [] });
    api.listChildren.mockResolvedValue({ children: [] });
    api.listMessages.mockRejectedValue(new Error('temporary read failure'));
    await useVivyStore.getState().openRun('r1', 's1');
    useVivyStore.setState({ streamingText: 'complete answer', streamingReasoning: 'reasoning' });
    subscription.onEvent?.({ run_id: 'r1', seq: 1, type: 'run.completed', created_at: 2, payload_version: 1, payload: {} });
    await vi.waitFor(() => expect(useVivyStore.getState().runError).toContain('消息刷新失败'));
    expect(useVivyStore.getState().streamingText).toBe('complete answer');
    expect(useVivyStore.getState().streamingReasoning).toBe('reasoning');
  });

  it('loads session todos with the selected session and drops a stale response', async () => {
    const first = deferred<{ todos: Array<{ id: string; session_id: string; subject: string; description: string; status: 'pending'; blocks: string[]; blocked_by: string[]; position: number; created_at: number; updated_at: number }> }>();
    const second = deferred<{ todos: Array<{ id: string; session_id: string; subject: string; description: string; status: 'in_progress'; blocks: string[]; blocked_by: string[]; position: number; created_at: number; updated_at: number }> }>();
    api.listMessages.mockResolvedValue({ messages: [] });
    api.listTodos.mockImplementation((id: string) => id === 's1' ? first.promise : second.promise);
    const p1 = useVivyStore.getState().selectSession('s1');
    const p2 = useVivyStore.getState().selectSession('s2');
    second.resolve({ todos: [{ id: '2', session_id: 's2', subject: 'current', description: 'now', status: 'in_progress', blocks: [], blocked_by: [], position: 0, created_at: 2, updated_at: 2 }] });
    await p2;
    first.resolve({ todos: [{ id: '1', session_id: 's1', subject: 'stale', description: 'old', status: 'pending', blocks: [], blocked_by: [], position: 0, created_at: 1, updated_at: 1 }] });
    await p1;
    expect(useVivyStore.getState().activeSessionId).toBe('s2');
    expect(useVivyStore.getState().todos.map((item) => item.subject)).toEqual(['current']);
    expect(useVivyStore.getState().todosPhase).toBe('ready');
  });

  it('refreshes todos when a task tool finishes', async () => {
    api.listMessages.mockResolvedValue({ messages: [] });
    api.getRun.mockResolvedValue({ id: 'r1', session_id: 's1', status: 'active', created_at: 1 });
    api.getRunLog.mockResolvedValue({ events: [] });
    api.listChildren.mockResolvedValue({ children: [] });
    api.listTodos
      .mockResolvedValueOnce({ todos: [] })
      .mockResolvedValueOnce({
        todos: [{ id: '1', session_id: 's1', subject: 'wire rpc', description: 'list', status: 'in_progress', blocks: [], blocked_by: [], position: 0, created_at: 1, updated_at: 1 }],
      });
    await useVivyStore.getState().selectSession('s1');
    await useVivyStore.getState().openRun('r1', 's1');
    subscription.onEvent?.({ run_id: 'r1', seq: 1, type: 'tool.finished', created_at: 2, payload_version: 1, payload: { tool_name: 'task_create', tool_call_id: 'c1', result: '{}' } });
    await vi.waitFor(() => expect(useVivyStore.getState().todos.map((item) => item.subject)).toEqual(['wire rpc']));
  });

  it('queues a message instead of dropping it while a run is active', async () => {
    api.listMessages.mockResolvedValue({ messages: [] });
    api.getRun.mockResolvedValue({ id: 'r1', session_id: 's1', status: 'active', created_at: 1 });
    api.getRunLog.mockResolvedValue({ events: [] });
    api.listChildren.mockResolvedValue({ children: [] });
    await useVivyStore.getState().selectSession('s1');
    await useVivyStore.getState().openRun('r1', 's1');
    await useVivyStore.getState().startRun('s1', 'second message');
    expect(api.startTurn).not.toHaveBeenCalled();
    expect(useVivyStore.getState().queuedMessages).toHaveLength(1);
    expect(useVivyStore.getState().queuedMessages[0]).toMatchObject({ text: 'second message', mode: 'normal' });
  });

  it('dispatches the queued message after the run completes', async () => {
    api.listMessages.mockResolvedValue({ messages: [] });
    api.getRun.mockResolvedValue({ id: 'r1', session_id: 's1', status: 'active', created_at: 1 });
    api.getRunLog.mockResolvedValue({ events: [] });
    api.listChildren.mockResolvedValue({ children: [] });
    api.startTurn.mockResolvedValue({ run_id: 'r2', status: 'active' });
    await useVivyStore.getState().selectSession('s1');
    await useVivyStore.getState().openRun('r1', 's1');
    await useVivyStore.getState().startRun('s1', 'second message');
    subscription.onEvent?.({ run_id: 'r1', seq: 1, type: 'run.completed', created_at: 2, payload_version: 1, payload: {} });
    await vi.waitFor(() => expect(api.startTurn).toHaveBeenCalledWith('s1', 'second message', 'normal', undefined, undefined, undefined));
    expect(useVivyStore.getState().queuedMessages).toEqual([]);
  });

  it('queues attachments with the message and dispatches them on completion', async () => {
    api.listMessages.mockResolvedValue({ messages: [] });
    api.getRun.mockResolvedValue({ id: 'r1', session_id: 's1', status: 'active', created_at: 1 });
    api.getRunLog.mockResolvedValue({ events: [] });
    api.listChildren.mockResolvedValue({ children: [] });
    api.startTurn.mockResolvedValue({ run_id: 'r2', status: 'active' });
    await useVivyStore.getState().selectSession('s1');
    await useVivyStore.getState().openRun('r1', 's1');
    const attachments = [{ name: 'dot.png', mime_type: 'image/png', data: 'aGVsbG8=' }];
    await useVivyStore.getState().startRun('s1', 'look at this', 'normal', undefined, attachments);
    expect(useVivyStore.getState().queuedMessages[0]).toMatchObject({ text: 'look at this', attachments });
    subscription.onEvent?.({ run_id: 'r1', seq: 1, type: 'run.completed', created_at: 2, payload_version: 1, payload: {} });
    await vi.waitFor(() => expect(api.startTurn).toHaveBeenCalledWith('s1', 'look at this', 'normal', undefined, attachments, undefined));
    expect(useVivyStore.getState().queuedMessages).toEqual([]);
  });

  it('carries the thinking preference through the queue', async () => {
    api.listMessages.mockResolvedValue({ messages: [] });
    api.getRun.mockResolvedValue({ id: 'r1', session_id: 's1', status: 'active', created_at: 1 });
    api.getRunLog.mockResolvedValue({ events: [] });
    api.listChildren.mockResolvedValue({ children: [] });
    api.startTurn.mockResolvedValue({ run_id: 'r2', status: 'active' });
    await useVivyStore.getState().selectSession('s1');
    await useVivyStore.getState().openRun('r1', 's1');
    await useVivyStore.getState().startRun('s1', 'think hard', 'normal', undefined, undefined, 'on');
    expect(useVivyStore.getState().queuedMessages[0]).toMatchObject({ text: 'think hard', thinking: 'on' });
    subscription.onEvent?.({ run_id: 'r1', seq: 1, type: 'run.completed', created_at: 2, payload_version: 1, payload: {} });
    await vi.waitFor(() => expect(api.startTurn).toHaveBeenCalledWith('s1', 'think hard', 'normal', undefined, undefined, 'on'));
  });

  it('retains queued messages when the run fails', async () => {
    api.listMessages.mockResolvedValue({ messages: [] });
    api.getRun.mockResolvedValue({ id: 'r1', session_id: 's1', status: 'active', created_at: 1 });
    api.getRunLog.mockResolvedValue({ events: [] });
    api.listChildren.mockResolvedValue({ children: [] });
    api.startTurn.mockResolvedValue({ run_id: 'r2', status: 'active' });
    await useVivyStore.getState().selectSession('s1');
    await useVivyStore.getState().openRun('r1', 's1');
    await useVivyStore.getState().startRun('s1', 'second message');
    subscription.onEvent?.({ run_id: 'r1', seq: 1, type: 'run.failed', created_at: 2, payload_version: 1, payload: { cause_category: 'internal_error', message: 'boom' } });
    await vi.waitFor(() => expect(useVivyStore.getState().runError).toBe('boom'));
    expect(api.startTurn).not.toHaveBeenCalled();
    expect(useVivyStore.getState().queuedMessages).toHaveLength(1);
  });

  it('clears the queue when switching sessions', async () => {
    api.listMessages.mockResolvedValue({ messages: [] });
    api.getRun.mockResolvedValue({ id: 'r1', session_id: 's1', status: 'active', created_at: 1 });
    api.getRunLog.mockResolvedValue({ events: [] });
    api.listChildren.mockResolvedValue({ children: [] });
    await useVivyStore.getState().selectSession('s1');
    await useVivyStore.getState().openRun('r1', 's1');
    await useVivyStore.getState().startRun('s1', 'second message');
    expect(useVivyStore.getState().queuedMessages).toHaveLength(1);
    await useVivyStore.getState().selectSession('s2');
    expect(useVivyStore.getState().queuedMessages).toEqual([]);
  });
});
