import { create } from 'zustand';
import * as api from './api';
import { subscribeRun, type RunEvent, type RunSubscription } from './run-subscription';
import { isTaskToolName } from './todos';
import { t } from '@/i18n';

export type Phase = 'idle' | 'loading' | 'refreshing' | 'ready' | 'empty' | 'error' | 'processing';
export type ConnectionState = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'error';

const ACTIVE_SESSION_KEY = 'vivy.ui.activeSession';
const DEMO_PREFIX = 'vivy.demo.';

export function getDemoData<T>(key: string, fallback: T): T {
  try { const value = localStorage.getItem(`${DEMO_PREFIX}${key}`); return value ? JSON.parse(value) as T : fallback; } catch { return fallback; }
}
export function setDemoData<T>(key: string, value: T): void {
  try { localStorage.setItem(`${DEMO_PREFIX}${key}`, JSON.stringify(value)); } catch { /* demo persistence is best effort */ }
}

function errorMessage(error: unknown): string { return error instanceof Error ? error.message : String(error); }
function runActive(run: api.Run | null): boolean { return !!run && !['completed', 'failed', 'cancelled'].includes(run.status); }
function replay(events: RunEvent[], type: string): string { return events.filter((event) => event.type === type).map((event) => String(event.payload.delta ?? '')).join(''); }
function lastRunId(messages: api.Message[]): string | null {
  for (let index = messages.length - 1; index >= 0; index -= 1) if (messages[index].run_id) return messages[index].run_id ?? null;
  return null;
}

interface RuntimeState {
  initialized: boolean;
  initializationError: string | null;
  capabilities: string[];
  connection: ConnectionState;
  sessions: api.Session[];
  sessionsPhase: Phase;
  sessionsError: string | null;
  sessionBusyId: string | null;
  activeSessionId: string | null;
  messages: api.Message[];
  messagesPhase: Phase;
  messagesError: string | null;
  todos: api.Todo[];
  todosPhase: Phase;
  todosError: string | null;
  todoPanelOpen: boolean;
  currentRun: api.Run | null;
  runEvents: RunEvent[];
  streamingText: string;
  streamingReasoning: string;
  runError: string | null;
  runBusy: boolean;
  backgroundRuns: api.BackgroundRun[];
  backgroundPhase: Phase;
  backgroundError: string | null;
  backgroundBusyId: string | null;
  children: api.ChildRun[];
  childrenPhase: Phase;
  childrenError: string | null;
  childBusyId: string | null;
  selectedChild: api.ChildRun | null;
  reviews: api.ReviewItem[];
  reviewsPhase: Phase;
  reviewsError: string | null;
  reviewBusyId: string | null;
  reviewCenterOpen: boolean;
  sessionDrawerOpen: boolean;
  settings: api.Settings | null;
  settingsPhase: Phase;
  settingsError: string | null;
  providers: api.ProviderEntry[];
  providersPhase: Phase;
  providersError: string | null;
  species: api.SpeciesInspect | null;
  generations: api.Generation[];
  evals: api.EvalRun[];
  promotions: api.Promotion[];
  lifecyclePhase: Phase;
  lifecycleError: string | null;
  lifecycleBusy: boolean;
  initialize: () => Promise<void>;
  loadSessions: () => Promise<void>;
  createSession: (title?: string) => Promise<api.Session>;
  renameSession: (id: string, title: string) => Promise<void>;
  deleteSession: (id: string) => Promise<void>;
  selectSession: (id: string) => Promise<void>;
  startRun: (sessionId: string, text: string, mode?: api.RunMode) => Promise<void>;
  cancelCurrentRun: () => Promise<void>;
  openRun: (runId: string, sessionId: string) => Promise<void>;
  loadBackgroundRuns: () => Promise<void>;
  attachBackgroundRun: (runId: string) => Promise<void>;
  loadChildren: (parentRunId?: string) => Promise<void>;
  startChild: (text: string, policyProfile?: string, toolNames?: string[]) => Promise<void>;
  openChild: (runId: string) => Promise<void>;
  waitChild: (runId: string) => Promise<void>;
  cancelChild: (runId: string) => Promise<void>;
  loadReviews: () => Promise<void>;
  respondReview: (id: string, response: { action: 'approve' | 'deny' | 'answer' | 'cancel'; reason?: string; answer?: string }) => Promise<void>;
  setReviewCenterOpen: (open: boolean) => void;
  setSessionDrawerOpen: (open: boolean) => void;
  loadTodos: (sessionId?: string) => Promise<void>;
  setTodoPanelOpen: (open: boolean) => void;
  loadSettings: () => Promise<void>;
  saveSettings: (value: api.SettingsUpdate) => Promise<void>;
  loadProviders: () => Promise<void>;
  saveProvider: (input: api.ProviderEntryInput) => Promise<void>;
  removeProvider: (id: string) => Promise<void>;
  loadLifecycle: () => Promise<void>;
  createGeneration: (params: Parameters<typeof api.createGeneration>[0]) => Promise<void>;
  rejectGeneration: (id: string) => Promise<void>;
  startEval: (params: Parameters<typeof api.startEval>[0]) => Promise<void>;
  recordEval: (params: Parameters<typeof api.recordEval>[0]) => Promise<void>;
  promoteGeneration: (params: Parameters<typeof api.promoteGeneration>[0]) => Promise<void>;
}

