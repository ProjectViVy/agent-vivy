import { request } from './api';

export interface WorkflowNode {
  key: string;
  task: string;
  tool_names?: string[];
}

export interface WorkflowEdge {
  from: string;
  to: string;
  input_key?: string;
}

export interface WorkflowDescriptor {
  schema_version: number;
  start_nodes: string[];
  nodes: WorkflowNode[];
  edges: WorkflowEdge[];
  outputs: string[];
}

// The authoritative topology and supported node catalog are validated by
// the host's INOFY schema and admission path, not duplicated in TypeScript.
export type WorkflowDefinition = { schema_version: 'inofy.workflow/v1'; graph: Record<string, unknown> };

export interface WorkflowNodeProjection {
  key: string;
  status: 'waiting' | 'running' | 'completed' | 'failed' | 'blocked' | 'cancelled';
  child_run_id?: string;
  result_digest?: string;
  error_category?: string;
  message?: string;
}

export interface WorkflowResult {
  id: string;
  parent_run_id: string;
  root_run_id: string;
  session_id: string;
  status: string;
  revision_digest: string;
  depth: number;
  created_at: number;
  created?: boolean;
  descriptor: WorkflowDescriptor;
  definition?: WorkflowDefinition;
  nodes: WorkflowNodeProjection[];
  outputs?: Record<string, string>;
}

export interface WorkflowProposal {
  definition: WorkflowDefinition;
  digest: string;
}

export const proposeWorkflow = (params: { parent_run_id: string; definition: WorkflowDefinition }) =>
  request<WorkflowProposal>('workflow/propose', params);

export const startWorkflow = (params: { parent_run_id: string; operation_id: string; definition: WorkflowDefinition }) =>
  request<WorkflowResult>('workflow/start', params);

export const getWorkflow = (runId: string) => request<WorkflowResult>('workflow/get', { run_id: runId });

export const listWorkflows = (parentRunId: string) => request<{ workflows: WorkflowResult[] }>('workflow/list', { parent_run_id: parentRunId });

export const cancelWorkflow = (runId: string) => request<WorkflowResult>('workflow/cancel', { run_id: runId });
