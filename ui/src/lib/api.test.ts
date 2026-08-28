import { beforeEach, describe, expect, it, vi } from 'vitest';

const call = vi.fn();
vi.mock('./rpc', () => ({
  RpcClientError: class RpcClientError extends Error { constructor(public code: number, message: string) { super(message); } },
  getRpcClient: vi.fn(async () => ({ call, capabilities: { protocol_version: 'vivy.rpc.v1', capabilities: [] } })),
}));

import * as api from './api';

describe('typed Vivy API', () => {
  beforeEach(() => { call.mockReset(); call.mockResolvedValue({}); });
  it('tracks every server method without duplicates', () => {
    expect(new Set(api.RPC_METHODS).size).toBe(api.RPC_METHODS.length);
    expect(api.RPC_METHODS).toContain('background/recover');
    expect(api.RPC_METHODS).toContain('generations/get');
    expect(api.RPC_METHODS).toContain('evals/start');
    expect(api.RPC_METHODS).toContain('settings/update');
    expect(api.RPC_METHODS).toContain('session/todos');
  });
  it('maps representative runtime and lifecycle operations to their wire methods', async () => {
    call.mockResolvedValueOnce({ session: { id: 's1', title: 'Session', created_at: 1 }, messages: [] });
    const detail = await api.getSession('s1'); expect(call).toHaveBeenLastCalledWith('session/get', { session_id: 's1' }); expect(detail.session.id).toBe('s1');
    await api.listTodos('s1'); expect(call).toHaveBeenLastCalledWith('session/todos', { session_id: 's1' });
    await api.preflight('s1', 'hello', 'normal'); expect(call).toHaveBeenLastCalledWith('preflight/run', { session_id: 's1', text: 'hello', mode: 'normal' });
    await api.waitChild('c1'); expect(call).toHaveBeenLastCalledWith('child/wait', { run_id: 'c1' });
    await api.getGeneration('g1'); expect(call).toHaveBeenLastCalledWith('generations/get', { id: 'g1' });
    await api.startEval({ candidate_id: 'g1', suite: 'smoke' }); expect(call).toHaveBeenLastCalledWith('evals/start', { candidate_id: 'g1', suite: 'smoke' });
    await api.promoteGeneration({ from_id: 'g0', to_id: 'g1', eval_id: 'e1', actor: 'human' }); expect(call).toHaveBeenLastCalledWith('promotions/promote', { from_id: 'g0', to_id: 'g1', eval_id: 'e1', actor: 'human' });
  });
});
