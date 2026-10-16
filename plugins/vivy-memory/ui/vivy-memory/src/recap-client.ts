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
 readonly messages: readonly { readonly id: string; readonly role: string; readonly content: string; readonly tool_status?: string; readonly references?: readonly { readonly id: string; readonly name: string }[] }[];
}
export interface RecapOutcome {
 readonly status: 'listed' | 'applied' | 'conflict' | 'failed';
 readonly head?: RecapHead;
 readonly revisions?: readonly RecapHead[];
 readonly next_before?: number;
 readonly sources?: readonly RecapSource[];
 readonly reason?: string;
 readonly activation?: 'next_run' | 'next_safe_boundary';
 readonly loaded_status?: 'unavailable';
}
export class RecapClient {
 constructor(private readonly actions: MemoryActionTransport) {}
 static fromRPC(rpc: FaceClientRPC) { return new RecapClient(createModuleActionClient(rpc)); }
 invoke(action: 'read' | 'edit' | 'clear' | 'history' | 'revision' | 'restore' | 'sources', sessionId: string, input: Record<string, unknown> = {}): Promise<RecapOutcome> {
  return this.actions.invoke({moduleId: 'vivy/diva-cognitive', actionId: `vivy.recap.${action}`, input: {...input, session_id: sessionId}});
 }
}
