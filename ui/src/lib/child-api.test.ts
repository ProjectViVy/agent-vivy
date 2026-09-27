import { beforeEach, describe, expect, it, vi } from 'vitest';

const call = vi.fn();
vi.mock('./rpc', () => ({
  RpcClientError: class RpcClientError extends Error { constructor(public code: number, message: string) { super(message); } },
  getRpcClient: vi.fn(async () => ({ call, capabilities: { protocol_version: 'vivy.rpc.v1', capabilities: [] } })),
}));

import * as childApi from './child-api';

describe('host-authoritative child controls', () => {
  beforeEach(() => { call.mockReset(); call.mockResolvedValue({}); });

  it('maps continuable child controls to their RPC methods', async () => {
    await childApi.startChild({ parent_run_id: 'r-parent', text: 'task', mode: 'continuable', operation_id: 'create-1' });
    expect(call).toHaveBeenLastCalledWith('child/start', { parent_run_id: 'r-parent', text: 'task', mode: 'continuable', operation_id: 'create-1' });
    await childApi.followupChild({ child_session_id: 'cs-1', parent_run_id: 'r-parent-2', operation_id: 'follow-1', text: 'next' });
    expect(call).toHaveBeenLastCalledWith('child/followup', { child_session_id: 'cs-1', parent_run_id: 'r-parent-2', operation_id: 'follow-1', text: 'next' });
    await childApi.interruptChild('r-child');
    expect(call).toHaveBeenLastCalledWith('child/interrupt', { run_id: 'r-child' });
    await childApi.getChildHistory('cs-1', 'r-parent-2');
    expect(call).toHaveBeenLastCalledWith('child/history', { child_session_id: 'cs-1', parent_run_id: 'r-parent-2' });
    await childApi.sendChildMessage({ child_session_id: 'cs-1', parent_run_id: 'r-parent-2', operation_id: 'mail-1', text: 'please continue' });
    expect(call).toHaveBeenLastCalledWith('child/message/send', { child_session_id: 'cs-1', parent_run_id: 'r-parent-2', operation_id: 'mail-1', text: 'please continue' });
    await childApi.listChildMessages('cs-1', 'r-parent-2');
    expect(call).toHaveBeenLastCalledWith('child/message/list', { child_session_id: 'cs-1', authorizer_run_id: 'r-parent-2' });
  });
});
