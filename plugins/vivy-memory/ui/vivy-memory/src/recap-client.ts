import { createModuleActionClient, type FaceClientRPC } from '@vivy/ui-sdk';
import type { MemoryActionTransport } from './memory-client';

export interface RecapHead {
 readonly revision: number;
 readonly text: string;
 readonly cause: string;
 readonly covered_through: number;
 readonly invalidation_epoch: number;
 readonly restored_from?: number;
 readonly updated_at: string;
}
export interface RecapSource {
 readonly interaction_id: string;
 readonly source_revision?: number;
 readonly references?: readonly { readonly id: string; readonly source_workspace: string; readonly items: readonly { readonly id: string; readonly author: string; readonly text: string }[] }[];
 readonly corrections?: readonly { readonly view_session_id: string; readonly reason: string; readonly message_ids: readonly string[]; readonly view_message_ids: readonly string[] }[];
 readonly messages: readonly {
  readonly id: string; readonly role: string; readonly content: string;
  readonly tool_status?: string; readonly tool_name?: string; readonly tool_arguments?: string; readonly tool_error?: string; readonly disposition?: string;
  readonly attachments?: readonly { readonly name: string; readonly mime_type: string; readonly data_url: string }[];
  readonly file_contexts?: readonly { readonly name: string; readonly path: string; readonly content: string }[];
  readonly references?: readonly { readonly id: string; readonly name: string }[];
 }[];
}
export interface RecapOutcome {
 readonly status: 'listed' | 'applied' | 'conflict' | 'failed';
 readonly head?: RecapHead;
 readonly revisions?: readonly RecapHead[];
 readonly next_before?: number;
 readonly sources?: readonly RecapSource[];
 readonly reason?: string;
 readonly activation?: 'next_run' | 'next_safe_boundary';
 readonly loaded_status?: 'unavailable' | 'loaded';
 readonly loaded?: { readonly status: 'loaded'; readonly run_id: string; readonly revision: number; readonly invalidation_epoch: number };
}
export class RecapClient {
 constructor(private readonly actions: MemoryActionTransport, private readonly bindSession?: (sessionId: string) => Promise<unknown>) {}
 static fromRPC(rpc: FaceClientRPC) { return new RecapClient(createModuleActionClient(rpc), sessionId => rpc.call('session/get', {session_id: sessionId})); }
 async invoke(action: 'read' | 'edit' | 'clear' | 'history' | 'revision' | 'restore' | 'sources', sessionId: string, input: Record<string, unknown> = {}): Promise<RecapOutcome> {
  // activeSessionId can change before the peer's asynchronous session lookup
  // completes. Bind on this same transport before every action, including after
  // reconnect; a denied lookup must never fall through to a write.
  await this.bindSession?.(sessionId);
  return this.actions.invoke({moduleId: 'vivy/diva-cognitive', actionId: `vivy.recap.${action}`, input: {...input, session_id: sessionId}});
 }
}