let initialization: Promise<void> | null = null;
let subscription: RunSubscription | null = null;
let sessionEpoch = 0;
let reviewEpoch = 0;

function stopSubscription(): void { subscription?.close(); subscription = null; }

async function refreshAfterTerminal(runId: string): Promise<void> {
  const state = useVivyStore.getState();
  const sessionId = state.activeSessionId;
  let messagesRefreshed = !sessionId;
  try {
    if (sessionId) await loadMessagesIntoStore(sessionId, sessionEpoch);
    messagesRefreshed = true;
  } catch (error) {
    if (useVivyStore.getState().currentRun?.id === runId) useVivyStore.setState({ runError: t('errors.refreshAfterRunFailed', { error: errorMessage(error) }) });
  }
  await Promise.all([state.loadBackgroundRuns(), state.loadChildren(runId), state.loadReviews(), state.loadTodos()]);
  if (messagesRefreshed && useVivyStore.getState().currentRun?.id === runId) useVivyStore.setState({ streamingText: '', streamingReasoning: '' });
}

function handleRunEvent(event: RunEvent): void {
  const state = useVivyStore.getState();
  if (state.currentRun?.id !== event.run_id || state.runEvents.some((item) => item.seq === event.seq)) return;
  const events = [...state.runEvents, event].sort((a, b) => a.seq - b.seq);
  const update: Partial<RuntimeState> = { runEvents: events, connection: 'connected', runError: null };
  if (event.type === 'run.started') update.currentRun = state.currentRun ? { ...state.currentRun, status: 'active' } : null;
  else if (event.type === 'model.delta') update.streamingText = state.streamingText + String(event.payload.delta ?? '');
  else if (event.type === 'model.reasoning_delta') update.streamingReasoning = state.streamingReasoning + String(event.payload.delta ?? '');
  else if (['run.completed', 'run.failed', 'run.cancelled'].includes(event.type)) {
    update.currentRun = state.currentRun ? { ...state.currentRun, status: event.type.slice(4) as api.RunStatus } : null;
    stopSubscription();
    void refreshAfterTerminal(event.run_id);
  } else if (event.type.startsWith('child.')) void state.loadChildren(event.run_id);
  else if (event.type.includes('approval') || event.type.includes('question')) void state.loadReviews();
  else if (event.type === 'tool.finished' && isTaskToolName(event.payload.tool_name)) void state.loadTodos();
  useVivyStore.setState(update);
}

function startSubscription(runId: string, afterSeq: number): void {
  stopSubscription();
  useVivyStore.setState({ connection: 'connecting', runError: null });
  subscription = subscribeRun(runId, afterSeq, handleRunEvent, (message) => {
    if (useVivyStore.getState().currentRun?.id === runId) useVivyStore.setState({ connection: 'reconnecting', runError: message });
  });
}

async function loadMessagesIntoStore(sessionId: string, epoch: number): Promise<api.Message[]> {
  const result = await api.listMessages(sessionId);
  const state = useVivyStore.getState();
  if (epoch === sessionEpoch && state.activeSessionId === sessionId) useVivyStore.setState({ messages: result.messages, messagesPhase: result.messages.length ? 'ready' : 'empty', messagesError: null });
  return result.messages;
}

