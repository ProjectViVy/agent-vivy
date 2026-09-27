import { request } from './api';
import type { ChildHistoryMessage, ChildMailboxMessage, ChildMode, ChildRun } from './api';

/** Child controls added after the pinned Face API contract was frozen. */
export const startChild = (params: {
  parent_run_id: string;
  text: string;
  mode?: ChildMode;
  operation_id?: string;
  policy_profile?: string;
  tool_names?: string[];
}) => request<ChildRun>('child/start', params);

export const followupChild = (params: {
  child_session_id: string;
  parent_run_id: string;
  operation_id: string;
  text: string;
  tool_names?: string[];
}) => request<ChildRun>('child/followup', params);

export const interruptChild = (runId: string) => request<ChildRun>('child/interrupt', { run_id: runId });

export const getChildHistory = (childSessionId: string, parentRunId: string) =>
  request<{ messages: ChildHistoryMessage[] }>('child/history', {
    child_session_id: childSessionId,
    parent_run_id: parentRunId,
  });

export const sendChildMessage = (params: {
  child_session_id: string;
  parent_run_id: string;
  operation_id: string;
  text: string;
}) => request<{ message: ChildMailboxMessage; inserted: boolean; acknowledgement: 'admitted' }>('child/message/send', params);

export const listChildMessages = (childSessionId: string, authorizerRunId: string) =>
  request<{ messages: ChildMailboxMessage[] }>('child/message/list', {
    child_session_id: childSessionId,
    authorizer_run_id: authorizerRunId,
  });
