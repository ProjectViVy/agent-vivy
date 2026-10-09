/**
 * VIVY host bridge for the workflow editor Module.
 *
 * Ordinary `inofy.*` calls are bound to the active host session (the backend
 * handlers are session-scoped and fail closed without it). Start calls use a
 * prepared immutable request that captures session, parent Run, source,
 * input and one idempotency key before the first send; retries forward that
 * request without consulting mutable host state.
 *
 * The journal is the sole event authority: `subscribe("inofy.events")`
 * multiplexes onto `run/subscribe` + `run/event` notifications and pages from
 * `inofy.events` are normalized through the same journal->engine vocabulary
 * mapping, so the editor renders committed host facts only.
 */
import type { FaceClientRPC, FaceClientStore, FaceStoreState } from '@vivy/ui-sdk';
import type { JsonValue, RunEvent } from './studio/schema';
import { TransportError, type WorkflowRunIntent, type WorkflowStartRequest } from './studio/transport';

/**
 * The seam between the editor client and the host face. `call` routes
 * `inofy.*` host actions; `subscribe` supplies the `inofy.events` live
 * channel. Implemented by FaceBridge against the real face and by test
 * doubles in unit tests.
 */
export interface HostBridge {
  call<T>(method: string, params?: unknown): Promise<T>;
  prepareStartRun(intent: WorkflowRunIntent): WorkflowStartRequest;
  subscribe(
    channel: string,
    params: Record<string, unknown> | undefined,
    onEvent: (data: unknown) => void,
    onError?: (err: unknown) => void,
  ): { close(): void };
}

interface JournalRow {
  seq: number;
  type?: string;
  kind?: string;
  at?: number;
  created_at?: number;
  data?: Record<string, unknown>;
  payload?: Record<string, unknown>;
}

const JOURNAL_TO_ENGINE_KIND: Record<string, string> = {
  'workflow.admitted': 'run_admitted',
  'workflow.started': 'run_started',
  'workflow.waiting': 'run_waiting',
  'workflow.resumed': 'run_resumed',
  'workflow.recovery_required': 'run_recovery_required',
  'workflow.node.started': 'node_started',
  'workflow.node.attempt': 'node_attempt',
  'workflow.node.completed': 'node_completed',
  'workflow.node.failed': 'node_failed',
  'workflow.node.waiting': 'node_wait',
  'workflow.node.degraded': 'node_degraded',
  'workflow.switch.decision': 'switch_decision',
  'workflow.repeat.iteration': 'repeat_iteration',
  'run.completed': 'run_succeeded',
  'run.failed': 'run_failed',
  'run.cancelled': 'run_cancelled',
};

const TERMINAL_JOURNAL = new Set(['run.completed', 'run.failed', 'run.cancelled']);

/** journal row ({seq,type,at,data}) or notification ({seq,type,payload}) -> editor RunEvent. */
export function normalizeJournalEvent(raw: JournalRow): RunEvent {
  const type = raw.type ?? raw.kind ?? '';
  const data = raw.data ?? raw.payload ?? {};
  const path = typeof data.node_key === 'string' ? data.node_key : undefined;
  const attempt = typeof data.attempt === 'number' ? data.attempt : undefined;
  return { seq: raw.seq, kind: JOURNAL_TO_ENGINE_KIND[type] ?? type, path, attempt, data: data as RunEvent['data'] };
}

const MAX_OPERATION_ID_BYTES = 128;

function snapshotJson(value: JsonValue): JsonValue {
  if (Array.isArray(value)) return Object.freeze(value.map((item) => snapshotJson(item))) as unknown as JsonValue;
  if (value !== null && typeof value === 'object') {
    const copy: Record<string, JsonValue> = {};
    for (const [key, item] of Object.entries(value)) copy[key] = snapshotJson(item);
    return Object.freeze(copy);
  }
  return value;
}

function validateStartRequest(request: unknown): asserts request is WorkflowStartRequest {
  if (request == null || typeof request !== 'object') throw new TypeError('prepared start request is required');
  const body = request as Record<string, unknown>;
  if (typeof body.workflow !== 'string' || body.workflow.length === 0) throw new TypeError('workflow is required');
  if (typeof body.session_id !== 'string' || body.session_id.length === 0) throw new TypeError('session_id is required');
  if (typeof body.parent_run_id !== 'string' || body.parent_run_id.length === 0) throw new TypeError('parent_run_id is required');
  if (typeof body.operation_id !== 'string' || body.operation_id.length === 0 || new TextEncoder().encode(body.operation_id).byteLength > MAX_OPERATION_ID_BYTES) {
    throw new TypeError(`operation_id must be nonempty and at most ${MAX_OPERATION_ID_BYTES} bytes`);
  }
  const hasRevision = body.revision !== undefined;
  const hasDraftETag = body.draft_etag !== undefined;
  if (hasRevision === hasDraftETag) throw new TypeError('exactly one start source (revision or draft_etag) is required');
  if (hasRevision && (!Number.isSafeInteger(body.revision) || (body.revision as number) <= 0)) throw new TypeError('revision must be a positive integer');
  if (hasDraftETag && (typeof body.draft_etag !== 'string' || body.draft_etag.length === 0)) throw new TypeError('draft_etag must be nonempty');
}

export class FaceBridge implements HostBridge {
  constructor(
    private rpc: FaceClientRPC,
    private store: FaceClientStore<FaceStoreState>,
  ) {}

