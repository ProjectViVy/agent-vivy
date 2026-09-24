import { beforeEach, describe, expect, it, vi } from 'vitest';

const api = vi.hoisted(() => ({
	ApiError: class ApiError extends Error { constructor(public status: number, public code: string, message: string) { super(message); } },
  initialize: vi.fn(), recoverBackgroundRuns: vi.fn(), listSessions: vi.fn(), listBackgroundRuns: vi.fn(), getSettings: vi.fn(), listMessages: vi.fn(), listTodos: vi.fn(), updateTodo: vi.fn(), getRun: vi.fn(), getRunLog: vi.fn(), listChildren: vi.fn(), listReviews: vi.fn(),
  createSession: vi.fn(), renameSession: vi.fn(), setSessionWorkspace: vi.fn(), deleteSession: vi.fn(), startTurn: vi.fn(), cancelRun: vi.fn(), attachBackgroundRun: vi.fn(), startChild: vi.fn(), getChild: vi.fn(), waitChild: vi.fn(), cancelChild: vi.fn(), respondReview: vi.fn(), updateSettings: vi.fn(), updateLocale: vi.fn(), inspectSpecies: vi.fn(), listGenerations: vi.fn(), listEvals: vi.fn(), listPromotions: vi.fn(), createGeneration: vi.fn(), rejectGeneration: vi.fn(), startEval: vi.fn(), recordEval: vi.fn(), promoteGeneration: vi.fn(),
  listProviders: vi.fn(), upsertProvider: vi.fn(), deleteProvider: vi.fn(), getSessionWork: vi.fn(),
}));
const subscription = vi.hoisted(() => ({ onEvent: undefined as undefined | ((event: { run_id: string; seq: number; type: string; created_at: number; payload_version: number; payload: Record<string, unknown> }) => void) }));
const workSubscription = vi.hoisted(() => ({
  onEvent: undefined as undefined | ((event: { seq: number; kind: string; request_id: string; created_at: number }) => void),
  onReconnect: undefined as undefined | (() => Promise<void>),
  onConnected: undefined as undefined | ((processEpoch: string) => Promise<void>),
  retry: vi.fn(),
  close: vi.fn(),
}));
vi.mock('./api', () => api);
vi.mock('./rpc', () => ({ resetRpcClient: vi.fn() }));
vi.mock('./run-subscription', () => ({ subscribeRun: vi.fn((_id: string, _seq: number, onEvent: typeof subscription.onEvent) => { subscription.onEvent = onEvent; return { close: vi.fn(), lastSeq: () => 0 }; }) }));
vi.mock('./work-subscription', () => ({ subscribeWork: vi.fn((_id: string, afterSeq: number, onEvent: typeof workSubscription.onEvent, _onError: unknown, onReconnect: typeof workSubscription.onReconnect, onConnected: typeof workSubscription.onConnected) => {
  workSubscription.onEvent = onEvent;
  workSubscription.onReconnect = onReconnect;
  workSubscription.onConnected = onConnected;
  return { close: workSubscription.close, retry: workSubscription.retry, lastSeq: () => afterSeq };
}) }));

import { resetStoreForTests, useVivyStore } from './store';
import * as localeStore from '@/i18n';

function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>((done) => { resolve = done; }); return { promise, resolve }; }
function hydrateLocale(locale: 'en' | 'zh'): void {
  expect(localeStore).toHaveProperty('hydrateLocale');
  (localeStore as typeof localeStore & { hydrateLocale: (value: 'en' | 'zh') => void }).hydrateLocale(locale);
}

function settings(locale: 'en' | 'zh', workspaceLocale: '' | 'en' | 'zh' = locale): Awaited<ReturnType<typeof api.getSettings>> {
  return {
    provider: 'deepseek', default_model: 'deepseek-flash', base_url: '', execute_max_timeout_seconds: 0,
    read_only: false, config_provider: '', config_model: '', config_execute_max_timeout_seconds: 30,
    locale, generation_locale: 'en', workspace_locale: workspaceLocale, locale_read_only: false,
  };
}
function work(sessionId: string, version: number, activation: 'armed' | 'disarmed', processEpoch: string, runId = '') {
  return { session_id: sessionId, version, activation, process_epoch: processEpoch, current_run_id: runId,
    plan: { active: false, review_status: 'none' }, goal: { id: 'goal-1', revision: 1, objective: 'ship', phase: 'active', max_rounds: 2, rounds_started: 1 } };
}

