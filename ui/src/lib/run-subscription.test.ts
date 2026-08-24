import { beforeEach, describe, expect, it, vi } from 'vitest';

let notifications: Record<string, (params: unknown) => void> = {};
const call = vi.fn(async (method: string) => method === 'run/subscribe' ? { subscription_id: 'sub-1' } : {});
const client = { call, onNotification: vi.fn((method: string, listener: (params: unknown) => void) => { notifications[method] = listener; return () => { delete notifications[method]; }; }), onClose: vi.fn(() => () => undefined) };
vi.mock('./rpc', () => ({ getRpcClient: vi.fn(async () => client), resetRpcClient: vi.fn() }));
import { subscribeRun } from './run-subscription';

describe('run subscription', () => {
  beforeEach(() => { call.mockClear(); notifications = {}; vi.stubGlobal('window', { setTimeout, clearTimeout }); });
  it('keeps the parent subscription open for child terminal events', async () => {
    const events: string[] = [];
    subscribeRun('run-1', 4, (event) => events.push(event.type));
    await Promise.resolve(); await Promise.resolve();
    notifications['run/event']?.({ subscription_id: 'sub-1', event: { run_id: 'run-1', seq: 5, type: 'child.completed', created_at: 1, payload_version: 1, payload: {} } });
    expect(call).not.toHaveBeenCalledWith('run/unsubscribe', expect.anything());
    notifications['run/event']?.({ subscription_id: 'sub-1', event: { run_id: 'run-1', seq: 6, type: 'run.completed', created_at: 2, payload_version: 1, payload: {} } });
    await Promise.resolve();
    expect(events).toEqual(['child.completed', 'run.completed']);
    expect(call).toHaveBeenCalledWith('run/unsubscribe', { subscription_id: 'sub-1' });
  });
  it('surfaces stream replay errors and reconnects without waiting for socket close', async () => {
    vi.useFakeTimers();
    vi.stubGlobal('window', { setTimeout, clearTimeout });
    const errors: string[] = [];
    subscribeRun('run-1', 8, () => undefined, (message) => errors.push(message));
    await Promise.resolve(); await Promise.resolve();
    notifications['run/stream_error']?.({ subscription_id: 'sub-1', message: 'event replay failed' });
    expect(errors).toContain('event replay failed');
    expect(call).toHaveBeenCalledWith('run/unsubscribe', { subscription_id: 'sub-1' });
    await vi.advanceTimersByTimeAsync(1000);
    expect(call.mock.calls.filter(([method]) => method === 'run/subscribe')).toHaveLength(2);
    vi.useRealTimers();
  });
});