  prepareStartRun(intent: WorkflowRunIntent): WorkflowStartRequest {
    if (typeof intent.workflow !== 'string' || intent.workflow.length === 0) throw new TypeError('workflow is required');
    const hasRevision = intent.revision !== undefined;
    const hasDraftETag = intent.draft_etag !== undefined;
    if (hasRevision === hasDraftETag) throw new TypeError('exactly one start source (revision or draft_etag) is required');
    if (hasRevision && (!Number.isSafeInteger(intent.revision) || intent.revision! <= 0)) throw new TypeError('revision must be a positive integer');
    if (hasDraftETag && (typeof intent.draft_etag !== 'string' || intent.draft_etag.length === 0)) throw new TypeError('draft_etag must be nonempty');
    const state = this.store.getState();
    const sessionId = state.activeSessionId;
    const parent = state.currentRun;
    if (!sessionId) throw new TypeError('an active session is required to start a workflow');
    if (!parent?.id) throw new TypeError('an active parent Run is required to start a workflow');
    if (parent.session_id !== sessionId) throw new TypeError('parent Run does not belong to the active session');
    const request: WorkflowStartRequest = {
      workflow: intent.workflow,
      ...(intent.revision !== undefined ? { revision: intent.revision } : {}),
      ...(intent.draft_etag !== undefined ? { draft_etag: intent.draft_etag } : {}),
      ...(intent.input !== undefined ? { input: snapshotJson(intent.input) } : {}),
      session_id: sessionId,
      parent_run_id: parent.id,
      operation_id: crypto.randomUUID(),
    };
    validateStartRequest(request);
    return Object.freeze(request);
  }

  call<T>(method: string, params?: unknown): Promise<T> {
    if (method === 'inofy.startRun') {
      validateStartRequest(params);
      return this.rpc.call<T>(method, params);
    }
    const state = this.store.getState();
    const body: Record<string, unknown> = { ...(params as Record<string, unknown> | undefined) };
    const sessionId = state.activeSessionId;
    if (sessionId) body.session_id = sessionId;
    else delete body.session_id;
    return this.rpc.call<T>(method, body);
  }

  /**
   * `inofy.events` live channel, served by `run/subscribe` + `run/event`.
   * Resumes from the last committed seq after stream errors or socket drops,
   * matching the shell's run-subscription semantics.
   */
  subscribe(
    channel: string,
    params: Record<string, unknown> | undefined,
    onEvent: (data: unknown) => void,
    onError?: (err: unknown) => void,
  ): { close(): void } {
    if (channel !== 'inofy.events') {
      onError?.(new TransportError(0, { code: 'unsupported_feature', message: `unknown channel ${channel}` }));
      return { close: () => undefined };
    }
    const runId = String(params?.run_id ?? '');
    let cursor = typeof params?.after === 'number' ? params.after : 0;
    let closed = false;
    let terminal = false;
    let subscriptionId = '';
    let removeEvent: (() => void) | undefined;
    let removeStreamError: (() => void) | undefined;
    let removeClose: (() => void) | undefined;
    let timer: ReturnType<typeof setTimeout> | undefined;

    const clearListeners = () => {
      removeEvent?.(); removeStreamError?.(); removeClose?.();
      removeEvent = undefined; removeStreamError = undefined; removeClose = undefined;
    };
    const unsubscribe = (id: string) => {
      if (id) void this.rpc.call('run/unsubscribe', { subscription_id: id }).catch(() => undefined);
    };
    const reconnect = () => {
      if (closed || timer !== undefined) return;
      timer = setTimeout(() => { timer = undefined; void connect(); }, 1000);
    };
    const close = () => {
      if (closed) return;
      closed = true;
      clearListeners();
      if (timer !== undefined) clearTimeout(timer);
      unsubscribe(subscriptionId);
      subscriptionId = '';
    };
    const connect = async () => {
      if (closed) return;
      clearListeners();
      subscriptionId = '';
      try {
        removeEvent = this.rpc.onNotification('run/event', (params) => {
          const envelope = params as { subscription_id?: string; event?: JournalRow };
          if (envelope.subscription_id !== subscriptionId || !envelope.event || envelope.event.seq <= cursor) return;
          const raw = envelope.event;
          cursor = raw.seq;
          onEvent(normalizeJournalEvent(raw));
          if (TERMINAL_JOURNAL.has(raw.type ?? '')) { terminal = true; close(); }
        });
        removeStreamError = this.rpc.onNotification('run/stream_error', (params) => {
          const envelope = params as { subscription_id?: string; message?: string };
          if (envelope.subscription_id !== subscriptionId) return;
          const failed = subscriptionId;
          subscriptionId = '';
          clearListeners();
          unsubscribe(failed);
          onError?.(envelope.message || 'run replay failed');
          reconnect();
        });
        removeClose = this.rpc.onClose(() => {
          clearListeners();
          reconnect();
        });
        const response = await this.rpc.call<{ subscription_id: string }>('run/subscribe', { run_id: runId, after_seq: cursor });
        if (closed) {
          unsubscribe(response.subscription_id);
          return;
        }
        subscriptionId = response.subscription_id;
      } catch (error) {
        clearListeners();
        if (closed || terminal) return;
        onError?.(error instanceof Error ? error.message : String(error));
        reconnect();
      }
    };
    void connect();
    return { close };
  }
}
