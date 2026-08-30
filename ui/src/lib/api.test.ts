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
    expect(api.RPC_METHODS).toContain('settings/providers/refresh');
    expect(api.RPC_METHODS).toContain('session/todos');
    expect(api.RPC_METHODS).toContain('session/set_permission');
    expect(api.RPC_METHODS).toContain('skills/list');
    expect(api.RPC_METHODS).toContain('skills/get');
    expect(api.RPC_METHODS).toContain('channel/inspect');
    expect(api.RPC_METHODS).toContain('channel/get');
    expect(api.RPC_METHODS).toContain('channel/update');
    expect(api.RPC_METHODS).toContain('cron/list');
    expect(api.RPC_METHODS).toContain('cron/create');
    expect(api.RPC_METHODS).toContain('cron/update');
    expect(api.RPC_METHODS).toContain('cron/delete');
    expect(api.RPC_METHODS).toContain('cron/trigger');
    expect(api.RPC_METHODS).toContain('cron/stop');
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
    await api.listSkills(); expect(call).toHaveBeenLastCalledWith('skills/list', undefined);
    await api.getSkill('demo-skill', 'references/guide.md'); expect(call).toHaveBeenLastCalledWith('skills/get', { name: 'demo-skill', path: 'references/guide.md' });
    await api.refreshProviderModels({ id: 'custom-1' }); expect(call).toHaveBeenLastCalledWith('settings/providers/refresh', { id: 'custom-1' });
    await api.refreshProviderModels({ bundle: 'openai', base_url: 'https://gateway.example.com/v1' }); expect(call).toHaveBeenLastCalledWith('settings/providers/refresh', { bundle: 'openai', base_url: 'https://gateway.example.com/v1' });
  });
  it('maps cron operations to their wire methods', async () => {
    const schedule = { kind: 'cron' as const, expr: '0 9 * * *', tz: 'Asia/Shanghai' };
    const payload = { kind: 'agent_turn', message: 'hello', deliver: false };
    const input = { name: 'brief', enabled: true, schedule, payload, delete_after_run: false };
    await api.listCronJobs(); expect(call).toHaveBeenLastCalledWith('cron/list', undefined);
    await api.createCronJob(input); expect(call).toHaveBeenLastCalledWith('cron/create', input);
    await api.updateCronJob('cron_1', input); expect(call).toHaveBeenLastCalledWith('cron/update', { id: 'cron_1', ...input });
    await api.deleteCronJob('cron_1'); expect(call).toHaveBeenLastCalledWith('cron/delete', { id: 'cron_1' });
    await api.triggerCronJob('cron_1'); expect(call).toHaveBeenLastCalledWith('cron/trigger', { id: 'cron_1' });
    await api.stopCronJob('cron_1'); expect(call).toHaveBeenLastCalledWith('cron/stop', { id: 'cron_1' });
  });
});
