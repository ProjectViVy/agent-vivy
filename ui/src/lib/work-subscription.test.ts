import { beforeEach, describe, expect, it, vi } from 'vitest';

type Listener = (params: unknown) => void;
const rpc = vi.hoisted(() => ({ getRpcClient: vi.fn(), resetRpcClient: vi.fn() }));
const clients: Array<{
  call: ReturnType<typeof vi.fn>;
  notifications: Record<string, Listener>;
  close?: () => void;
}> = [];
vi.mock('./rpc', () => rpc);
import { subscribeWork } from './work-subscription';

function client() {
  const subscriptionId = `sub-${clients.length + 1}`;
  const current = {
    call: vi.fn(async (method: string) => method === 'session/work/subscribe' ? { subscription_id: subscriptionId, process_epoch: 'process-b', watermark_seq: 1 } : {}),
    notifications: {} as Record<string, Listener>,
    close: undefined as undefined | (() => void),
  };
  clients.push(current);
  return {
    ...current,
    onNotification(method: string, listener: Listener) { current.notifications[method] = listener; return () => { delete current.notifications[method]; }; },
    onClose(listener: () => void) { current.close = listener; return () => { current.close = undefined; }; },
  };
}

describe('work subscription', () => {
  beforeEach(() => {
    clients.length = 0;
    rpc.getRpcClient.mockReset();
    vi.stubGlobal('window', { setTimeout, clearTimeout });
  });

  it('refreshes backend WorkView on reconnect before subscribing and deduplicates the session sequence', async () => {
    const first = client();
    rpc.getRpcClient.mockResolvedValueOnce(first);
    const second = client();
    rpc.getRpcClient.mockResolvedValueOnce(second);
    const events: number[] = [];
    const refresh = vi.fn(async () => undefined);
    const subscription = subscribeWork('s1', 1, (event) => events.push(event.seq), undefined, refresh);
    await vi.waitFor(() => expect(first.call).toHaveBeenCalledWith('session/work/subscribe', { session_id: 's1', after_seq: 1 }));
    clients[0].close?.();
    await new Promise((done) => setTimeout(done, 1100));
    await vi.waitFor(() => expect(second.call).toHaveBeenCalledWith('session/work/subscribe', { session_id: 's1', after_seq: 1 }));
    expect(refresh).toHaveBeenCalledOnce();
    expect(second.call.mock.invocationCallOrder[0]).toBeGreaterThan(refresh.mock.invocationCallOrder[0]);
    const event = { seq: 2, kind: 'goal.round_admitted', request_id: 'round-2', created_at: 2 };
    second.notifications['session/work/event']?.({ subscription_id: 'sub-2', event });
    second.notifications['session/work/event']?.({ subscription_id: 'sub-2', event });
    expect(events).toEqual([2]);
    subscription.close();
  });

  it('reports the first subscription process epoch even without a later event', async () => {
    const first = client();
    rpc.getRpcClient.mockResolvedValueOnce(first);
    const connected = vi.fn(async (_epoch: string) => undefined);
    const subscription = subscribeWork('s1', 1, () => undefined, undefined, undefined, connected);

    await vi.waitFor(() => expect(connected).toHaveBeenCalledWith('process-b'));
    subscription.close();
  });

  it('retries through the existing reconnect path after a failed event refresh', async () => {
    const first = client();
    const second = client();
    rpc.getRpcClient.mockResolvedValueOnce(first).mockResolvedValueOnce(second);
    const refresh = vi.fn(async () => undefined);
    const subscription = subscribeWork('s1', 1, () => undefined, undefined, refresh);
    await vi.waitFor(() => expect(first.call).toHaveBeenCalledWith('session/work/subscribe', { session_id: 's1', after_seq: 1 }));

    subscription.retry();
    await vi.waitFor(() => expect(second.call).toHaveBeenCalledWith('session/work/subscribe', { session_id: 's1', after_seq: 1 }), { timeout: 1600 });

    expect(refresh).toHaveBeenCalledOnce();
    subscription.close();
  });
});
