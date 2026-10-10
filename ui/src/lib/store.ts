import { create } from 'zustand';
import * as api from './api';
import { runFailedMessage } from './failure';
import { resetRpcClient } from './rpc';
import { subscribeRun, type RunEvent, type RunSubscription } from './run-subscription';
import { subscribeWork, type WorkSubscription } from './work-subscription';
import { isTaskToolName } from './todos';
import { recentRunIds } from './run-rows';
import { checkDeliverable, DeliverableTransferError, downloadDeliverable, readDeliveryPreview } from './deliverable-download';
import { hydrateLocale, t } from '@/i18n';

export type Phase = 'idle' | 'loading' | 'refreshing' | 'ready' | 'empty' | 'error' | 'processing';
export type ConnectionState = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'error';

export interface ReferenceDraft {
  id: string;
  preview: api.ReferencePreview;
  selection: api.ReferenceSelection;
  /** further-reading 勾选：本引用要求把源会话纳入任务级读域。 */
  allowFurther?: boolean;
}

let draftSeq = 0;
const newDraftRequestId = () =>
  typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
    ? `req_${crypto.randomUUID()}`
    : `req_${Date.now()}_${++draftSeq}`;

/** Full copy so later composer edits cannot mutate a queued/draft submission. */
const copySubmission = (submission: api.TurnSubmission): api.TurnSubmission => ({
  ...submission,
  attachments: submission.attachments?.map((item) => ({ ...item })),
  continuity: submission.continuity && {
    request_id: submission.continuity.request_id,
    references: submission.continuity.references?.map((reference) => ({
      ...reference,
      selection: {
        ...reference.selection,
        refs: reference.selection.refs?.map((ref) => ({ ...ref })),
        run_range: reference.selection.run_range && { ...reference.selection.run_range },
      },
    })),
    history_scope: submission.continuity.history_scope && {
      ...submission.continuity.history_scope,
      session_ids: submission.continuity.history_scope.session_ids && [...submission.continuity.history_scope.session_ids],
    },
  },
});

const ACTIVE_SESSION_KEY = 'vivy.ui.activeSession';
const DEMO_PREFIX = 'vivy.demo.';

export function getDemoData<T>(key: string, fallback: T): T {
  try { const value = localStorage.getItem(`${DEMO_PREFIX}${key}`); return value ? JSON.parse(value) as T : fallback; } catch { return fallback; }
}
export function setDemoData<T>(key: string, value: T): void {
  try { localStorage.setItem(`${DEMO_PREFIX}${key}`, JSON.stringify(value)); } catch { /* demo persistence is best effort */ }
}

function errorMessage(error: unknown): string { return error instanceof Error ? error.message : String(error); }
export function runActive(run: api.Run | null): boolean { return !!run && !['completed', 'failed', 'cancelled'].includes(run.status); }
function replay(events: RunEvent[], type: string): string { return events.filter((event) => event.type === type).map((event) => String(event.payload.delta ?? '')).join(''); }
function lastRunId(messages: api.Message[]): string | null {
  for (let index = messages.length - 1; index >= 0; index -= 1) if (messages[index].run_id) return messages[index].run_id ?? null;
  return null;
}

/** 历史运行事件缓存上限：转写只折叠最近若干个运行，避免一次会话拉爆内存。 */
const RUN_LOG_CACHE_LIMIT = 8;

/** 交付传输的 RPC 接缝（SC-D4 §12）：只有 digest 绑定的 read + close。
 * 调用时解析引用，测试可替换 api 模块实现。 */
const deliverableRpc = {
  read: (sessionId: string, req: api.DeliveryReadRequest) => api.deliverablesRead(sessionId, req),
  close: (sessionId: string, transferId: string) => api.deliverablesClose(sessionId, transferId),
};

/** 每个 item 同时只允许一个活动下载（由 store 内存控制器保证）。 */
const deliveryDownloads = new Map<string, AbortController>();

const deliveryErrorStatus = (err: unknown): api.DeliveryItemStatus => {
  const reason = err instanceof DeliverableTransferError ? err.reason : 'unavailable';
  return reason === 'changed' || reason === 'missing' || reason === 'forbidden' || reason === 'unavailable' ? reason : 'unavailable';
};

/** 写入一条运行事件缓存；超出上限时按插入顺序淘汰最早的。 */
function withCachedRunLog(cache: Record<string, RunEvent[]>, runId: string, events: RunEvent[]): Record<string, RunEvent[]> {
  if (events.length === 0) return cache;
  const next: Record<string, RunEvent[]> = { ...cache, [runId]: events };
  const keys = Object.keys(next);
  for (const stale of keys.slice(0, Math.max(0, keys.length - RUN_LOG_CACHE_LIMIT))) {
    if (stale !== runId) delete next[stale];
  }
  return next;
}

