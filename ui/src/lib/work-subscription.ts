import { getRpcClient, resetRpcClient, type RpcClient } from './rpc';
import type { WorkEvent } from './api';
import { t } from '@/i18n';

export interface WorkSubscription {
  close(): void;
  retry(): void;
  lastSeq(): number;
}

export function subscribeWork(
  sessionId: string,
  afterSeq: number,
  onEvent: (event: WorkEvent) => void,
  onError?: (message: string) => void,
  onReconnect?: () => Promise<void>,
  onConnected?: (processEpoch: string) => Promise<void>,
): WorkSubscription {
  let closed = false;
  let cursor = afterSeq;
  let subscriptionId = '';
  let client: RpcClient | null = null;
  let removeEvent: (() => void) | undefined;
  let removeStreamError: (() => void) | undefined;
  let removeClose: (() => void) | undefined;
  let timer: number | undefined;
  let connected = false;

  const clearListeners = () => {
    removeEvent?.();
    removeStreamError?.();
    removeClose?.();
    removeEvent = undefined;
    removeStreamError = undefined;
    removeClose = undefined;
  };
  const reconnect = () => {
    if (closed || timer !== undefined) return;
    onError?.(t('errors.reconnecting', { cursor }));
    timer = window.setTimeout(() => {
      timer = undefined;
      void connect();
    }, 1000);
  };
  const retry = () => {
    if (closed) return;
    const previousId = subscriptionId;
    subscriptionId = '';
    clearListeners();
    if (client && previousId) void client.call('session/work/unsubscribe', { subscription_id: previousId }).catch(() => undefined);
    reconnect();
  };
  const connect = async () => {
    if (closed) return;
    clearListeners();
    subscriptionId = '';
    try {
      client = await getRpcClient();
      if (connected) await onReconnect?.();
      if (closed) return;
      removeEvent = client.onNotification('session/work/event', (params) => {
        const envelope = params as { subscription_id?: string; event?: WorkEvent };
        if (envelope.subscription_id !== subscriptionId || !envelope.event || envelope.event.seq <= cursor) return;
        cursor = envelope.event.seq;
        onEvent(envelope.event);
      });
      removeStreamError = client.onNotification('session/work/stream_error', (params) => {
        const envelope = params as { subscription_id?: string; message?: string };
        if (envelope.subscription_id !== subscriptionId) return;
        const failedSubscription = subscriptionId;
        subscriptionId = '';
        clearListeners();
        if (client && failedSubscription) void client.call('session/work/unsubscribe', { subscription_id: failedSubscription }).catch(() => undefined);
        onError?.(envelope.message || t('errors.runReplayFailed'));
        reconnect();
      });
      removeClose = client.onClose(() => {
        clearListeners();
        resetRpcClient();
        reconnect();
      });
      const response = await client.call<{ subscription_id: string; process_epoch: string }>('session/work/subscribe', {
        session_id: sessionId,
        after_seq: cursor,
      });
      if (closed) {
        await client.call('session/work/unsubscribe', { subscription_id: response.subscription_id }).catch(() => undefined);
        return;
      }
      subscriptionId = response.subscription_id;
      if (!response.process_epoch) throw new Error('work subscription missing process epoch');
      await onConnected?.(response.process_epoch);
      connected = true;
    } catch (error) {
      const failedId = subscriptionId;
      subscriptionId = '';
      clearListeners();
      if (client && failedId) void client.call('session/work/unsubscribe', { subscription_id: failedId }).catch(() => undefined);
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
    if (client && subscriptionId) {
      void client.call('session/work/unsubscribe', { subscription_id: subscriptionId }).catch(() => undefined);
    }
  };
  void connect();
  return { close, retry, lastSeq: () => cursor };
}