describe('Vivy store integrity', () => {
  beforeEach(() => {
    const values = new Map<string, string>();
    const storage = { getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => values.set(key, value), removeItem: (key: string) => values.delete(key) };
    vi.stubGlobal('localStorage', storage);
    vi.stubGlobal('window', { localStorage: storage });
    resetStoreForTests();
    localeStore.resetLocaleForTests();
    useVivyStore.setState(useVivyStore.getInitialState(), true);
    vi.clearAllMocks();
    subscription.onEvent = undefined;
    workSubscription.onEvent = undefined;
    workSubscription.onReconnect = undefined;
    workSubscription.onConnected = undefined;
    workSubscription.close.mockReset();
    workSubscription.retry.mockReset();
    api.initialize.mockResolvedValue({ protocol_version: 'vivy.rpc.v1', capabilities: ['session', 'run.subscribe'] });
    api.recoverBackgroundRuns.mockResolvedValue({ recovered: true });
    api.listBackgroundRuns.mockResolvedValue({ runs: [] });
    api.getSettings.mockResolvedValue({ provider: 'deepseek', default_model: 'deepseek-flash', base_url: '', execute_max_timeout_seconds: 0, read_only: false, config_provider: '', config_model: '', config_execute_max_timeout_seconds: 30, locale: 'en', generation_locale: 'en', workspace_locale: '', locale_read_only: false });
    api.listProviders.mockResolvedValue({ entries: [], catalog: [], active_provider: '', active_model: '', active_base_url: '', read_only: false, config_provider: '', config_model: '' });
    api.listReviews.mockResolvedValue({ reviews: [] });
    api.listTodos.mockResolvedValue({ todos: [] });
    api.getSessionWork.mockImplementation(async (id: string) => work(id, 0, 'disarmed', 'process-a'));
  });

  it('initializes one authoritative active session and restores its messages', async () => {
    api.listSessions.mockResolvedValue({ sessions: [{ id: 's1', title: 'One', created_at: 1 }] });
    api.listMessages.mockResolvedValue({ messages: [{ id: 'm1', role: 'user', content: 'hello', created_at: 2 }] });
    await useVivyStore.getState().initialize();
    expect(useVivyStore.getState()).toMatchObject({ initialized: true, activeSessionId: 's1', messagesPhase: 'ready' });
    expect(useVivyStore.getState().messages[0].content).toBe('hello');
  });

  it('hydrates the effective locale from settings during initialization', async () => {
    hydrateLocale('en');
    api.getSettings.mockResolvedValue({ provider: 'deepseek', default_model: 'deepseek-flash', base_url: '', execute_max_timeout_seconds: 0, read_only: false, config_provider: '', config_model: '', config_execute_max_timeout_seconds: 30, locale: 'zh', generation_locale: 'en', workspace_locale: 'zh', locale_read_only: false });
    api.listSessions.mockResolvedValue({ sessions: [{ id: 's1', title: 'One', created_at: 1 }] });
    api.listMessages.mockResolvedValue({ messages: [] });

    await useVivyStore.getState().initialize();

    expect(localeStore.getLocale()).toBe('zh');
    expect(localStorage.getItem('vivy.language')).toBe('zh');
  });

  it('applies only the locale returned by the backend when saving', async () => {
    hydrateLocale('zh');
    useVivyStore.setState({ settings: await api.getSettings() });
    api.updateLocale.mockResolvedValue({ locale: 'en', generation_locale: 'en', workspace_locale: '', locale_read_only: false });
    const saveLocale = (useVivyStore.getState() as ReturnType<typeof useVivyStore.getState> & {
      saveLocale: (locale: 'en' | 'zh') => Promise<void>;
    }).saveLocale;
    expect(saveLocale).toBeTypeOf('function');

    await saveLocale('zh');

    expect(api.updateLocale).toHaveBeenCalledWith('zh');
    expect(localeStore.getLocale()).toBe('en');
    expect(useVivyStore.getState().settings).toMatchObject({ locale: 'en', workspace_locale: '' });
    expect(useVivyStore.getState()).toMatchObject({ settingsPhase: 'ready', settingsError: null });
  });

  it('keeps the effective locale and exposes settingsError when saving fails', async () => {
    hydrateLocale('en');
    api.updateLocale.mockRejectedValue(new Error('settings are read-only'));
    const saveLocale = (useVivyStore.getState() as ReturnType<typeof useVivyStore.getState> & {
      saveLocale: (locale: 'en' | 'zh') => Promise<void>;
    }).saveLocale;
    expect(saveLocale).toBeTypeOf('function');

    await expect(saveLocale('zh')).rejects.toThrow('settings are read-only');

    expect(localeStore.getLocale()).toBe('en');
    expect(useVivyStore.getState()).toMatchObject({ settingsPhase: 'error', settingsError: 'settings are read-only' });
  });

  it('does not let a delayed settings read override a newer locale save', async () => {
    const read = deferred<Awaited<ReturnType<typeof api.getSettings>>>();
    api.getSettings.mockReturnValueOnce(read.promise);
    api.updateLocale.mockResolvedValue({ locale: 'zh', generation_locale: 'en', workspace_locale: 'zh', locale_read_only: false });
    useVivyStore.setState({ settings: settings('en', '') });

    const load = useVivyStore.getState().loadSettings();
    const save = useVivyStore.getState().saveLocale('zh');
    await save;
    read.resolve(settings('en', ''));
    await load;

    expect(localeStore.getLocale()).toBe('zh');
    expect(useVivyStore.getState().settings).toMatchObject({ locale: 'zh', workspace_locale: 'zh' });
    expect(useVivyStore.getState()).toMatchObject({ settingsPhase: 'ready', settingsError: null });
  });

  it('does not let a settings read started during a locale save suppress that save', async () => {
    const localeSave = deferred<Awaited<ReturnType<typeof api.updateLocale>>>();
    api.updateLocale.mockReturnValueOnce(localeSave.promise);
    api.getSettings.mockResolvedValueOnce(settings('en', ''));
    useVivyStore.setState({ settings: settings('en', '') });

    const save = useVivyStore.getState().saveLocale('zh');
    await useVivyStore.getState().loadSettings();
    localeSave.resolve({ locale: 'zh', generation_locale: 'en', workspace_locale: 'zh', locale_read_only: false });
    await save;

    expect(localeStore.getLocale()).toBe('zh');
    expect(useVivyStore.getState().settings).toMatchObject({ locale: 'zh', workspace_locale: 'zh' });
    expect(useVivyStore.getState()).toMatchObject({ settingsPhase: 'ready', settingsError: null });
  });

  it('does not let delayed initialization settings override a locale save started later', async () => {
    const initializationSettings = deferred<Awaited<ReturnType<typeof api.getSettings>>>();
    api.getSettings.mockReturnValueOnce(initializationSettings.promise);
    api.listSessions.mockResolvedValue({ sessions: [{ id: 's1', title: 'One', created_at: 1 }] });
    api.listMessages.mockResolvedValue({ messages: [] });
    api.updateLocale.mockResolvedValue({ locale: 'zh', generation_locale: 'en', workspace_locale: 'zh', locale_read_only: false });
    useVivyStore.setState({ settings: settings('en', '') });

    const initialize = useVivyStore.getState().initialize();
    await vi.waitFor(() => expect(api.getSettings).toHaveBeenCalledTimes(1));
    await useVivyStore.getState().saveLocale('zh');
    initializationSettings.resolve(settings('en', ''));
    await initialize;

    expect(localeStore.getLocale()).toBe('zh');
    expect(useVivyStore.getState().settings).toMatchObject({ locale: 'zh', workspace_locale: 'zh' });
  });

  it('does not let an older locale-save response override a newer locale save', async () => {
    const firstSave = deferred<Awaited<ReturnType<typeof api.updateLocale>>>();
    api.updateLocale.mockReturnValueOnce(firstSave.promise).mockResolvedValueOnce({ locale: 'en', generation_locale: 'en', workspace_locale: 'en', locale_read_only: false });
    useVivyStore.setState({ settings: settings('en', '') });

    const older = useVivyStore.getState().saveLocale('zh');
    await useVivyStore.getState().saveLocale('en');
    firstSave.resolve({ locale: 'zh', generation_locale: 'en', workspace_locale: 'zh', locale_read_only: false });
    await older;

    expect(localeStore.getLocale()).toBe('en');
    expect(useVivyStore.getState().settings).toMatchObject({ locale: 'en', workspace_locale: 'en' });
  });

  it('hydrates locale from the newest ordinary settings-save response', async () => {
    const localeSave = deferred<Awaited<ReturnType<typeof api.updateLocale>>>();
    const settingsSave = deferred<Awaited<ReturnType<typeof api.updateSettings>>>();
    api.updateLocale.mockReturnValueOnce(localeSave.promise);
    api.updateSettings.mockReturnValueOnce(settingsSave.promise);
    useVivyStore.setState({ settings: settings('zh', 'zh') });
    hydrateLocale('zh');

    const older = useVivyStore.getState().saveLocale('zh');
    const newer = useVivyStore.getState().saveSettings({ provider: 'deepseek', default_model: 'deepseek-flash', base_url: '' });
    settingsSave.resolve(settings('en', 'en'));
    await newer;
    localeSave.resolve({ locale: 'zh', generation_locale: 'en', workspace_locale: 'zh', locale_read_only: false });
    await older;

    expect(localeStore.getLocale()).toBe('en');
    expect(localStorage.getItem('vivy.language')).toBe('en');
    expect(useVivyStore.getState().settings).toMatchObject({ locale: 'en', workspace_locale: 'en' });
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

  it('replaces armed activation with the restarted backend WorkView on reconnect', async () => {
    api.listMessages.mockResolvedValue({ messages: [] });
    api.getSessionWork.mockResolvedValueOnce(work('s1', 1, 'armed', 'process-a', 'r1'));
    await useVivyStore.getState().selectSession('s1');
    expect(useVivyStore.getState().work).toMatchObject({ activation: 'armed', process_epoch: 'process-a' });
    api.getSessionWork.mockResolvedValueOnce(work('s1', 1, 'disarmed', 'process-b'));

    expect(workSubscription.onReconnect).toBeTypeOf('function');
    await workSubscription.onReconnect?.();

    expect(useVivyStore.getState().work).toMatchObject({ activation: 'disarmed', process_epoch: 'process-b', version: 1 });
    expect(localStorage.getItem('vivy.ui.work')).toBeNull();
  });

  it('drops an armed initial WorkView when the first subscription belongs to a restarted process', async () => {
    api.listMessages.mockResolvedValue({ messages: [] });
    api.getSessionWork.mockResolvedValueOnce(work('s1', 1, 'armed', 'process-a', 'r1'));
    await useVivyStore.getState().selectSession('s1');
    api.getSessionWork.mockResolvedValueOnce(work('s1', 1, 'disarmed', 'process-b'));

    expect(workSubscription.onConnected).toBeTypeOf('function');
    await workSubscription.onConnected?.('process-b');

    expect(useVivyStore.getState().work).toMatchObject({ activation: 'disarmed', process_epoch: 'process-b' });
    expect(api.getSessionWork).toHaveBeenCalledTimes(2);
  });

  it('does not reopen an older run after a work event arrives during session initialization', async () => {
    const background = deferred<{ runs: [] }>();
    api.listMessages.mockResolvedValue({ messages: [] });
    api.listBackgroundRuns.mockReturnValueOnce(background.promise);
    api.getSessionWork.mockResolvedValueOnce(work('s1', 1, 'armed', 'process-a', 'r1'));
    api.getSessionWork.mockResolvedValueOnce(work('s1', 2, 'armed', 'process-a', 'r2'));
    api.getRun.mockImplementation(async (id: string) => ({ id, session_id: 's1', status: 'completed', created_at: 2 }));
    api.getRunLog.mockResolvedValue({ events: [] });
    api.listChildren.mockResolvedValue({ children: [] });

    const selecting = useVivyStore.getState().selectSession('s1');
    await vi.waitFor(() => expect(workSubscription.onEvent).toBeTypeOf('function'));
    workSubscription.onEvent?.({ seq: 2, kind: 'goal.round_admitted', request_id: 'round-2', created_at: 2 });
    await vi.waitFor(() => expect(useVivyStore.getState().currentRun?.id).toBe('r2'));
    background.resolve({ runs: [] });
    await selecting;

    expect(useVivyStore.getState().currentRun?.id).toBe('r2');
    expect(api.getRun).not.toHaveBeenCalledWith('r1');
  });

  it('does not let an older overlapping run open finish after a newer open', async () => {
    const oldRun = deferred<{ id: string; session_id: string; status: string; created_at: number }>();
    api.listMessages.mockResolvedValue({ messages: [] });
    await useVivyStore.getState().selectSession('s1');
    api.getRun.mockImplementation((id: string) => id === 'r1' ? oldRun.promise : Promise.resolve({ id, session_id: 's1', status: 'completed', created_at: 2 }));
    api.getRunLog.mockResolvedValue({ events: [] });
    api.listChildren.mockResolvedValue({ children: [] });

    const openingOld = useVivyStore.getState().openRun('r1', 's1');
    await useVivyStore.getState().openRun('r2', 's1');
    oldRun.resolve({ id: 'r1', session_id: 's1', status: 'completed', created_at: 1 });
    await openingOld;

    expect(useVivyStore.getState().currentRun?.id).toBe('r2');
  });

  it('drops armed activation and retries when an event WorkView refresh fails', async () => {
    api.listMessages.mockResolvedValue({ messages: [] });
    api.getSessionWork.mockResolvedValueOnce(work('s1', 1, 'armed', 'process-a'));
    await useVivyStore.getState().selectSession('s1');
    api.getSessionWork.mockRejectedValueOnce(new Error('temporary work read failure'));

    workSubscription.onEvent?.({ seq: 2, kind: 'goal.paused', request_id: 'pause-2', created_at: 2 });
    await vi.waitFor(() => expect(useVivyStore.getState().workPhase).toBe('error'));

    expect(useVivyStore.getState().work).toBeNull();
    expect(workSubscription.retry).toHaveBeenCalledOnce();

    api.getSessionWork.mockResolvedValueOnce(work('s1', 2, 'disarmed', 'process-a'));
    await workSubscription.onReconnect?.();
    expect(useVivyStore.getState().work).toMatchObject({ version: 2, activation: 'disarmed' });
  });

  it('opens a committed automatic run once when its work event is duplicated', async () => {
    api.listMessages.mockResolvedValue({ messages: [] });
    api.getSessionWork.mockResolvedValueOnce(work('s1', 1, 'armed', 'process-a'));
    api.getSessionWork.mockResolvedValueOnce(work('s1', 2, 'armed', 'process-a', 'r2'));
    api.getRun.mockResolvedValue({ id: 'r2', session_id: 's1', status: 'active', created_at: 2 });
    api.getRunLog.mockResolvedValue({ events: [] });
    api.listChildren.mockResolvedValue({ children: [] });
    await useVivyStore.getState().selectSession('s1');
    const event = { seq: 2, kind: 'goal.round_admitted', request_id: 'round-2', created_at: 2 };

    workSubscription.onEvent?.(event);
    workSubscription.onEvent?.(event);
    await vi.waitFor(() => expect(useVivyStore.getState().currentRun?.id).toBe('r2'));

    expect(api.getSessionWork).toHaveBeenCalledTimes(2);
    expect(api.getRun).toHaveBeenCalledTimes(1);
  });

  it('discards a previous session WorkView response after a switch', async () => {
    api.listMessages.mockResolvedValue({ messages: [] });
    const oldWork = deferred<ReturnType<typeof work>>();
    api.getSessionWork.mockImplementation((id: string) => id === 's1' ? oldWork.promise : Promise.resolve(work('s2', 3, 'disarmed', 'process-b')));
    const first = useVivyStore.getState().selectSession('s1');
    const second = useVivyStore.getState().selectSession('s2');
    await second;
    oldWork.resolve(work('s1', 4, 'armed', 'process-a', 'r-old'));
    await first;

    expect(useVivyStore.getState().activeSessionId).toBe('s2');
    expect(useVivyStore.getState().work).toMatchObject({ session_id: 's2', process_epoch: 'process-b', activation: 'disarmed' });
  });

  it('sets a workspace on the empty active session without creating another chat', async () => {
    useVivyStore.setState({
      sessions: [{ id: 's1', title: '', created_at: 1, workspace_path: '' }],
      activeSessionId: 's1',
      messages: [],
      currentRun: null,
    });
    api.setSessionWorkspace.mockResolvedValue({ id: 's1', title: '', created_at: 1, workspace_path: '/code/vivy' });

    await useVivyStore.getState().chooseWorkspace('/code/vivy');

    expect(api.setSessionWorkspace).toHaveBeenCalledWith('s1', '/code/vivy');
    expect(api.createSession).not.toHaveBeenCalled();
    expect(useVivyStore.getState().sessions[0].workspace_path).toBe('/code/vivy');
  });

  it('starts a new workspace-bound chat when the active session has history', async () => {
    useVivyStore.setState({
      sessions: [{ id: 's1', title: 'Existing', created_at: 1, workspace_path: '' }],
      activeSessionId: 's1',
      messages: [{ id: 'm1', role: 'user', content: 'hello', created_at: 2 }],
      currentRun: null,
    });
    api.createSession.mockResolvedValue({ id: 's2', title: '', created_at: 3, workspace_path: '/code/vivy' });
    api.listMessages.mockResolvedValue({ messages: [] });

    await useVivyStore.getState().chooseWorkspace('/code/vivy');

    expect(api.setSessionWorkspace).not.toHaveBeenCalled();
    expect(api.createSession).toHaveBeenCalledWith('', '/code/vivy');
    expect(useVivyStore.getState().activeSessionId).toBe('s2');
  });

	it('keeps the current chat when its workspace is selected again', async () => {
		const active = { id: 's1', title: 'Existing', created_at: 1, workspace_path: '/code/vivy' };
		useVivyStore.setState({
			sessions: [active],
			activeSessionId: 's1',
			messages: [{ id: 'm1', role: 'user', content: 'hello', created_at: 2 }],
			currentRun: null,
		});

		await expect(useVivyStore.getState().chooseWorkspace('/code/vivy')).resolves.toEqual(active);

		expect(api.setSessionWorkspace).not.toHaveBeenCalled();
		expect(api.createSession).not.toHaveBeenCalled();
		expect(useVivyStore.getState().activeSessionId).toBe('s1');
	});

	it('keeps a historical default-workspace chat when default is selected again', async () => {
		const active = { id: 's1', title: 'Existing', created_at: 1, workspace_path: '' };
		useVivyStore.setState({
			sessions: [active],
			activeSessionId: 's1',
			messages: [{ id: 'm1', role: 'user', content: 'hello', created_at: 2 }],
			currentRun: null,
		});

		await expect(useVivyStore.getState().chooseWorkspace('')).resolves.toEqual(active);
		expect(api.setSessionWorkspace).not.toHaveBeenCalled();
		expect(api.createSession).not.toHaveBeenCalled();
	});

	it('starts a new chat when the backend reports hidden durable history', async () => {
		useVivyStore.setState({
			sessions: [{ id: 's1', title: '', created_at: 1, workspace_path: '' }],
			activeSessionId: 's1',
			messages: [],
			currentRun: null,
		});
		api.setSessionWorkspace.mockRejectedValue(new api.ApiError(409, 'conflict', 'workspace is locked'));
		api.createSession.mockResolvedValue({ id: 's2', title: '', created_at: 3, workspace_path: '/code/vivy' });
		api.listMessages.mockResolvedValue({ messages: [] });

		await useVivyStore.getState().chooseWorkspace('/code/vivy');

		expect(api.createSession).toHaveBeenCalledWith('', '/code/vivy');
		expect(useVivyStore.getState().activeSessionId).toBe('s2');
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
      payload: { cause_category: 'provider_error', message: 'provider deepseek: API key missing' },
    });
    expect(useVivyStore.getState().currentRun?.status).toBe('failed');
    expect(useVivyStore.getState().runError).toBe('Unable to connect! Check your provider configuration!');
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
    await vi.waitFor(() => expect(useVivyStore.getState().runError).toContain('refreshing messages failed'));
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

  it('updates todo status when session is idle', async () => {
    useVivyStore.setState({
      activeSessionId: 's1',
      currentRun: null,
      todos: [{ id: '1', session_id: 's1', subject: 'task 1', description: '', status: 'pending', blocks: [], blocked_by: [], position: 0, created_at: 1, updated_at: 1 }],
    });
    api.updateTodo.mockResolvedValueOnce({
      todo: { id: '1', session_id: 's1', subject: 'task 1', description: '', status: 'completed', blocks: [], blocked_by: [], position: 0, created_at: 1, updated_at: 2 },
    });
    await useVivyStore.getState().updateTodoStatus('1', 'completed');
    expect(api.updateTodo).toHaveBeenCalledWith('s1', '1', 'completed');
    expect(useVivyStore.getState().todos[0].status).toBe('completed');
  });

  it('refuses to update todo status while a run is active', async () => {
    useVivyStore.setState({
      activeSessionId: 's1',
      currentRun: { id: 'r1', session_id: 's1', status: 'active', created_at: 1 },
      todos: [{ id: '1', session_id: 's1', subject: 'task 1', description: '', status: 'pending', blocks: [], blocked_by: [], position: 0, created_at: 1, updated_at: 1 }],
    });
    await useVivyStore.getState().updateTodoStatus('1', 'completed');
    expect(api.updateTodo).not.toHaveBeenCalled();
    expect(useVivyStore.getState().todos[0].status).toBe('pending');
  });

  it('rolls back todo status on api failure', async () => {
    useVivyStore.setState({
      activeSessionId: 's1',
      currentRun: null,
      todos: [{ id: '1', session_id: 's1', subject: 'task 1', description: '', status: 'pending', blocks: [], blocked_by: [], position: 0, created_at: 1, updated_at: 1 }],
    });
    api.updateTodo.mockRejectedValueOnce(new Error('session has an active run'));
    await useVivyStore.getState().updateTodoStatus('1', 'completed');
    expect(useVivyStore.getState().todos[0].status).toBe('pending');
    expect(useVivyStore.getState().todosError).toBe('session has an active run');
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

  it('does not overwrite session created while initialize is in-flight', async () => {
    const sessionsDeferred = deferred<{ sessions: Array<{ id: string; title: string; created_at: number }> }>();
    api.listSessions.mockReturnValue(sessionsDeferred.promise);
    api.createSession.mockResolvedValue({ id: 's-user-created', title: 'User Session', created_at: 10 });
    api.listMessages.mockResolvedValue({ messages: [] });

    // Start initialize (which suspends on listSessions)
    const initPromise = useVivyStore.getState().initialize();

    // User explicitly creates a session during the wait window
    await useVivyStore.getState().createSession('User Session');
    expect(useVivyStore.getState().activeSessionId).toBe('s-user-created');

    // Server responds later with its snapshot
    sessionsDeferred.resolve({ sessions: [{ id: 's-server-1', title: 'Server Session', created_at: 1 }] });
    await initPromise;

    // User session remains active and list contains both
    expect(useVivyStore.getState().activeSessionId).toBe('s-user-created');
    expect(useVivyStore.getState().sessions.map((s) => s.id)).toEqual(['s-user-created', 's-server-1']);
  });

  it('does not create redundant default session if user created one during initialize', async () => {
    const sessionsDeferred = deferred<{ sessions: Array<{ id: string; title: string; created_at: number }> }>();
    api.listSessions.mockReturnValue(sessionsDeferred.promise);
    api.createSession.mockResolvedValue({ id: 's-user-created', title: 'User Session', created_at: 10 });
    api.listMessages.mockResolvedValue({ messages: [] });

    const initPromise = useVivyStore.getState().initialize();
    await useVivyStore.getState().createSession('User Session');
    sessionsDeferred.resolve({ sessions: [] });
    await initPromise;

    expect(api.createSession).toHaveBeenCalledTimes(1);
    expect(useVivyStore.getState().activeSessionId).toBe('s-user-created');
    expect(useVivyStore.getState().sessions.map((s) => s.id)).toEqual(['s-user-created']);
  });

  it('throws and records runError when startRun is called with mismatched session', async () => {
    api.listMessages.mockResolvedValue({ messages: [] });
    await useVivyStore.getState().selectSession('s1');

    await expect(useVivyStore.getState().startRun('s2', 'hello')).rejects.toThrow();
    expect(api.startTurn).not.toHaveBeenCalled();
    expect(useVivyStore.getState().runError).toBeTruthy();
  });

  it('throws and records runError when editSession is called with mismatched session', async () => {
    api.listMessages.mockResolvedValue({ messages: [] });
    await useVivyStore.getState().selectSession('s1');

    await expect(useVivyStore.getState().editSession('s2', 'm1', 'new content')).rejects.toThrow();
    expect(api.startTurn).not.toHaveBeenCalled();
    expect(useVivyStore.getState().runError).toBeTruthy();
  });
});
