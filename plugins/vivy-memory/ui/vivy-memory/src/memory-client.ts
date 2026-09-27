import {
  createModuleActionClient,
  type FaceClientRPC,
} from '@vivy/ui-sdk';

/** The control-action plane is owned by the BML backend Module, not this UI Module. */
export const MEMORY_MODULE_ID = 'vivy/memory-bml' as const;

export const MEMORY_ACTIONS = Object.freeze({
  list: 'vivy.memory.list',
  search: 'vivy.memory.search',
  get: 'vivy.memory.get',
  add: 'vivy.memory.add',
  update: 'vivy.memory.update',
  remove: 'vivy.memory.remove',
  rulesRead: 'vivy.memory.rules.read',
  rulesWrite: 'vivy.memory.rules.write',
  status: 'vivy.memory.status',
} as const);

/** Stable reason codes the backend reports on failed outcomes. */
export const MEMORY_REASONS = Object.freeze({
  revisionConflict: 'memory_revision_conflict',
  notFound: 'memory_not_found',
  unavailable: 'bml_unavailable',
  invalidRequest: 'memory_invalid_request',
} as const);

/** Wire shape of one BML memory record (serde spelling, see bml.MemoryEntry). */
export interface MemoryEntry {
  readonly id: string;
  readonly content: string;
  readonly trust: string;
  readonly provenance: string | null;
  readonly evidence_refs: readonly MemoryEvidenceRef[];
  readonly revision: number;
  readonly created_at: string;
  readonly updated_at: string;
}

export interface MemoryEvidenceRef {
  readonly id: string;
  readonly source: string;
  readonly uri: string;
  readonly excerpt?: string | null;
  readonly hash?: string | null;
  readonly created_at?: string;
}

/**
 * MemoryOutcome mirrors bml.MemoryCrudOutcome — an internally tagged union on
 * `status`: `listed` carries `entries`, `applied` carries `entry` plus an
 * optional `evidence_advisory`, `proposal_created` carries `proposal_id`,
 * `failed` carries `reason`.
 */
export interface MemoryOutcome {
  readonly status: 'listed' | 'applied' | 'proposal_created' | 'failed' | string;
  readonly entries?: readonly MemoryEntry[];
  readonly entry?: MemoryEntry | null;
  readonly evidence_advisory?: string;
  readonly proposal_id?: string;
  readonly reason?: string;
}

/** rules.read returns its own shape: the handbook plus the digest CAS token. */
export interface MemoryRulesOutcome {
  readonly status: 'listed' | 'failed' | string;
  readonly content?: string;
  readonly source?: string;
  readonly revision?: string;
  readonly reason?: string;
}

export interface MemoryStatusOutcome {
  readonly status: 'listed' | 'failed' | string;
  readonly available?: boolean;
  readonly startup_revision?: number;
  readonly database_present?: boolean;
  readonly rules_revision?: string;
  readonly reason?: string;
}

export interface MemoryListInput {
  readonly limit?: number;
}

export interface MemorySearchInput {
  readonly query: string;
  readonly limit?: number;
}

export interface MemoryGetInput {
  readonly id: string;
}

export interface MemoryAddInput {
  readonly kind: 'long_term';
  readonly content: string;
}

export interface MemoryUpdateInput {
  readonly id: string;
  readonly content: string;
  readonly base_revision: number;
}

export interface MemoryRemoveInput {
  readonly id: string;
  readonly reason: string;
  readonly base_revision: number;
}

export interface MemoryRulesWriteInput {
  readonly content: string;
  readonly base_revision: string;
}

/** The narrow transport seam makes the UI client independently testable. */
export interface MemoryActionTransport {
  invoke<Input = unknown, Result = unknown>(request: {
    readonly moduleId: string;
    readonly actionId: string;
    readonly input: Input;
  }): Promise<Result>;
}

export class MemoryClient {
  constructor(private readonly actions: MemoryActionTransport) {}

  static fromRPC(rpc: FaceClientRPC): MemoryClient {
    return new MemoryClient(createModuleActionClient(rpc));
  }

  list(input: MemoryListInput = {}): Promise<MemoryOutcome> {
    return this.invoke<MemoryListInput, MemoryOutcome>(MEMORY_ACTIONS.list, input);
  }

  search(input: MemorySearchInput): Promise<MemoryOutcome> {
    return this.invoke<MemorySearchInput, MemoryOutcome>(MEMORY_ACTIONS.search, input);
  }

  get(input: MemoryGetInput): Promise<MemoryOutcome> {
    return this.invoke<MemoryGetInput, MemoryOutcome>(MEMORY_ACTIONS.get, input);
  }

  add(input: MemoryAddInput): Promise<MemoryOutcome> {
    return this.invoke<MemoryAddInput, MemoryOutcome>(MEMORY_ACTIONS.add, input);
  }

  update(input: MemoryUpdateInput): Promise<MemoryOutcome> {
    return this.invoke<MemoryUpdateInput, MemoryOutcome>(MEMORY_ACTIONS.update, input);
  }

  remove(input: MemoryRemoveInput): Promise<MemoryOutcome> {
    return this.invoke<MemoryRemoveInput, MemoryOutcome>(MEMORY_ACTIONS.remove, input);
  }

  rulesRead(): Promise<MemoryRulesOutcome> {
    return this.invoke<Record<string, never>, MemoryRulesOutcome>(MEMORY_ACTIONS.rulesRead, {});
  }

  rulesWrite(input: MemoryRulesWriteInput): Promise<MemoryOutcome> {
    return this.invoke<MemoryRulesWriteInput, MemoryOutcome>(MEMORY_ACTIONS.rulesWrite, input);
  }

  status(): Promise<MemoryStatusOutcome> {
    return this.invoke<Record<string, never>, MemoryStatusOutcome>(MEMORY_ACTIONS.status, {});
  }

  private invoke<Input, Result>(actionId: string, input: Input): Promise<Result> {
    return this.actions.invoke<Input, Result>({ moduleId: MEMORY_MODULE_ID, actionId, input });
  }
}