interface RuntimeState {
  initialized: boolean;
  initializationError: string | null;
  capabilities: string[];
  /** Backend-advertised ability to submit runs with the code Face. */
  codeModeAvailable: boolean;
  /** Per-face-session code mode toggle; mask selection never owns this state. */
  codeMode: boolean;
  connection: ConnectionState;
  sessions: api.Session[];
  sessionsPhase: Phase;
  sessionsError: string | null;
  sessionBusyId: string | null;
  activeSessionId: string | null;
  messages: api.Message[];
  messagesPhase: Phase;
  messagesError: string | null;
  /** session/context —— 聊天环与设置压缩卡的真实上下文压力。 */
  sessionContext: api.SessionContext | null;
  todos: api.Todo[];
  todosPhase: Phase;
  todosError: string | null;
  todoPanelOpen: boolean;
  currentRun: api.Run | null;
  runEvents: RunEvent[];
  /** 历史运行的事件缓存（run/log 回放）：转写据此折叠工具/思考行。 */
  runLogs: Record<string, RunEvent[]>;
  streamingText: string;
  streamingReasoning: string;
  runError: string | null;
  runBusy: boolean;
  work: api.WorkState | null;
  workPhase: Phase;
  workError: string | null;
  workBusy: boolean;
  /** 内核双轨队列（pi parity, VCP-B3）：steer 轨在运行中立即 steering，
   *  follow_up 轨在终态 settle 后由内核按完整提交准入下一条 run。 */
  kernelQueue: api.QueueState | null;
  /** 队列事件（abort-flush/clear）回填编辑器的草稿；seq 去重。 */
  queueRestoreText: { text: string; seq: number } | null;
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
  reviewBusyIds: string[];
  reviewCenterOpen: boolean;
  filesPanelOpen: boolean;
  sessionDrawerOpen: boolean;
  settings: api.Settings | null;
  settingsPhase: Phase;
  settingsError: string | null;
  providers: api.ProviderEntry[];
  /**
   * 后端 `settings/providers` 的目录快照（PROV-P4）：前端唯一的厂商/端点/模型
   * 数据源，任何组件都不得自持目录数据。与 `providers` 同一次 RPC 写入。
   */
  catalog: api.ProviderCatalogEntry[];
  /**
   * provider 注册表相位：`idle` = 尚未请求（目录未装载），`loading` = RPC 在途，
   * `ready` = 已应答（目录可用），`error` = 失败，`processing` = 写操作在途
   * （目录仍为上一次的快照）。设置页据此在目录到达前渲染 loading。
   */
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
  retryInitialize: () => Promise<void>;
  setCodeMode: (enabled: boolean) => void;
  loadSessions: () => Promise<void>;
  createSession: (title?: string, workspacePath?: string) => Promise<api.Session>;
	chooseWorkspace: (workspacePath: string) => Promise<api.Session>;
  renameSession: (id: string, title: string) => Promise<void>;
  setSessionPermission: (id: string, preset: Exclude<api.PermissionPreset, 'custom'>) => Promise<void>;
  deleteSession: (id: string) => Promise<void>;
  selectSession: (id: string) => Promise<void>;
  startRun: (sessionId: string, submission: api.TurnSubmission) => Promise<void>;
	editSession: (sessionId: string, messageId: string, text: string, mode?: api.RunMode, face?: api.Face, thinking?: api.ThinkingMode) => Promise<void>;
  enqueueMessage: (submission: api.TurnSubmission) => Promise<void>;
  /** 内核队列动词：完整提交走持久双轨队列。 */
  steerMessage: (submission: api.TurnSubmission) => Promise<void>;
  followUpMessage: (submission: api.TurnSubmission) => Promise<void>;
  refreshQueue: (sessionId?: string) => Promise<void>;
  removeKernelQueued: (queueId: string) => Promise<void>;
  /** pi Alt+Up：弹出最新 pending follow-up 回编辑框；空轨返回 null。 */
  dequeueQueuedTurn: () => Promise<string | null>;
  draftReferences: ReferenceDraft[];
  draftScope: api.HistoryScope | null;
  draftRequestId: string;
  addDraftReference: (preview: api.ReferencePreview, selection: api.ReferenceSelection, allowFurtherReading: boolean) => void;
  removeDraftReference: (id: string) => void;
  setDraftScope: (scope: api.HistoryScope | null) => void;
  clearDraftContext: () => void;
  referenceViews: Record<string, api.ReferenceView | null>;
  loadReferenceView: (referenceId: string) => Promise<void>;
  /** 交付组列表（FilesPanel 汇总与聊天卡片共用同一份已提交数据）。 */
  deliverySets: api.DeliverySet[];
  deliverySetsPhase: Phase;
  /** 条目可用性：缺省即 unchecked；验证/预览/下载共享同一状态机。 */
  deliveryItemStates: Record<string, api.DeliveryItemState>;
  loadDeliverySets: () => Promise<void>;
  checkDeliveryItem: (item: api.Deliverable) => Promise<void>;
  previewDeliveryItem: (item: api.Deliverable) => Promise<void>;
  downloadDeliveryItem: (item: api.Deliverable) => Promise<void>;
  cancelDeliveryDownload: (itemId: string) => void;
  clearQueue: () => void;
  cancelCurrentRun: () => Promise<void>;
  openRun: (runId: string, sessionId: string) => Promise<void>;
  /** 按需拉取并缓存某个历史运行的事件（失败即静默回退到投影渲染）。 */
  loadRunLog: (runId: string) => Promise<void>;
  loadWork: (sessionId?: string) => Promise<void>;
  commitWork: (method: api.WorkMethod, fields?: Record<string, unknown>) => Promise<api.WorkCommitResult>;
  createGoal: (objective: string, maxRounds: number) => Promise<api.WorkCommitResult>;
  editGoal: (objective: string, maxRounds: number, goalRef: Pick<api.WorkGoal, 'id' | 'revision'>) => Promise<api.WorkCommitResult>;
  pauseGoal: (reason?: string) => Promise<api.WorkCommitResult>;
  resumeGoal: () => Promise<api.WorkCommitResult>;
  clearGoal: () => Promise<api.WorkCommitResult>;
  enterPlan: () => Promise<api.WorkCommitResult>;
  leavePlan: () => Promise<api.WorkCommitResult>;
  decidePlan: (action: api.PlanAction, feedback?: string, objective?: string, maxRounds?: number, submissionId?: string) => Promise<api.WorkCommitResult>;
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
  setFilesPanelOpen: (open: boolean) => void;
  setSessionDrawerOpen: (open: boolean) => void;
  loadTodos: (sessionId?: string) => Promise<void>;
  updateTodoStatus: (todoId: string, status: api.TodoStatus) => Promise<void>;
  setTodoPanelOpen: (open: boolean) => void;
  loadSettings: () => Promise<void>;
  saveSettings: (value: api.SettingsUpdate) => Promise<void>;
  saveLocale: (locale: api.Locale) => Promise<void>;
  loadSessionContext: (sessionId?: string) => Promise<void>;
  compactSession: (sessionId: string, instructions?: string) => Promise<api.CompactResult>;
  /** session/rewind 后重读消息；返回刷新后的可见视图。 */
  rewindSession: (sessionId: string, messageId: string) => Promise<api.Message[]>;
  /** session/fork 后刷新会话列表；返回新会话 id（导航由调用方做）。 */
  forkSession: (sessionId: string, messageId: string, title?: string) => Promise<string>;
  loadProviders: () => Promise<void>;
  saveProvider: (input: api.ProviderEntryInput) => Promise<void>;
  removeProvider: (id: string) => Promise<void>;
  refreshProvider: (input: api.ProviderRefreshInput) => Promise<api.ProviderEntry>;
  loadLifecycle: () => Promise<void>;
  createGeneration: (params: Parameters<typeof api.createGeneration>[0]) => Promise<void>;
  rejectGeneration: (id: string) => Promise<void>;
  startEval: (params: Parameters<typeof api.startEval>[0]) => Promise<void>;
  recordEval: (params: Parameters<typeof api.recordEval>[0]) => Promise<void>;
  promoteGeneration: (params: Parameters<typeof api.promoteGeneration>[0]) => Promise<void>;
}

let initialization: Promise<void> | null = null;
let subscription: RunSubscription | null = null;
let workSubscription: WorkSubscription | null = null;
let workSubscriptionSessionID: string | null = null;
let sessionEpoch = 0;
let workRead = 0;
let workEventSeq = 0;
let runOpen = 0;
let reviewEpoch = 0;
let queueRestoreSeq = 0;

/** steer/follow_up 应答分流：queued=true 走内核轨（steer 补乐观 user 气泡——
 *  HistoryModifier resume 不发 user 消息事件）；idle 回退已直接开新 run，
 *  接管并订阅（与 startRun 的成功块同构）。 */
function adoptQueueResult(sessionId: string, result: api.QueueTurnResult, submission: api.TurnSubmission): void {
  const state = useVivyStore.getState();
  if (state.activeSessionId !== sessionId) return;
  if (result.queued) {
    if (result.track === 'steer') {
      useVivyStore.setState((s) => ({ messages: [...s.messages, { id: `local-steer-${result.queue_id}`, run_id: s.currentRun?.id, role: 'user' as const, content: submission.text, created_at: Date.now() }] }));
    }
    return;
  }
  if (result.run_id && !runActive(state.currentRun)) {
    const run: api.Run = { id: result.run_id, session_id: sessionId, status: (result.status as api.RunStatus) ?? 'accepted', created_at: Date.now() };
    useVivyStore.setState((s) => ({
      currentRun: run, runEvents: [], streamingText: '', streamingReasoning: '', connection: 'connecting',
      messages: [...s.messages, { id: `local-${run.id}`, run_id: run.id, role: 'user' as const, content: submission.text, created_at: Date.now() }],
      backgroundRuns: [run, ...s.backgroundRuns.filter((item) => item.id !== run.id)],
    }));
    startSubscription(run.id, 0);
  }
}

/** 内核 settle-admission 解析（pi）：follow-up 在终态自动准入新 run，但
 *  admitted_run_id 与终态发布存在竞态，短暂轮询后放弃。命中时接管为
 *  currentRun 并订阅其事件流（与 TUI admitSettledCmd 同构）。 */
