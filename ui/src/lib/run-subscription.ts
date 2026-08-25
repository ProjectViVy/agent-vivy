import { getRpcClient, resetRpcClient, type RpcClient } from './rpc';
import type { RunLogEvent } from './api';
import { t } from '@/i18n';

export type RunEvent = RunLogEvent;
export interface RunSubscription { close(): void; lastSeq(): number }
const TERMINAL = new Set(['run.completed', 'run.failed', 'run.cancelled']);

export function subscribeRun(runId: string, afterSeq: number, onEvent: (event: RunEvent) => void, onError?: (message: string) => void): RunSubscription {
  let closed = false;
  let cursor = afterSeq;
  let subscriptionId = '';
  let client: RpcClient | null = null;
  let removeEvent: (() => void) | undefined;
  let removeStreamError: (() => void) | undefined;
  let removeClose: (() => void) | undefined;
  let timer: number | undefined;

  const clearListeners = () => { removeEvent?.(); removeStreamError?.(); removeClose?.(); removeEvent = undefined; removeStreamError = undefined; removeClose = undefined; };
  const reconnect = () => {
    if (closed || timer !== undefined) return;
    onError?.(t('errors.reconnecting', { cursor }));
    timer = window.setTimeout(() => { timer = undefined; void connect(); }, 1000);
  };
  const connect = async () => {
    if (closed) return;
    clearListeners();
    subscriptionId = '';
    try {
      client = await getRpcClient();
      removeEvent = client.onNotification('run/event', (params) => {
        const envelope = params as { subscription_id?: string; event?: RunEvent };
        if (envelope.subscription_id !== subscriptionId || !envelope.event || envelope.event.seq <= cursor) return;
        cursor = envelope.event.seq;
        onEvent(envelope.event);
        if (TERMINAL.has(envelope.event.type)) close();
      });
      removeStreamError = client.onNotification('run/stream_error', (params) => {
        const envelope = params as { subscription_id?: string; message?: string };
        if (envelope.subscription_id !== subscriptionId) return;
        const failedSubscription = subscriptionId;
        subscriptionId = '';
        clearListeners();
        if (client && failedSubscription) void client.call('run/unsubscribe', { subscription_id: failedSubscription }).catch(() => undefined);
        onError?.(envelope.message || t('errors.runReplayFailed'));
        reconnect();
      });
      removeClose = client.onClose(() => { clearListeners(); resetRpcClient(); reconnect(); });
      const response = await client.call<{ subscription_id: string }>('run/subscribe', { run_id: runId, after_seq: cursor });
      if (closed) {
        await client.call('run/unsubscribe', { subscription_id: response.subscription_id }).catch(() => undefined);
        return;
      }
      subscriptionId = response.subscription_id;
    } catch (error) {
      clearListeners();
      resetRpcClient();
      onError?.(error instanceof Error ? error.message : String(error));
      reconnect();
    }
  };
  const close = () => {
    if (closed) return;
    closed = true;
    clearListeners();
    if (timer !== undefined) window.clearTimeout(timer);
    if (client && subscriptionId) void client.call('run/unsubscribe', { subscription_id: subscriptionId }).catch(() => undefined);
  };
  void connect();
  return { close, lastSeq: () => cursor };
}
