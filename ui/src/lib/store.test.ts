import { beforeEach, describe, expect, it, vi } from 'vitest';

const api = vi.hoisted(() => ({
  initialize: vi.fn(), recoverBackgroundRuns: vi.fn(), listSessions: vi.fn(), listBackgroundRuns: vi.fn(), getSettings: vi.fn(), listMessages: vi.fn(), getRun: vi.fn(), getRunLog: vi.fn(), listChildren: vi.fn(), listReviews: vi.fn(),
  createSession: vi.fn(), renameSession: vi.fn(), deleteSession: vi.fn(), startTurn: vi.fn(), cancelRun: vi.fn(), attachBackgroundRun: vi.fn(), startChild: vi.fn(), getChild: vi.fn(), waitChild: vi.fn(), cancelChild: vi.fn(), respondReview: vi.fn(), updateSettings: vi.fn(), inspectSpecies: vi.fn(), listGenerations: vi.fn(), listEvals: vi.fn(), listPromotions: vi.fn(), createGeneration: vi.fn(), rejectGeneration: vi.fn(), startEval: vi.fn(), recordEval: vi.fn(), promoteGeneration: vi.fn(),
}));
const subscription = vi.hoisted(() => ({ onEvent: undefined as undefined | ((event: { run_id: string; seq: number; type: string; created_at: number; payload_version: number; payload: Record<string, unknown> }) => void) }));
vi.mock('./api', () => api);
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
    api.getSettings.mockResolvedValue({ provider: 'mock', default_model: 'mock', base_url: '', execute_max_timeout_seconds: 0, read_only: false, config_provider: '', config_model: '', config_execute_max_timeout_seconds: 30 });
    api.listReviews.mockResolvedValue({ reviews: [] });
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
    api.createSession.mockResolvedValue({ id: 's-new', title: '新会话', created_at: 1 });
    api.listMessages.mockResolvedValue({ messages: [] });
    await useVivyStore.getState().initialize();
    expect(api.createSession).toHaveBeenCalledTimes(1);
    expect(api.createSession).toHaveBeenCalledWith('新会话');
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
});
