// Run event subscriptions use the same bidirectional JSON-RPC connection as
// commands. The public shape remains stable for the UI state layer while the
// transport now supports server-initiated notifications and replay cursors.

import { getRpcClient, resetRpcClient, type RpcClient } from "./rpc";

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
  "policy.evaluated",
  "hook.started",
  "hook.completed",
  "hook.blocked",
  "user.question_required",
  "user.question_answered",
  "child.requested",
  "child.started",
  "child.suspended",
  "child.resumed",
  "child.completed",
  "child.failed",
  "child.cancelled",
  "run.completed",
  "run.failed",
  "run.cancelled",
] as const;

export type RunEventType = (typeof RUN_EVENT_TYPES)[number];

export const TERMINAL_EVENT_TYPES: ReadonlySet<string> = new Set([
  "run.completed",
  "run.failed",
  "run.cancelled",
  "child.completed",
  "child.failed",
  "child.cancelled",
]);

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
  let cursor = afterSeq;
  let subscriptionID = "";
  let client: RpcClient | null = null;
  let removeEvent: (() => void) | undefined;
  let removeClose: (() => void) | undefined;
  let retryTimer: number | undefined;

  const cleanupListeners = () => {
    removeEvent?.();
    removeClose?.();
    removeEvent = undefined;
    removeClose = undefined;
  };

  const scheduleReconnect = () => {
    if (closed || retryTimer !== undefined) return;
    retryTimer = window.setTimeout(() => {
      retryTimer = undefined;
      void connect();
    }, RECONNECT_DELAY_MS);
    onError?.(`control plane interrupted; reconnecting from seq ${cursor}`);
  };

  const connect = async () => {
    if (closed) return;
    cleanupListeners();
    subscriptionID = "";
    try {
      client = await getRpcClient();
      removeEvent = client.onNotification("run/event", (params) => {
        const envelope = params as { subscription_id?: string; event?: EventEnvelope };
        if (envelope.subscription_id !== subscriptionID || !envelope.event) return;
        if (envelope.event.seq <= cursor) return;
        cursor = envelope.event.seq;
        onEvent(envelope.event);
        if (TERMINAL_EVENT_TYPES.has(envelope.event.type)) close();
      });
      removeClose = client.onClose(() => {
        cleanupListeners();
        resetRpcClient();
        scheduleReconnect();
      });
      const response = await client.call<{ subscription_id: string }>("run/subscribe", {
        run_id: runID,
        after_seq: cursor,
      });
      if (closed) {
        await client.call("run/unsubscribe", { subscription_id: response.subscription_id }).catch(() => undefined);
        return;
      }
      subscriptionID = response.subscription_id;
    } catch (error) {
      cleanupListeners();
      resetRpcClient();
      scheduleReconnect();
      if (!closed) onError?.(`subscription failed: ${error}`);
    }
  };

  const close = () => {
    if (closed) return;
    closed = true;
    cleanupListeners();
    if (retryTimer !== undefined) window.clearTimeout(retryTimer);
    retryTimer = undefined;
    if (client && subscriptionID) {
      void client.call("run/unsubscribe", { subscription_id: subscriptionID }).catch(() => undefined);
    }
  };

  void connect();
  return { close, lastSeq: () => cursor };
}
