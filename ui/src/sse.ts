// SSE subscription with a reconnect cursor. The server replays from the
// journal then streams live; reconnecting with after_seq=<last seen seq>
// resumes without gaps or duplicates (AS-7). Reconnection is managed here
// (not left to EventSource's built-in retry) because the cursor must move
// with every delivered frame.

export const RUN_EVENT_TYPES = [
  "run.started",
  "provider.retry",
  "provider.stall",
  "model.reasoning_delta",
  "model.delta",
  "model.usage",
  "model.completed",
  "tool.requested",
  "tool.approval_required",
  "tool.started",
  "tool.finished",
  "user.question_required",
  "user.question_answered",
  "run.completed",
  "run.failed",
  "run.cancelled",
] as const;

export type RunEventType = (typeof RUN_EVENT_TYPES)[number];

export const TERMINAL_EVENT_TYPES: ReadonlySet<string> = new Set([
  "run.completed",
  "run.failed",
  "run.cancelled",
]);

// EventEnvelope mirrors the A3 envelope written by events.writeFrame.
export interface EventEnvelope {
  run_id: string;
  seq: number;
  type: RunEventType;
  created_at: number;
  payload_version: number;
  payload: Record<string, unknown>;
}

export interface RunSubscription {
  close(): void;
  lastSeq(): number;
}

const RECONNECT_DELAY_MS = 1000;

export function subscribeRun(
  runID: string,
  afterSeq: number,
  onEvent: (env: EventEnvelope) => void,
  onError?: (message: string) => void,
): RunSubscription {
  let closed = false;
  let source: EventSource | null = null;
  let retryTimer: number | undefined;
  let cursor = afterSeq;

  const open = () => {
    if (closed) return;
    const es = new EventSource(`/api/runs/${runID}/events?after_seq=${cursor}`);
    source = es;
    for (const type of RUN_EVENT_TYPES) {
      es.addEventListener(type, (ev) => {
        let env: EventEnvelope;
        try {
          env = JSON.parse((ev as MessageEvent).data) as EventEnvelope;
        } catch {
          return; // Undecodable frame: skip; the journal stays the truth.
        }
        cursor = env.seq;
        onEvent(env);
        if (TERMINAL_EVENT_TYPES.has(env.type)) {
          shutdown();
        }
      });
    }
    es.onerror = () => {
      es.close();
      source = null;
      if (closed) return;
      // Network blip or server restart: reopen from the cursor shortly.
      retryTimer = window.setTimeout(open, RECONNECT_DELAY_MS);
      if (onError) onError(`event stream interrupted; reconnecting from seq ${cursor}`);
    };
  };

  const shutdown = () => {
    closed = true;
    if (retryTimer !== undefined) window.clearTimeout(retryTimer);
    if (source) {
      source.close();
      source = null;
    }
  };

  open();
  return {
    close: shutdown,
    lastSeq: () => cursor,
  };
}