async function admitSettledRun(settlingRunId: string, attemptsLeft = 10): Promise<void> {
  const state = useVivyStore.getState();
  const sessionId = state.activeSessionId;
  if (!sessionId) return;
  try {
    const queue = await api.getQueueState(sessionId, settlingRunId);
    const current = useVivyStore.getState();
    if (current.activeSessionId !== sessionId || current.currentRun?.id !== settlingRunId) return;
    // 无可准入项（follow-up 车道空且无 admitted 记录）：立即让位本地 FIFO，
    // 不为零队列付出整段轮询延迟。
    if (!queue.admitted_run_id && (queue.follow_up?.length ?? 0) === 0) return;
    if (queue.admitted_run_id) {
      const run: api.Run = { id: queue.admitted_run_id, session_id: sessionId, status: 'active', created_at: Date.now() };
      useVivyStore.setState((s) => ({
        currentRun: run, kernelQueue: queue, runEvents: [], streamingText: '', streamingReasoning: '', connection: 'connecting',
        backgroundRuns: [run, ...s.backgroundRuns.filter((item) => item.id !== run.id)],
      }));
      startSubscription(run.id, 0);
      return;
    }
  } catch { /* 轮询失败继续重试直至上限 */ }
  if (attemptsLeft > 0) {
    await new Promise((resolve) => setTimeout(resolve, 200));
    await admitSettledRun(settlingRunId, attemptsLeft - 1);
  }
}
let settingsRead = 0;
let settingsMutation = 0;
let pendingSettingsMutation: number | null = null;

type SettingsOperation =
  | { kind: 'read'; id: number; mutation: number }
  | { kind: 'write'; mutation: number };

function beginSettingsRead(): SettingsOperation | null {
  if (pendingSettingsMutation !== null) return null;
  return { kind: 'read', id: ++settingsRead, mutation: settingsMutation };
}

function beginSettingsMutation(): SettingsOperation {
  const mutation = ++settingsMutation;
  pendingSettingsMutation = mutation;
  return { kind: 'write', mutation };
}

function finishSettingsMutation(operation: SettingsOperation): void {
  if (operation.kind === 'write' && pendingSettingsMutation === operation.mutation) pendingSettingsMutation = null;
}

function settingsOperationCurrent(operation: SettingsOperation | null): operation is SettingsOperation {
  if (!operation) return false;
  if (operation.kind === 'write') return operation.mutation === settingsMutation;
  return pendingSettingsMutation === null && operation.mutation === settingsMutation && operation.id === settingsRead;
}

function applyAuthoritativeSettings(result: api.Settings | api.LocaleSettings, operation: SettingsOperation | null): boolean {
  if (!settingsOperationCurrent(operation)) return false;
  hydrateLocale(result.locale);
  useVivyStore.setState((state) => ({
    settings: 'provider' in result ? result : state.settings ? { ...state.settings, ...result } : null,
    settingsPhase: 'ready',
    settingsError: null,
  }));
  return true;
}

function applySettingsError(error: unknown, operation: SettingsOperation | null): void {
  if (settingsOperationCurrent(operation)) useVivyStore.setState({ settingsPhase: 'error', settingsError: errorMessage(error) });
}