async function loadTodosIntoStore(sessionId: string, epoch: number): Promise<void> {
  const result = await api.listTodos(sessionId);
  const state = useVivyStore.getState();
  if (epoch !== sessionEpoch || state.activeSessionId !== sessionId) return;
  useVivyStore.setState({ todos: result.todos, todosPhase: result.todos.length ? 'ready' : 'empty', todosError: null });
}

export const useVivyStore = create<RuntimeState>((set, get) => ({
  initialized: false, initializationError: null, capabilities: [], connection: 'idle',
  sessions: [], sessionsPhase: 'idle', sessionsError: null, sessionBusyId: null, activeSessionId: null,
  messages: [], messagesPhase: 'idle', messagesError: null,
  todos: [], todosPhase: 'idle', todosError: null, todoPanelOpen: false,
  currentRun: null, runEvents: [], streamingText: '', streamingReasoning: '', runError: null, runBusy: false,
  backgroundRuns: [], backgroundPhase: 'idle', backgroundError: null, backgroundBusyId: null,
  children: [], childrenPhase: 'idle', childrenError: null, childBusyId: null, selectedChild: null,
  reviews: [], reviewsPhase: 'idle', reviewsError: null, reviewBusyId: null, reviewCenterOpen: false, sessionDrawerOpen: false,
  settings: null, settingsPhase: 'idle', settingsError: null,
  providers: [], providersPhase: 'idle', providersError: null,
  species: null, generations: [], evals: [], promotions: [], lifecyclePhase: 'idle', lifecycleError: null, lifecycleBusy: false,

  initialize: async () => {
    if (initialization) return initialization;
    initialization = (async () => {
      set({ initializationError: null, connection: 'connecting' });
      try {
        const capabilities = await api.initialize();
        await api.recoverBackgroundRuns().catch(() => undefined);
        const [sessions, background, settings, providers] = await Promise.all([api.listSessions(), api.listBackgroundRuns(), api.getSettings().catch(() => null), api.listProviders().catch(() => null)]);
        let sessionItems = sessions.sessions;
        let initialSessionError: string | null = null;
        if (!sessionItems.length) {
          try { sessionItems = [await api.createSession(t('errors.newSessionDefault'))]; }
          catch (error) { initialSessionError = errorMessage(error); }
        }
        const saved = localStorage.getItem(ACTIVE_SESSION_KEY);
        const activeId = sessionItems.some((item) => item.id === saved) ? saved : sessionItems[0]?.id ?? null;
        set({ initialized: true, connection: 'connected', capabilities: capabilities.capabilities, sessions: sessionItems, sessionsPhase: initialSessionError ? 'error' : sessionItems.length ? 'ready' : 'empty', sessionsError: initialSessionError, backgroundRuns: background.runs, backgroundPhase: background.runs.length ? 'ready' : 'empty', settings, settingsPhase: settings ? 'ready' : 'error', providers: providers?.entries ?? [], providersPhase: providers ? 'ready' : 'error' });
        if (activeId) await get().selectSession(activeId);
        void get().loadReviews();
      } catch (error) {
        set({ initialized: true, initializationError: errorMessage(error), connection: 'error' });
      }
    })();
    return initialization;
  },

  loadSessions: async () => {
    set((state) => ({ sessionsPhase: state.sessions.length ? 'refreshing' : 'loading', sessionsError: null }));
    try { const result = await api.listSessions(); set({ sessions: result.sessions, sessionsPhase: result.sessions.length ? 'ready' : 'empty' }); }
    catch (error) { set((state) => ({ sessionsPhase: state.sessions.length ? 'ready' : 'error', sessionsError: errorMessage(error) })); }
  },
  createSession: async (title = t('errors.newSessionDefault')) => {
    set({ sessionBusyId: 'create', sessionsError: null });
    try { const created = await api.createSession(title); set((state) => ({ sessions: [created, ...state.sessions], sessionsPhase: 'ready' })); await get().selectSession(created.id); return created; }
    catch (error) { set((state) => ({ sessionsError: errorMessage(error), sessionsPhase: state.sessions.length ? state.sessionsPhase : 'error' })); throw error; } finally { set({ sessionBusyId: null }); }
  },
  renameSession: async (id, title) => {
    set({ sessionBusyId: id, sessionsError: null });
    try { const renamed = await api.renameSession(id, title); set((state) => ({ sessions: state.sessions.map((item) => item.id === id ? renamed : item) })); }
    catch (error) { set({ sessionsError: errorMessage(error) }); throw error; } finally { set({ sessionBusyId: null }); }
  },
  deleteSession: async (id) => {
    set({ sessionBusyId: id, sessionsError: null });
    try {
      await api.deleteSession(id);
      const remaining = get().sessions.filter((item) => item.id !== id);
      set({ sessions: remaining, sessionsPhase: remaining.length ? 'ready' : 'empty' });
      if (get().activeSessionId === id) {
        stopSubscription(); localStorage.removeItem(ACTIVE_SESSION_KEY);
        set({ activeSessionId: null, messages: [], todos: [], todosPhase: 'idle', todosError: null, currentRun: null, runEvents: [], children: [] });
        if (remaining[0]) await get().selectSession(remaining[0].id);
        else await get().createSession();
      }
    } catch (error) { set({ sessionsError: errorMessage(error) }); throw error; } finally { set({ sessionBusyId: null }); }
  },
  selectSession: async (id) => {
    const epoch = ++sessionEpoch;
    stopSubscription(); localStorage.setItem(ACTIVE_SESSION_KEY, id);
    set({ activeSessionId: id, messages: [], messagesPhase: 'loading', messagesError: null, todos: [], todosPhase: 'loading', todosError: null, currentRun: null, runEvents: [], streamingText: '', streamingReasoning: '', runError: null, children: [], selectedChild: null });
    try {
      const [messages] = await Promise.all([loadMessagesIntoStore(id, epoch), loadTodosIntoStore(id, epoch).catch((error) => {
        if (epoch === sessionEpoch && get().activeSessionId === id) set({ todosPhase: get().todos.length ? 'ready' : 'error', todosError: errorMessage(error) });
      })]);
      const background = await api.listBackgroundRuns();
      if (epoch !== sessionEpoch || get().activeSessionId !== id) return;
      set({ backgroundRuns: background.runs, backgroundPhase: background.runs.length ? 'ready' : 'empty' });
      const runId = background.runs.filter((run) => run.session_id === id).sort((a, b) => b.created_at - a.created_at)[0]?.id ?? lastRunId(messages);
      if (runId) await get().openRun(runId, id);
    } catch (error) { if (epoch === sessionEpoch) set({ messagesPhase: 'error', messagesError: errorMessage(error) }); }
  },
  loadTodos: async (sessionId = get().activeSessionId ?? undefined) => {
    if (!sessionId) return;
    const epoch = sessionEpoch;
    set((state) => ({ todosPhase: state.todos.length ? 'refreshing' : 'loading', todosError: null }));
    try { await loadTodosIntoStore(sessionId, epoch); }
    catch (error) { if (epoch === sessionEpoch && get().activeSessionId === sessionId) set((state) => ({ todosPhase: state.todos.length ? 'ready' : 'error', todosError: errorMessage(error) })); }
  },
  setTodoPanelOpen: (open) => set({ todoPanelOpen: open }),
  openRun: async (runId, sessionId) => {
    try {
      const [run, log, children] = await Promise.all([api.getRun(runId), api.getRunLog(runId), api.listChildren(runId, true)]);
      if (get().activeSessionId !== sessionId) return;
      const events = log.events.sort((a, b) => a.seq - b.seq);
      const active = runActive(run);
      set({ currentRun: run, runEvents: events, streamingText: active ? replay(events, 'model.delta') : '', streamingReasoning: active ? replay(events, 'model.reasoning_delta') : '', children: children.children, childrenPhase: children.children.length ? 'ready' : 'empty', connection: active ? 'connecting' : 'connected' });
      if (active) startSubscription(runId, events.reduce((max, event) => Math.max(max, event.seq), 0));
    } catch (error) { if (get().activeSessionId === sessionId) set({ runError: errorMessage(error) }); }
  },
  startRun: async (sessionId, text, mode = 'normal') => {
    if (get().activeSessionId !== sessionId || runActive(get().currentRun) || get().runBusy) return;
    set({ runBusy: true, runError: null });
    try {
      const result = await api.startTurn(sessionId, text, mode);
      if (get().activeSessionId !== sessionId) { await get().loadBackgroundRuns(); return; }
      const run: api.Run = { id: result.run_id, session_id: sessionId, status: result.status, created_at: Date.now() };
      set((state) => ({ currentRun: run, runEvents: [], streamingText: '', streamingReasoning: '', connection: 'connecting', messages: [...state.messages, { id: `local-${run.id}`, run_id: run.id, role: 'user', content: text, created_at: Date.now() }], messagesPhase: 'ready', backgroundRuns: [run, ...state.backgroundRuns.filter((item) => item.id !== run.id)], backgroundPhase: 'ready' }));
      startSubscription(run.id, 0);
    } catch (error) { set({ runError: errorMessage(error) }); throw error; } finally { set({ runBusy: false }); }
  },
  cancelCurrentRun: async () => {
    const run = get().currentRun; if (!runActive(run) || get().runBusy || !run) return;
    set({ runBusy: true, runError: null });
    try { await api.cancelRun(run.id); } catch (error) { set({ runError: errorMessage(error) }); } finally { set({ runBusy: false }); }
  },
  loadBackgroundRuns: async () => {
    set((state) => ({ backgroundPhase: state.backgroundRuns.length ? 'refreshing' : 'loading', backgroundError: null }));
    try { const result = await api.listBackgroundRuns(); set({ backgroundRuns: result.runs, backgroundPhase: result.runs.length ? 'ready' : 'empty' }); }
    catch (error) { set({ backgroundPhase: 'error', backgroundError: errorMessage(error) }); }
  },
  attachBackgroundRun: async (runId) => {
    set({ backgroundBusyId: runId, backgroundError: null });
    try { const run = await api.attachBackgroundRun(runId); if (get().activeSessionId !== run.session_id) await get().selectSession(run.session_id); await get().openRun(run.id, run.session_id); }
    catch (error) { set({ backgroundError: errorMessage(error) }); } finally { set({ backgroundBusyId: null }); }
  },
  loadChildren: async (parentRunId = get().currentRun?.id) => {
    if (!parentRunId) return;
    set((state) => ({ childrenPhase: state.children.length ? 'refreshing' : 'loading', childrenError: null }));
    try { const result = await api.listChildren(parentRunId, true); if (get().currentRun?.id === parentRunId) set({ children: result.children, childrenPhase: result.children.length ? 'ready' : 'empty' }); }
    catch (error) { set({ childrenPhase: 'error', childrenError: errorMessage(error) }); }
  },
  startChild: async (text, policyProfile, toolNames) => {
    const parent = get().currentRun; if (!parent) return;
    set({ childBusyId: 'create', childrenError: null });
    try { await api.startChild({ parent_run_id: parent.id, text, policy_profile: policyProfile || undefined, tool_names: toolNames?.length ? toolNames : undefined }); await get().loadChildren(parent.id); }
    catch (error) { set({ childrenError: errorMessage(error) }); throw error; } finally { set({ childBusyId: null }); }
  },
  openChild: async (runId) => { set({ childBusyId: runId }); try { set({ selectedChild: await api.getChild(runId) }); } catch (error) { set({ childrenError: errorMessage(error) }); } finally { set({ childBusyId: null }); } },
  waitChild: async (runId) => { set({ childBusyId: runId }); try { set({ selectedChild: await api.waitChild(runId) }); await get().loadChildren(); } catch (error) { set({ childrenError: errorMessage(error) }); } finally { set({ childBusyId: null }); } },
  cancelChild: async (runId) => { set({ childBusyId: runId }); try { set({ selectedChild: await api.cancelChild(runId) }); await get().loadChildren(); } catch (error) { set({ childrenError: errorMessage(error) }); } finally { set({ childBusyId: null }); } },
  loadReviews: async () => {
    const epoch = ++reviewEpoch; set({ reviewsPhase: get().reviews.length ? 'refreshing' : 'loading', reviewsError: null });
    try { const result = await api.listReviews({ limit: 100 }); if (epoch === reviewEpoch) set({ reviews: result.reviews, reviewsPhase: result.reviews.length ? 'ready' : 'empty' }); }
    catch (error) { if (epoch === reviewEpoch) set({ reviewsPhase: 'error', reviewsError: errorMessage(error) }); }
  },
  respondReview: async (id, response) => {
    if (get().reviewBusyId) return;
    set({ reviewBusyId: id, reviewsError: null });
    try {
      await api.respondReview(id, response);
      const status: api.ReviewStatus = response.action === 'approve' ? 'approved' : response.action === 'deny' ? 'denied' : response.action === 'answer' ? 'answered' : 'cancelled';
      set((state) => ({ reviews: state.reviews.map((review) => review.id === id ? { ...review, status } : review), reviewsPhase: 'ready' }));
      await get().loadReviews();
    }
    catch (error) { set({ reviewsError: errorMessage(error) }); throw error; } finally { set({ reviewBusyId: null }); }
  },
  setReviewCenterOpen: (open) => set({ reviewCenterOpen: open }),
  setSessionDrawerOpen: (open) => set({ sessionDrawerOpen: open }),
  loadSettings: async () => { set({ settingsPhase: 'loading', settingsError: null }); try { set({ settings: await api.getSettings(), settingsPhase: 'ready' }); } catch (error) { set({ settingsPhase: 'error', settingsError: errorMessage(error) }); } },
  saveSettings: async (value) => { set({ settingsPhase: 'processing', settingsError: null }); try { set({ settings: await api.updateSettings(value), settingsPhase: 'ready' }); } catch (error) { set({ settingsPhase: 'error', settingsError: errorMessage(error) }); throw error; } },
  loadProviders: async () => { set({ providersPhase: 'loading', providersError: null }); try { const view = await api.listProviders(); set({ providers: view.entries, providersPhase: 'ready' }); } catch (error) { set({ providersPhase: 'error', providersError: errorMessage(error) }); } },
  saveProvider: async (input) => {
    set({ providersPhase: 'processing', providersError: null });
    try {
      await api.upsertProvider(input);
      set({ providersPhase: 'ready' });
      await get().loadProviders();
    } catch (error) { set({ providersPhase: 'error', providersError: errorMessage(error) }); throw error; }
  },
  removeProvider: async (id) => {
    set({ providersPhase: 'processing', providersError: null });
    try {
      await api.deleteProvider(id);
      set({ providersPhase: 'ready' });
      await get().loadProviders();
    } catch (error) { set({ providersPhase: 'error', providersError: errorMessage(error) }); throw error; }
  },
  loadLifecycle: async () => {
    set({ lifecyclePhase: 'loading', lifecycleError: null });
    try { const [species, generations, evals, promotions] = await Promise.all([api.inspectSpecies(), api.listGenerations(), api.listEvals(), api.listPromotions()]); set({ species, generations: generations.generations, evals: evals.evals, promotions: promotions.promotions, lifecyclePhase: 'ready' }); }
    catch (error) { set({ lifecyclePhase: 'error', lifecycleError: errorMessage(error) }); }
  },
  createGeneration: async (params) => { set({ lifecycleBusy: true, lifecycleError: null }); try { await api.createGeneration(params); await get().loadLifecycle(); } catch (error) { set({ lifecycleError: errorMessage(error) }); throw error; } finally { set({ lifecycleBusy: false }); } },
  rejectGeneration: async (id) => { set({ lifecycleBusy: true, lifecycleError: null }); try { await api.rejectGeneration(id); await get().loadLifecycle(); } catch (error) { set({ lifecycleError: errorMessage(error) }); throw error; } finally { set({ lifecycleBusy: false }); } },
  startEval: async (params) => { set({ lifecycleBusy: true, lifecycleError: null }); try { await api.startEval(params); await get().loadLifecycle(); } catch (error) { set({ lifecycleError: errorMessage(error) }); throw error; } finally { set({ lifecycleBusy: false }); } },
  recordEval: async (params) => { set({ lifecycleBusy: true, lifecycleError: null }); try { await api.recordEval(params); await get().loadLifecycle(); } catch (error) { set({ lifecycleError: errorMessage(error) }); throw error; } finally { set({ lifecycleBusy: false }); } },
  promoteGeneration: async (params) => { set({ lifecycleBusy: true, lifecycleError: null }); try { await api.promoteGeneration(params); await get().loadLifecycle(); } catch (error) { set({ lifecycleError: errorMessage(error) }); throw error; } finally { set({ lifecycleBusy: false }); } },
}));

export function resetStoreForTests(): void {
  stopSubscription(); initialization = null; sessionEpoch = 0; reviewEpoch = 0;
}
