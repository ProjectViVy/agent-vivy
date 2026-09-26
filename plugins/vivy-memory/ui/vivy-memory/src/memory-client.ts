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
}

/**
 * MemoryCrudOutcome is an internally tagged union: `listed` carries
 * `entries`, `failed` carries `reason`. Other statuses are not produced by
 * the read actions but stay representable so the view never has to invent a
 * fallback payload.
 */
export interface MemoryOutcome {
  readonly status: 'listed' | 'applied' | 'proposal_created' | 'failed' | string;
  readonly entries?: readonly MemoryEntry[];
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

  get(id: string): Promise<MemoryOutcome> {
    return this.invoke<MemoryGetInput, MemoryOutcome>(MEMORY_ACTIONS.get, { id });
  }

  private invoke<Input, Result>(actionId: string, input: Input): Promise<Result> {
    return this.actions.invoke<Input, Result>({ moduleId: MEMORY_MODULE_ID, actionId, input });
  }
}
