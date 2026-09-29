/**
 * VIVY host bridge for the vendored INOFY editor.
 *
 * Every `inofy.*` call is bound to the active host session (the backend
 * handlers are session-scoped and fail closed without it). `inofy.startRun`
 * additionally carries the run parentage the product contract requires:
 * `parent_run_id` is the host's current run and `operation_id` a per-call
 * idempotency key, the same model the Run Inspector uses for workflow/start.
 *
 * The journal is the sole event authority: `subscribe("inofy.events")`
 * multiplexes onto `run/subscribe` + `run/event` notifications and pages from
 * `inofy.events` are normalized through the same journal->engine vocabulary
 * mapping, so the editor renders committed host facts only.
 */
import type { FaceClientRPC, FaceClientStore, FaceStoreState } from '@vivy/ui-sdk';
import { VivyTransport, type HostBridge } from './studio/vivy-transport';
import type { RunEvent } from './studio/schema';
import { TransportError } from './studio/transport';

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

export class FaceBridge implements HostBridge {
  constructor(
    private rpc: FaceClientRPC,
    private store: FaceClientStore<FaceStoreState>,
  ) {}

  call<T>(method: string, params?: unknown): Promise<T> {
    const state = this.store.getState();
    const body: Record<string, unknown> = { ...(params as Record<string, unknown> | undefined) };
    const sessionId = state.activeSessionId;
    if (sessionId) body.session_id = sessionId;
    else delete body.session_id;
    if (method === 'inofy.startRun') {
      const parent = state.currentRun;
      if (parent?.id) body.parent_run_id = parent.id;
      if (typeof body.operation_id !== 'string' || !body.operation_id) {
        body.operation_id = crypto.randomUUID();
      }
      if ((body.operation_id as string).length > MAX_OPERATION_ID_BYTES) {
        body.operation_id = (body.operation_id as string).slice(0, MAX_OPERATION_ID_BYTES);
      }
    }
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

/** StudioTransport whose paged journal rows are normalized like live events. */
export class FaceVivyTransport extends VivyTransport {
  override async events(id: string, afterSeq?: number) {
    const page = await super.events(id, afterSeq);
    return { ...page, events: page.events.map((e) => normalizeJournalEvent(e as unknown as JournalRow)) };
  }
}
