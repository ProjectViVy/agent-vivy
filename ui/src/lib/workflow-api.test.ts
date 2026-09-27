import { beforeEach, describe, expect, it, vi } from 'vitest';

const call = vi.fn();
vi.mock('./rpc', () => ({
  RpcClientError: class RpcClientError extends Error { constructor(public code: number, message: string) { super(message); } },
  getRpcClient: vi.fn(async () => ({ call, capabilities: { protocol_version: 'vivy.rpc.v1', capabilities: [] } })),
}));

import * as workflowApi from './workflow-api';

describe('host workflow controls', () => {
  beforeEach(() => { call.mockReset(); call.mockResolvedValue({}); });

  it('keeps proposal and operation identity explicit at the RPC boundary', async () => {
    const descriptor: workflowApi.WorkflowDescriptor = {
      schema_version: 1,
      start_nodes: ['draft'],
      nodes: [{ key: 'draft', task: 'draft a result' }],
      edges: [],
      outputs: ['draft'],
    };
    await workflowApi.proposeWorkflow({ parent_run_id: 'r-parent', descriptor });
    expect(call).toHaveBeenLastCalledWith('workflow/propose', { parent_run_id: 'r-parent', descriptor });
    await workflowApi.startWorkflow({ parent_run_id: 'r-parent', operation_id: 'workflow-op-1', descriptor });
    expect(call).toHaveBeenLastCalledWith('workflow/start', { parent_run_id: 'r-parent', operation_id: 'workflow-op-1', descriptor });
    await workflowApi.getWorkflow('r-workflow');
    expect(call).toHaveBeenLastCalledWith('workflow/get', { run_id: 'r-workflow' });
    await workflowApi.listWorkflows('r-parent');
    expect(call).toHaveBeenLastCalledWith('workflow/list', { parent_run_id: 'r-parent' });
    await workflowApi.cancelWorkflow('r-workflow');
    expect(call).toHaveBeenLastCalledWith('workflow/cancel', { run_id: 'r-workflow' });
  });
});