function stopSubscription(): void { subscription?.close(); subscription = null; }
function stopWorkSubscription(): void {
  workSubscription?.close();
  workSubscription = null;
  workSubscriptionSessionID = null;
  workEventSeq = 0;
}
function workRequestID(prefix: string): string {
  try { return `${prefix}-${crypto.randomUUID()}`; }
  catch { return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2)}`; }
}

async function refreshAfterTerminal(runId: string): Promise<void> {
  const state = useVivyStore.getState();
  const sessionId = state.activeSessionId;
  let messagesRefreshed = !sessionId;
  try {
    if (sessionId) await loadMessagesIntoStore(sessionId, sessionEpoch);
    messagesRefreshed = true;
  } catch (error) {
    if (useVivyStore.getState().currentRun?.id === runId && !useVivyStore.getState().runError) {
      useVivyStore.setState({ runError: t('errors.refreshAfterRunFailed', { error: errorMessage(error) }) });
    }
  }
  if (sessionId) void loadContextIntoStore(sessionId);
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
    if (event.type === 'run.failed') update.runError = runFailedMessage(event.payload) ?? t('errors.runFailedTitle');
    stopSubscription();
    // Follow-up admissions are owned by the kernel; adopt its settled run.
    void refreshAfterTerminal(event.run_id).then(() => admitSettledRun(event.run_id)).then(() => state.refreshQueue());
  } else if (event.type === 'turn.queued' || event.type === 'turn.dequeued' || event.type === 'turn.steered') {
    void state.refreshQueue();
    // abort-flush / clear 把队列文本还给用户：最新一条回填编辑框（pi）。
    if (event.type === 'turn.dequeued') {
      const reason = String(event.payload.reason ?? '');
      const text = String(event.payload.text ?? '');
      if ((reason === 'aborted' || reason === 'cleared') && text) {
        update.queueRestoreText = { text, seq: ++queueRestoreSeq };
      }
    }
  } else if (event.type.startsWith('child.')) void state.loadChildren(event.run_id);
  else if (event.type.includes('approval') || event.type.includes('question')) void state.loadReviews();
  else if (event.type === 'tool.finished' && isTaskToolName(event.payload.tool_name)) void state.loadTodos();
  else if (event.type === 'context.compacted' && state.activeSessionId) void loadContextIntoStore(state.activeSessionId);
  useVivyStore.setState(update);
}

function startSubscription(runId: string, afterSeq: number): void {
  stopSubscription();
  useVivyStore.setState({ connection: 'connecting', runError: null });
  subscription = subscribeRun(runId, afterSeq, handleRunEvent, (message) => {
    if (useVivyStore.getState().currentRun?.id === runId) useVivyStore.setState({ connection: 'reconnecting', runError: message });
  });
}

function handleWorkEvent(sessionId: string, epoch: number, event: api.WorkEvent): void {
  const state = useVivyStore.getState();
  if (epoch !== sessionEpoch || state.activeSessionId !== sessionId || event.seq <= workEventSeq) return;
  workEventSeq = event.seq;
  if (event.seq > (state.work?.version ?? 0)) {
    useVivyStore.setState({ work: null, workPhase: 'loading' });
    void state.loadWork(sessionId).then(() => {
      const current = useVivyStore.getState();
      if (epoch === sessionEpoch && current.activeSessionId === sessionId && current.workPhase === 'error') workSubscription?.retry();
    });
  }
}

function startWorkSubscription(sessionId: string, afterSeq: number): void {
  stopWorkSubscription();
  const epoch = sessionEpoch;
  workEventSeq = afterSeq;
  workSubscriptionSessionID = sessionId;
  workSubscription = subscribeWork(sessionId, afterSeq, (event) => handleWorkEvent(sessionId, epoch, event), (message) => {
    if (epoch === sessionEpoch && useVivyStore.getState().activeSessionId === sessionId) {
      useVivyStore.setState({ work: null, workError: message, workPhase: 'error' });
    }
  }, async () => {
    if (epoch === sessionEpoch && useVivyStore.getState().activeSessionId === sessionId) {
      await useVivyStore.getState().loadWork(sessionId);
      const state = useVivyStore.getState();
      if (state.activeSessionId === sessionId && state.workPhase === 'error') {
        throw new Error(state.workError ?? 'work refresh failed');
      }
    }
  }, async (processEpoch) => {
    const state = useVivyStore.getState();
    if (epoch !== sessionEpoch || state.activeSessionId !== sessionId) return;
    if (state.work && 'process_epoch' in state.work && state.work.process_epoch === processEpoch) return;
    useVivyStore.setState({ work: null, workPhase: 'loading' });
    await state.loadWork(sessionId);
    if (useVivyStore.getState().activeSessionId === sessionId && useVivyStore.getState().workPhase === 'error') {
      throw new Error(useVivyStore.getState().workError ?? 'work refresh failed');
    }
  });
}

async function loadWorkIntoStore(sessionId: string, epoch: number): Promise<api.WorkView | null> {
  const read = ++workRead;
  try {
    const work = await api.getSessionWork(sessionId);
    const state = useVivyStore.getState();
    if (read === workRead && epoch === sessionEpoch && state.activeSessionId === sessionId) {
      useVivyStore.setState({ work, workPhase: 'ready', workError: null });
      return work;
    }
    return null;
  } catch (error) {
    if (read === workRead && epoch === sessionEpoch && useVivyStore.getState().activeSessionId === sessionId) {
      useVivyStore.setState({ workPhase: 'error', workError: errorMessage(error) });
    }
    return null;
  }
}

async function loadMessagesIntoStore(sessionId: string, epoch: number): Promise<api.Message[]> {
  const result = await api.listMessages(sessionId);
  const state = useVivyStore.getState();
  if (epoch === sessionEpoch && state.activeSessionId === sessionId) useVivyStore.setState({ messages: result.messages, messagesPhase: result.messages.length ? 'ready' : 'empty', messagesError: null });
  return result.messages;
}

async function loadContextIntoStore(sessionId: string): Promise<void> {
  try {
    const context = await api.getSessionContext(sessionId);
    if (useVivyStore.getState().activeSessionId === sessionId) useVivyStore.setState({ sessionContext: context });
  } catch {
    // non-fatal: the chat ring falls back to hidden while the backend is
    // unreachable; the next run or terminal refresh retries.
  }
}

async function loadTodosIntoStore(sessionId: string, epoch: number): Promise<void> {
  const result = await api.listTodos(sessionId);
  const state = useVivyStore.getState();
  if (epoch !== sessionEpoch || state.activeSessionId !== sessionId) return;
  useVivyStore.setState({ todos: result.todos, todosPhase: result.todos.length ? 'ready' : 'empty', todosError: null });
}

export const useVivyStore = create<RuntimeState>((set, get) => ({
  initialized: false, initializationError: null, capabilities: [], codeModeAvailable: false, codeMode: false, connection: 'idle',
  sessions: [], sessionsPhase: 'idle', sessionsError: null, sessionBusyId: null, activeSessionId: null,
  messages: [], messagesPhase: 'idle', messagesError: null, sessionContext: null,
  todos: [], todosPhase: 'idle', todosError: null, todoPanelOpen: false,
  currentRun: null, runEvents: [], runLogs: {}, streamingText: '', streamingReasoning: '', runError: null, runBusy: false, kernelQueue: null, queueRestoreText: null,
  work: null, workPhase: 'idle', workError: null, workBusy: false,
  backgroundRuns: [], backgroundPhase: 'idle', backgroundError: null, backgroundBusyId: null,
  children: [], childrenPhase: 'idle', childrenError: null, childBusyId: null, selectedChild: null,
  reviews: [], reviewsPhase: 'idle', reviewsError: null, reviewBusyIds: [], reviewCenterOpen: false, filesPanelOpen: false, sessionDrawerOpen: false,
  settings: null, settingsPhase: 'idle', settingsError: null,
  providers: [], catalog: [], providersPhase: 'idle', providersError: null,
  species: null, generations: [], evals: [], promotions: [], lifecyclePhase: 'idle', lifecycleError: null, lifecycleBusy: false,

  initialize: async () => {
    if (initialization) return initialization;
    initialization = (async () => {
      set({ initializationError: null, connection: 'connecting' });
      try {
        const capabilities = await api.initialize();
        await api.recoverBackgroundRuns().catch(() => undefined);
        const settingsOperation = beginSettingsRead();
        const [sessions, background, settings, providers, species] = await Promise.all([
          api.listSessions(),
          api.listBackgroundRuns(),
          api.getSettings().catch(() => null),
          api.listProviders().catch(() => null),
          // Runtime identity is diagnostic metadata. An older/control-plane
          // endpoint that cannot provide it must not prevent the Face from
          // starting; the host then reports the identity as unavailable.
          Promise.resolve(api.inspectSpecies()).catch(() => null),
        ]);
        // 合并会话列表：去重合并用户在 initialize 等待期内可能已创建/更新的会话
        const inFlightSessions = get().sessions;
        const serverSessionIds = new Set(sessions.sessions.map((item) => item.id));
        const freshlyCreated = inFlightSessions.filter((item) => !serverSessionIds.has(item.id));
        const inFlightMap = new Map(inFlightSessions.map((item) => [item.id, item]));
        let sessionItems = [
          ...freshlyCreated,
          ...sessions.sessions.map((serverItem) => inFlightMap.get(serverItem.id) ?? serverItem),
        ];

        let initialSessionError: string | null = null;
        if (!sessionItems.length) {
          try { sessionItems = [await api.createSession('')]; }
          catch (error) { initialSessionError = errorMessage(error); }
        }

        // 尊重实时意图：若用户在加载等待期已显式创建/选中会话，坚决不回写覆盖
        const currentActive = get().activeSessionId;
        const hasValidActive = Boolean(currentActive && sessionItems.some((item) => item.id === currentActive));
        const saved = localStorage.getItem(ACTIVE_SESSION_KEY);
        const targetActiveId = hasValidActive
          ? currentActive
          : (sessionItems.some((item) => item.id === saved) ? saved : sessionItems[0]?.id ?? null);

        set({
          initialized: true,
          connection: 'connected',
          capabilities: capabilities.capabilities,
          codeModeAvailable: capabilities.code_mode_available === true,
          codeMode: false,
          sessions: sessionItems,
          sessionsPhase: initialSessionError ? 'error' : sessionItems.length ? 'ready' : 'empty',
          sessionsError: initialSessionError,
          backgroundRuns: background.runs,
          backgroundPhase: background.runs.length ? 'ready' : 'empty',
          providers: providers?.entries ?? [],
          catalog: providers?.catalog ?? [],
          providersPhase: providers ? 'ready' : 'error',
          species: species ?? null,
        });
        if (settings) applyAuthoritativeSettings(settings, settingsOperation);
        else if (settingsOperationCurrent(settingsOperation)) set({ settingsPhase: 'error' });

        if (!hasValidActive && targetActiveId) await get().selectSession(targetActiveId);
        void get().loadReviews();
      } catch (error) {
        set({ initialized: true, initializationError: errorMessage(error), connection: 'error' });
      }
    })();
    return initialization;
  },
  retryInitialize: async () => {
    if (!get().initialized) return;
    stopSubscription();
    stopWorkSubscription();
    resetRpcClient();
    initialization = null;
    sessionEpoch += 1;
    reviewEpoch += 1;
    set({
      initialized: false,
      initializationError: null,
      connection: 'connecting',
      codeModeAvailable: false,
      codeMode: false,
      sessions: [],
      sessionsPhase: 'idle',
      sessionsError: null,
      activeSessionId: null,
      species: null,
      messages: [],
      messagesPhase: 'idle',
      messagesError: null,
      sessionContext: null,
      todos: [],
      todosPhase: 'idle',
      todosError: null,
      providers: [],
      catalog: [],
      providersPhase: 'idle',
      providersError: null,
      currentRun: null,
      runEvents: [],
      streamingText: '',
      streamingReasoning: '',
      runError: null,
      work: null,
      workPhase: 'idle',
      workError: null,
      workBusy: false,
    });
    await get().initialize();
  },
  setCodeMode: (enabled) => set((state) => ({ codeMode: state.codeModeAvailable && enabled })),

  loadSessions: async () => {
    set((state) => ({ sessionsPhase: state.sessions.length ? 'refreshing' : 'loading', sessionsError: null }));
    try { const result = await api.listSessions(); set({ sessions: result.sessions, sessionsPhase: result.sessions.length ? 'ready' : 'empty' }); }
    catch (error) { set((state) => ({ sessionsPhase: state.sessions.length ? 'ready' : 'error', sessionsError: errorMessage(error) })); }
  },
  createSession: async (title = '', workspacePath = '') => {
    set({ sessionBusyId: 'create', sessionsError: null });
    try { const created = await api.createSession(title, workspacePath); set((state) => ({ sessions: [created, ...state.sessions], sessionsPhase: 'ready' })); await get().selectSession(created.id); return created; }
    catch (error) { set((state) => ({ sessionsError: errorMessage(error), sessionsPhase: state.sessions.length ? state.sessionsPhase : 'error' })); throw error; } finally { set({ sessionBusyId: null }); }
  },
	chooseWorkspace: async (workspacePath) => {
		const state = get();
		const activeId = state.activeSessionId;
		const active = state.sessions.find((session) => session.id === activeId);
		if (active && (active.workspace_path ?? '') === workspacePath) return active;
		if (!activeId || state.messages.length > 0 || runActive(state.currentRun)) {
			return get().createSession('', workspacePath);
		}
		set({ sessionBusyId: activeId, sessionsError: null });
		try {
			const updated = await api.setSessionWorkspace(activeId, workspacePath);
			set((current) => ({ sessions: current.sessions.map((session) => session.id === activeId ? updated : session) }));
			return updated;
		} catch (error) {
			if (error instanceof api.ApiError && error.status === 409) {
				return get().createSession('', workspacePath);
			}
			set({ sessionsError: errorMessage(error) });
			throw error;
		} finally {
			if (get().sessionBusyId === activeId) set({ sessionBusyId: null });
		}
	},
  renameSession: async (id, title) => {
    set({ sessionBusyId: id, sessionsError: null });
    try { const renamed = await api.renameSession(id, title); set((state) => ({ sessions: state.sessions.map((item) => item.id === id ? renamed : item) })); }
    catch (error) { set({ sessionsError: errorMessage(error) }); throw error; } finally { set({ sessionBusyId: null }); }
  },
  setSessionPermission: async (id, preset) => {
    set({ sessionBusyId: id, sessionsError: null });
    try {
      const updated = await api.setSessionPermission(id, preset);
      set((state) => ({ sessions: state.sessions.map((item) => item.id === id ? { ...item, ...updated } : item) }));
    } catch (error) { set({ sessionsError: errorMessage(error) }); throw error; } finally { set({ sessionBusyId: null }); }
  },
  deleteSession: async (id) => {
    set({ sessionBusyId: id, sessionsError: null });
    try {
      await api.deleteSession(id);
      const remaining = get().sessions.filter((item) => item.id !== id);
      set({ sessions: remaining, sessionsPhase: remaining.length ? 'ready' : 'empty' });
      if (get().activeSessionId === id) {
        stopSubscription(); stopWorkSubscription(); localStorage.removeItem(ACTIVE_SESSION_KEY);
        set({ activeSessionId: null, messages: [], sessionContext: null, todos: [], todosPhase: 'idle', todosError: null, currentRun: null, runEvents: [], kernelQueue: null, queueRestoreText: null, children: [], selectedChild: null, work: null, workPhase: 'idle', workError: null, draftReferences: [], draftScope: null, draftRequestId: newDraftRequestId(), referenceViews: {}, deliverySets: [], deliverySetsPhase: 'idle', deliveryItemStates: {} });
        if (remaining[0]) await get().selectSession(remaining[0].id);
        else await get().createSession();
      }
    } catch (error) { set({ sessionsError: errorMessage(error) }); throw error; } finally { set({ sessionBusyId: null }); }
  },
  selectSession: async (id) => {
    const epoch = ++sessionEpoch;
    runOpen += 1;
    stopSubscription(); stopWorkSubscription(); localStorage.setItem(ACTIVE_SESSION_KEY, id);
    set({ activeSessionId: id, messages: [], messagesPhase: 'loading', messagesError: null, sessionContext: null, todos: [], todosPhase: 'loading', todosError: null, currentRun: null, runEvents: [], runLogs: {}, streamingText: '', streamingReasoning: '', runError: null, kernelQueue: null, queueRestoreText: null, children: [], selectedChild: null, work: null, workPhase: 'loading', workError: null, draftReferences: [], draftScope: null, draftRequestId: newDraftRequestId(), referenceViews: {}, deliverySets: [], deliverySetsPhase: 'idle', deliveryItemStates: {} });
    try {
      const [messages] = await Promise.all([loadMessagesIntoStore(id, epoch), loadTodosIntoStore(id, epoch).catch((error) => {
        if (epoch === sessionEpoch && get().activeSessionId === id) set({ todosPhase: get().todos.length ? 'ready' : 'error', todosError: errorMessage(error) });
      }), loadWorkIntoStore(id, epoch)]);
      void loadContextIntoStore(id);
      void get().refreshQueue(id);
      const work = get().work;
      if (work && epoch === sessionEpoch && get().activeSessionId === id) startWorkSubscription(id, work.version);
      const background = await api.listBackgroundRuns();
      if (epoch !== sessionEpoch || get().activeSessionId !== id) return;
      set({ backgroundRuns: background.runs, backgroundPhase: background.runs.length ? 'ready' : 'empty' });
      const runId = get().work?.current_run_id || background.runs.filter((run) => run.session_id === id).sort((a, b) => b.created_at - a.created_at)[0]?.id || lastRunId(messages);
      if (runId && get().currentRun?.id !== runId) await get().openRun(runId, id);
      // 最近的两个更早运行按需回放事件，让前几轮也按工具/思考行渲染。
      const older = recentRunIds(messages, 3).filter((candidate) => candidate !== runId);
      await Promise.all(older.map((candidate) => get().loadRunLog(candidate)));
    } catch (error) { if (epoch === sessionEpoch) set({ messagesPhase: 'error', messagesError: errorMessage(error) }); }
  },
  loadWork: async (sessionId = get().activeSessionId ?? undefined) => {
    if (!sessionId) return;
    const epoch = sessionEpoch;
    set({ workPhase: get().work ? 'refreshing' : 'loading', workError: null });
    const work = await loadWorkIntoStore(sessionId, epoch);
    if (work?.current_run_id && get().currentRun?.id !== work.current_run_id) {
      await get().openRun(work.current_run_id, sessionId);
    }
    if (work && epoch === sessionEpoch && get().activeSessionId === sessionId
      && (workSubscription === null || workSubscriptionSessionID !== sessionId)) {
      startWorkSubscription(sessionId, work.version);
    }
  },
  commitWork: async (method, fields = {}) => {
    const epoch = sessionEpoch;
    const state = get();
    const sessionId = state.activeSessionId;
    const work = state.work;
    if (!sessionId || !work) throw new Error(t('workControl.unavailable'));
    set({ workBusy: true, workError: null });
    try {
      const result = await api.commitWork(method, {
        session_id: sessionId,
        expected_version: work.version,
        request_id: workRequestID(method.replace('/', '-')),
        ...fields,
      });
      if (epoch === sessionEpoch && get().activeSessionId === sessionId) {
        // Automatic rounds may publish newer Work before the mutation's
        // response reaches this peer. Never roll those facts back.
        const latest = get().work;
        set({ work: latest && latest.version > result.work.version ? latest : result.work, workPhase: 'ready', workError: null });
      }
      return result;
    } catch (error) {
      if (epoch === sessionEpoch && get().activeSessionId === sessionId) set({ workError: errorMessage(error) });
      throw error;
    } finally {
      if (epoch === sessionEpoch && get().activeSessionId === sessionId) set({ workBusy: false });
    }
  },
  createGoal: (objective, maxRounds) => get().commitWork('goal/create', { objective, max_rounds: maxRounds }),
  editGoal: (objective, maxRounds, goalRef) => get().commitWork('goal/edit', {
    goal_id: goalRef.id, goal_revision: goalRef.revision, objective, max_rounds: maxRounds,
  }),
  pauseGoal: (reason = '') => {
    const goal = get().work?.goal;
    if (!goal) return Promise.reject(new Error(t('workControl.noGoal')));
    return get().commitWork('goal/pause', { goal_id: goal.id, goal_revision: goal.revision, reason });
  },
  resumeGoal: () => {
    const goal = get().work?.goal;
    if (!goal) return Promise.reject(new Error(t('workControl.noGoal')));
    return get().commitWork('goal/resume', { goal_id: goal.id, goal_revision: goal.revision });
  },
  clearGoal: () => {
    const goal = get().work?.goal;
    if (!goal) return Promise.reject(new Error(t('workControl.noGoal')));
    return get().commitWork('goal/clear', { goal_id: goal.id, goal_revision: goal.revision });
  },
  enterPlan: () => get().commitWork('plan/enter'),
  leavePlan: () => get().commitWork('plan/leave'),
  decidePlan: (action, feedback = '', objective = '', maxRounds = 0, exactSubmissionID) => {
    const submissionID = exactSubmissionID ?? get().work?.plan.submission_id;
    if (!submissionID || get().work?.plan.submission_id !== submissionID || get().work?.plan.review_status !== 'pending') {
      const error = new Error(t('workControl.reviewStale'));
      set({ workError: error.message });
      return Promise.reject(error);
    }
    const fields: Record<string, unknown> = { submission_id: submissionID, action, feedback };
    if (action === 'start_goal') {
      fields.objective = objective;
      fields.max_rounds = maxRounds;
    }
    return get().commitWork('plan/decide', fields);
  },
  loadTodos: async (sessionId = get().activeSessionId ?? undefined) => {
    if (!sessionId) return;
    const epoch = sessionEpoch;
    set((state) => ({ todosPhase: state.todos.length ? 'refreshing' : 'loading', todosError: null }));
    try { await loadTodosIntoStore(sessionId, epoch); }
    catch (error) { if (epoch === sessionEpoch && get().activeSessionId === sessionId) set((state) => ({ todosPhase: state.todos.length ? 'ready' : 'error', todosError: errorMessage(error) })); }
  },
  updateTodoStatus: async (todoId: string, status: api.TodoStatus) => {
    const state = get();
    const sessionId = state.activeSessionId;
    if (!sessionId) return;
    if (runActive(state.currentRun)) return;

    const previousTodos = state.todos;
    const target = previousTodos.find((item) => item.id === todoId);
    if (!target || target.status === status) return;

    const nextTodos = previousTodos.map((item) =>
      item.id === todoId ? { ...item, status, updated_at: Date.now() } : item,
    );
    set({ todos: nextTodos, todosError: null });

    try {
      const result = await api.updateTodo(sessionId, todoId, status);
      if (result?.todo) {
        set((curr) => ({
          todos: curr.todos.map((item) => (item.id === todoId ? result.todo : item)),
        }));
      }
    } catch (error) {
      set({ todos: previousTodos, todosError: errorMessage(error) });
    }
  },
  setTodoPanelOpen: (open) => set({ todoPanelOpen: open }),
  openRun: async (runId, sessionId) => {
    const request = ++runOpen;
    const epoch = sessionEpoch;
    try {
      const [run, log, children] = await Promise.all([api.getRun(runId), api.getRunLog(runId), api.listChildren(runId, true)]);
      if (request !== runOpen || epoch !== sessionEpoch || get().activeSessionId !== sessionId) return;
      const events = log.events.sort((a, b) => a.seq - b.seq);
      const active = runActive(run);
      const failed = !active && run.status === 'failed'
        ? runFailedMessage([...events].reverse().find((event) => event.type === 'run.failed')?.payload) ?? t('errors.runFailedTitle')
        : null;
      // 切换运行前先把上一轮的事件留在缓存里，否则它下一轮就会退回投影渲染。
      const previous = get();
      const runLogs = previous.currentRun && previous.currentRun.id !== runId && previous.runEvents.length > 0
        ? withCachedRunLog(previous.runLogs, previous.currentRun.id, previous.runEvents)
        : previous.runLogs;
      set({ currentRun: run, runEvents: events, runLogs, streamingText: active ? replay(events, 'model.delta') : '', streamingReasoning: active ? replay(events, 'model.reasoning_delta') : '', children: children.children, childrenPhase: children.children.length ? 'ready' : 'empty', connection: active ? 'connecting' : 'connected', runError: failed, selectedChild: null });
      if (active) startSubscription(runId, events.reduce((max, event) => Math.max(max, event.seq), 0));
      else if (run.status === 'completed') void admitSettledRun(runId);
    } catch (error) { if (request === runOpen && epoch === sessionEpoch && get().activeSessionId === sessionId) set({ runError: errorMessage(error) }); }
  },
  loadRunLog: async (runId) => {
    if (runId === '' || get().runLogs[runId] !== undefined) return;
    try {
      const log = await api.getRunLog(runId);
      const events = log.events.sort((a, b) => a.seq - b.seq);
      set((state) => ({ runLogs: withCachedRunLog(state.runLogs, runId, events) }));
    } catch { /* 历史事件不可得（已删除/清理）：该运行保持投影渲染 */ }
  },
  startRun: async (sessionId, submission) => {
    if (get().activeSessionId !== sessionId) {
      const message = t('errors.sessionMismatch');
      set({ runError: message });
      throw new Error(message);
    }
    // 运行中改为入队（对照 Crush），不再静默丢弃。
    if (runActive(get().currentRun) || get().runBusy) { await get().followUpMessage(submission); return; }
    set({ runBusy: true, runError: null });
    const { text, attachments } = submission;
    try {
      const result = await api.startTurn(sessionId, submission);
      if (get().activeSessionId !== sessionId) { await get().loadBackgroundRuns(); return; }
      const run: api.Run = { id: result.run_id, session_id: sessionId, status: result.status, created_at: Date.now() };
      const localAttachments: api.MessageAttachment[] | undefined = attachments?.map((item) => ({ name: item.name, mime_type: item.mime_type, data_url: `data:${item.mime_type};base64,${item.data}` }));
      // 新一轮开始前把上一轮的事件留在缓存里：它的工具/思考行不会因为切换而消失。
      const previous = get();
      const runLogs = previous.currentRun && previous.runEvents.length > 0
        ? withCachedRunLog(previous.runLogs, previous.currentRun.id, previous.runEvents)
        : previous.runLogs;
      set((state) => ({ currentRun: run, runEvents: [], runLogs, streamingText: '', streamingReasoning: '', connection: 'connecting', messages: [...state.messages, { id: `local-${run.id}`, run_id: run.id, role: 'user', content: text, created_at: Date.now(), attachments: localAttachments }], messagesPhase: 'ready', backgroundRuns: [run, ...state.backgroundRuns.filter((item) => item.id !== run.id)], backgroundPhase: 'ready' }));
      startSubscription(run.id, 0);
    } catch (error) { set({ runError: errorMessage(error) }); throw error; } finally { set({ runBusy: false }); }
  },
	editSession: async (sessionId, messageId, text, mode = 'normal', face?: api.Face, thinking?: api.ThinkingMode) => {
		if (get().activeSessionId !== sessionId) {
			const message = t('errors.sessionMismatch');
			set({ runError: message });
			throw new Error(message);
		}
		if (runActive(get().currentRun) || get().runBusy) return;
		set({ runBusy: true, runError: null });
		try {
			const result = await api.editSession(sessionId, messageId, text, mode, face, thinking);
			if (get().activeSessionId !== sessionId) { await get().loadBackgroundRuns(); return; }
			const run: api.Run = { id: result.run_id, session_id: sessionId, status: result.status, created_at: Date.now() };
			await loadMessagesIntoStore(sessionId, sessionEpoch);
			const previous = get();
			const runLogs = previous.currentRun && previous.runEvents.length > 0
				? withCachedRunLog(previous.runLogs, previous.currentRun.id, previous.runEvents)
				: previous.runLogs;
			set((state) => ({ currentRun: run, runEvents: [], runLogs, streamingText: '', streamingReasoning: '', connection: 'connecting', backgroundRuns: [run, ...state.backgroundRuns.filter((item) => item.id !== run.id)], backgroundPhase: 'ready' }));
			startSubscription(run.id, 0);
		} catch (error) { set({ runError: errorMessage(error) }); throw error; }
		finally { set({ runBusy: false }); }
	},
  enqueueMessage: (submission) => get().followUpMessage(copySubmission(submission)),
  // 完整提交由内核统一排队。
  steerMessage: async (submission) => {
    const sessionId = get().activeSessionId;
    if (!sessionId) return;
    try {
      adoptQueueResult(sessionId, await api.steerTurn(sessionId, copySubmission(submission)), submission);
      void get().refreshQueue();
    } catch (error) { set({ runError: errorMessage(error) }); throw error; }
  },
  followUpMessage: async (submission) => {
    const sessionId = get().activeSessionId;
    if (!sessionId) return;
    try {
      adoptQueueResult(sessionId, await api.followUpTurn(sessionId, copySubmission(submission)), submission);
      void get().refreshQueue();
    } catch (error) { set({ runError: errorMessage(error) }); throw error; }
  },
  refreshQueue: async (sessionId = get().activeSessionId ?? undefined) => {
    if (!sessionId) return;
    try {
      const queue = await api.getQueueState(sessionId);
      if (get().activeSessionId === sessionId) set({ kernelQueue: queue });
    } catch { /* 队列视图非关键：静默重试于下一事件 */ }
  },
  removeKernelQueued: async (queueId) => {
    const sessionId = get().activeSessionId;
    if (!sessionId) return;
    try { await api.removeQueuedTurn(sessionId, queueId); } catch (error) { set({ runError: errorMessage(error) }); throw error; }
    void get().refreshQueue();
  },
  dequeueQueuedTurn: async () => {
    const sessionId = get().activeSessionId;
    if (!sessionId) return null;
    try {
      const result = await api.dequeueQueuedTurn(sessionId);
      void get().refreshQueue();
      return result.dequeued ? (result.text ?? null) : null;
    } catch { return null; }
  },
  draftReferences: [],
  draftScope: null,
  draftRequestId: newDraftRequestId(),
  addDraftReference: (preview, selection, allowFurtherReading) => set((state) => {
    const draft: ReferenceDraft = { id: `ref_${++draftSeq}`, preview, selection, allowFurther: allowFurtherReading };
    const scope = state.draftScope ?? {};
    const source = selection.selection.source_session_id;
    const ids = scope.session_ids ? [...scope.session_ids] : [];
    if (allowFurtherReading && !ids.includes(source)) ids.push(source);
    return {
      draftReferences: [...state.draftReferences, draft],
      draftScope: allowFurtherReading ? { ...scope, session_ids: ids } : scope,
    };
  }),
  removeDraftReference: (id) => set((state) => {
    const removed = state.draftReferences.find((item) => item.id === id);
    const remaining = state.draftReferences.filter((item) => item.id !== id);
    let scope = state.draftScope;
    // 其 further-reading scope 项随引用一起移除（无其他引用仍需该源会话时）。
    if (removed?.allowFurther && scope?.session_ids) {
      const source = removed.selection.selection.source_session_id;
      const stillNeeded = remaining.some((item) => item.allowFurther && item.selection.selection.source_session_id === source);
      if (!stillNeeded) {
        const ids = scope.session_ids.filter((session) => session !== source);
        scope = { ...scope, session_ids: ids.length ? ids : undefined };
        if (!scope.session_ids && !scope.workspace) scope = null;
      }
    }
    return { draftReferences: remaining, draftScope: scope };
  }),
  setDraftScope: (scope) => set({ draftScope: scope }),
  clearDraftContext: () => set({ draftReferences: [], draftScope: null, draftRequestId: newDraftRequestId() }),
  referenceViews: {},
  deliverySets: [],
  deliverySetsPhase: 'idle',
  deliveryItemStates: {},
  loadDeliverySets: async () => {
    const sessionId = get().activeSessionId;
    if (!sessionId || get().deliverySetsPhase === 'loading') return;
    set({ deliverySetsPhase: 'loading' });
    try {
      const items: api.DeliverySet[] = [];
      const seen = new Set<string>();
      let cursor: string | undefined;
      do {
        const page = await api.deliverablesList(sessionId, { cursor, limit: 100 });
        for (const set_ of page.items) {
          if (seen.has(set_.id)) continue;
          seen.add(set_.id);
          items.push(set_);
        }
        cursor = page.next_cursor === '' ? undefined : page.next_cursor;
      } while (cursor !== undefined);
      set({ deliverySets: items, deliverySetsPhase: items.length === 0 ? 'empty' : 'ready' });
    } catch {
      set({ deliverySetsPhase: 'error' });
    }
  },
  checkDeliveryItem: async (item) => {
    const sessionId = get().activeSessionId;
    const current = get().deliveryItemStates[item.id];
    if (!sessionId || current?.status === 'checking' || current?.status === 'downloading') return;
    set((state) => ({ deliveryItemStates: { ...state.deliveryItemStates, [item.id]: { status: 'checking' } } }));
    try {
      await checkDeliverable(item, sessionId, deliverableRpc);
      set((state) => ({ deliveryItemStates: { ...state.deliveryItemStates, [item.id]: { ...state.deliveryItemStates[item.id], status: 'available' } } }));
    } catch (err) {
      set((state) => ({ deliveryItemStates: { ...state.deliveryItemStates, [item.id]: { ...state.deliveryItemStates[item.id], status: deliveryErrorStatus(err) } } }));
    }
  },
  previewDeliveryItem: async (item) => {
    const sessionId = get().activeSessionId;
    if (!sessionId || get().deliveryItemStates[item.id]?.status === 'checking') return;
    try {
      const preview = await readDeliveryPreview(item, sessionId, deliverableRpc);
      set((state) => ({ deliveryItemStates: { ...state.deliveryItemStates, [item.id]: { ...state.deliveryItemStates[item.id], status: 'available', preview } } }));
    } catch (err) {
      set((state) => ({ deliveryItemStates: { ...state.deliveryItemStates, [item.id]: { ...state.deliveryItemStates[item.id], status: deliveryErrorStatus(err) } } }));
    }
  },
  downloadDeliveryItem: async (item) => {
    const sessionId = get().activeSessionId;
    if (!sessionId || deliveryDownloads.has(item.id)) return;
    const controller = new AbortController();
    deliveryDownloads.set(item.id, controller);
    set((state) => ({ deliveryItemStates: { ...state.deliveryItemStates, [item.id]: { ...state.deliveryItemStates[item.id], status: 'downloading' } } }));
    try {
      await downloadDeliverable(item, sessionId, deliverableRpc, controller.signal);
      set((state) => ({ deliveryItemStates: { ...state.deliveryItemStates, [item.id]: { ...state.deliveryItemStates[item.id], status: 'downloaded' } } }));
    } catch (err) {
      const aborted = err instanceof DeliverableTransferError && err.reason === 'aborted';
      set((state) => ({ deliveryItemStates: { ...state.deliveryItemStates, [item.id]: { ...state.deliveryItemStates[item.id], status: aborted ? 'unchecked' : deliveryErrorStatus(err) } } }));
    } finally {
      deliveryDownloads.delete(item.id);
    }
  },
  cancelDeliveryDownload: (itemId) => {
    deliveryDownloads.get(itemId)?.abort();
  },
  // 已提交引用的活状态按 id 缓存；失败记为 null，快照本身仍可读。
  loadReferenceView: async (referenceId) => {
    const sessionId = get().activeSessionId;
    if (!sessionId || get().referenceViews[referenceId] !== undefined) return;
    try {
      const view = await api.referenceGet(sessionId, referenceId);
      set((state) => state.referenceViews[referenceId] === undefined ? { referenceViews: { ...state.referenceViews, [referenceId]: view } } : {});
    } catch {
      set((state) => state.referenceViews[referenceId] === undefined ? { referenceViews: { ...state.referenceViews, [referenceId]: null } } : {});
    }
  },
  clearQueue: () => {
    const sessionId = get().activeSessionId;
    if (sessionId) void api.clearSessionQueue(sessionId).then(() => get().refreshQueue()).catch((error) => set({ runError: errorMessage(error) }));
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
    try {
      await api.startChild({ parent_run_id: parent.id, text, policy_profile: policyProfile || undefined, tool_names: toolNames?.length ? toolNames : undefined });
      await get().loadChildren(parent.id);
    }
    catch (error) { set({ childrenError: errorMessage(error) }); throw error; } finally { set({ childBusyId: null }); }
  },
  openChild: async (runId) => {
    set({ childBusyId: runId });
    try {
      const child = await api.getChild(runId);
      set({ selectedChild: child });
    } catch (error) { set({ childrenError: errorMessage(error) }); }
    finally { set({ childBusyId: null }); }
  },
  waitChild: async (runId) => { set({ childBusyId: runId }); try { set({ selectedChild: await api.waitChild(runId) }); await get().loadChildren(); } catch (error) { set({ childrenError: errorMessage(error) }); } finally { set({ childBusyId: null }); } },
  cancelChild: async (runId) => { set({ childBusyId: runId }); try { set({ selectedChild: await api.cancelChild(runId) }); await get().loadChildren(); } catch (error) { set({ childrenError: errorMessage(error) }); } finally { set({ childBusyId: null }); } },
  loadReviews: async () => {
    const epoch = ++reviewEpoch; set({ reviewsPhase: get().reviews.length ? 'refreshing' : 'loading', reviewsError: null });
    try { const result = await api.listReviews({ limit: 100 }); if (epoch === reviewEpoch) set({ reviews: result.reviews, reviewsPhase: result.reviews.length ? 'ready' : 'empty' }); }
    catch (error) { if (epoch === reviewEpoch) set({ reviewsPhase: 'error', reviewsError: errorMessage(error) }); }
  },
  respondReview: async (id, response) => {
    if (get().reviewBusyIds.includes(id)) return;
    set((state) => ({ reviewBusyIds: [...state.reviewBusyIds, id], reviewsError: null }));
    try {
      await api.respondReview(id, response);
      const status: api.ReviewStatus = response.action === 'approve' ? 'approved' : response.action === 'deny' ? 'denied' : response.action === 'answer' ? 'answered' : 'cancelled';
      set((state) => ({ reviews: state.reviews.map((review) => review.id === id ? { ...review, status } : review), reviewsPhase: 'ready' }));
      await get().loadReviews();
    }
    catch (error) { set({ reviewsError: errorMessage(error) }); throw error; } finally { set((state) => ({ reviewBusyIds: state.reviewBusyIds.filter((busy) => busy !== id) })); }
  },
  setReviewCenterOpen: (open) => set({ reviewCenterOpen: open }),
  setFilesPanelOpen: (open) => set({ filesPanelOpen: open }),
  setSessionDrawerOpen: (open) => set({ sessionDrawerOpen: open }),
  loadSettings: async () => {
    const operation = beginSettingsRead();
    if (!operation) return;
    set({ settingsPhase: 'loading', settingsError: null });
    try { applyAuthoritativeSettings(await api.getSettings(), operation); }
    catch (error) { applySettingsError(error, operation); }
  },
  saveSettings: async (value) => {
    const operation = beginSettingsMutation();
    set({ settingsPhase: 'processing', settingsError: null });
    try { applyAuthoritativeSettings(await api.updateSettings(value), operation); }
    catch (error) { applySettingsError(error, operation); throw error; }
    finally { finishSettingsMutation(operation); }
  },
  saveLocale: async (locale) => {
    const operation = beginSettingsMutation();
    set({ settingsPhase: 'processing', settingsError: null });
    try {
      const result = await api.updateLocale(locale);
      applyAuthoritativeSettings(result, operation);
    } catch (error) {
      applySettingsError(error, operation);
      throw error;
    } finally { finishSettingsMutation(operation); }
  },
  loadSessionContext: async (sessionId = get().activeSessionId ?? undefined) => { if (sessionId) await loadContextIntoStore(sessionId); },
  compactSession: async (sessionId, instructions) => {
    const result = await api.compactSession(sessionId, instructions);
    await loadContextIntoStore(sessionId);
    return result;
  },
  rewindSession: async (sessionId, messageId) => {
    await api.rewindSession(sessionId, messageId);
    void loadContextIntoStore(sessionId);
    return loadMessagesIntoStore(sessionId, sessionEpoch);
  },
  forkSession: async (sessionId, messageId, title) => {
    const result = await api.forkSession(sessionId, messageId, title);
    await get().loadSessions();
    return result.session_id;
  },
  // 目录与注册表来自同一次 settings/providers：两者一起写入，目录到达前
  // providersPhase 停在 idle/loading，设置页据此渲染 loading。
  loadProviders: async () => { set({ providersPhase: 'loading', providersError: null }); try { const view = await api.listProviders(); set({ providers: view.entries, catalog: view.catalog ?? [], providersPhase: 'ready' }); } catch (error) { set({ providersPhase: 'error', providersError: errorMessage(error) }); } },
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
  refreshProvider: async (input) => {
    set({ providersPhase: 'processing', providersError: null });
    try {
      const saved = await api.refreshProviderModels(input);
      set({ providersPhase: 'ready' });
      await get().loadProviders();
      return saved;
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

/** Reconcile an admission-changing RPC before using the local queue gate. */
export async function reconcileRun(runId: string, sessionId: string): Promise<void> {
  const epoch = sessionEpoch;
  const run = await api.getRun(runId);
  const state = useVivyStore.getState();
  if (epoch !== sessionEpoch || state.activeSessionId !== sessionId || state.currentRun?.id !== runId) return;
  // A terminal notification may win this read; never restore an active
  // snapshot over it. Keep the existing transcript and subscription intact.
  if (!runActive(state.currentRun)) return;
  useVivyStore.setState({ currentRun: run, backgroundRuns: state.backgroundRuns.map((item) => item.id === runId ? run : item) });
}

export function resetStoreForTests(): void {
  stopSubscription(); stopWorkSubscription(); initialization = null; sessionEpoch = 0; reviewEpoch = 0;
  workRead = 0; workEventSeq = 0;
  settingsRead = 0; settingsMutation = 0; pendingSettingsMutation = null;
}
