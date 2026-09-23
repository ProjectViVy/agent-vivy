import {
  createModuleActionClient,
  type FaceClientRPC,
} from '@vivy/ui-sdk';

/** The backend owner is the internal mask service, while this Module owns only the UI. */
export const MASK_MODULE_ID = 'vivy/masks' as const;

export const MASK_ACTIONS = Object.freeze({
  catalogList: 'vivy.masks.catalog.list',
  catalogGet: 'vivy.masks.catalog.get',
  catalogCreate: 'vivy.masks.catalog.create',
  catalogUpdate: 'vivy.masks.catalog.update',
  catalogDelete: 'vivy.masks.catalog.delete',
  selectionGet: 'vivy.masks.selection.get',
  selectionSet: 'vivy.masks.selection.set',
} as const);

export interface MaskMetadata {
  readonly id: string;
  readonly name: string;
  readonly description: string;
  readonly digest: string;
  readonly generation_id: string;
  readonly revision: number;
  readonly built_in: boolean;
}

export interface MaskDefinition extends MaskMetadata {
  readonly body: string;
}

export interface MaskPageResult {
  readonly items: readonly MaskMetadata[];
  readonly next_after_id: string;
}

export interface MaskSelection {
  readonly session_id: string;
  readonly mask_id: string;
  readonly revision: number;
  readonly available: boolean;
  readonly inactive_reason: '' | 'not_compiled' | string;
}

export interface MaskListInput {
  readonly after_id?: string;
  readonly limit?: number;
}

export interface MaskCreateInput {
  readonly operation_id: string;
  readonly name: string;
  readonly description: string;
  readonly body: string;
}

export interface MaskUpdateInput {
  readonly id: string;
  readonly expected_revision: number;
  readonly name: string;
  readonly description: string;
  readonly body: string;
}

export interface MaskDeleteInput {
  readonly id: string;
  readonly expected_revision: number;
}

export interface MaskSelectionGetInput {
  readonly session_id: string;
}

export interface MaskSelectionSetInput {
  readonly session_id: string;
  readonly mask_id: string;
  readonly expected_revision: number;
}

/** The narrow transport seam makes the UI client independently testable. */
export interface MaskActionTransport {
  invoke<Input = unknown, Result = unknown>(request: {
    readonly moduleId: string;
    readonly actionId: string;
    readonly input: Input;
  }): Promise<Result>;
}

export class MaskClient {
  constructor(private readonly actions: MaskActionTransport) {}

  static fromRPC(rpc: FaceClientRPC): MaskClient {
    return new MaskClient(createModuleActionClient(rpc));
  }

  list(input: MaskListInput = {}): Promise<MaskPageResult> {
    return this.invoke<MaskListInput, MaskPageResult>(MASK_ACTIONS.catalogList, {
      after_id: input.after_id ?? '',
      limit: input.limit ?? 0,
    });
  }

  get(id: string): Promise<MaskDefinition> {
    return this.invoke<{ readonly id: string }, MaskDefinition>(MASK_ACTIONS.catalogGet, { id });
  }

  create(input: MaskCreateInput): Promise<MaskDefinition> {
    return this.invoke<MaskCreateInput, MaskDefinition>(MASK_ACTIONS.catalogCreate, input);
  }

  update(input: MaskUpdateInput): Promise<MaskDefinition> {
    return this.invoke<MaskUpdateInput, MaskDefinition>(MASK_ACTIONS.catalogUpdate, input);
  }

  remove(input: MaskDeleteInput): Promise<{ readonly id: string }> {
    return this.invoke<MaskDeleteInput, { readonly id: string }>(MASK_ACTIONS.catalogDelete, input);
  }

  getSelection(sessionId: string): Promise<MaskSelection> {
    return this.invoke<MaskSelectionGetInput, MaskSelection>(MASK_ACTIONS.selectionGet, { session_id: sessionId });
  }

  setSelection(input: MaskSelectionSetInput): Promise<MaskSelection> {
    return this.invoke<MaskSelectionSetInput, MaskSelection>(MASK_ACTIONS.selectionSet, input);
  }

  private invoke<Input, Result>(actionId: string, input: Input): Promise<Result> {
    return this.actions.invoke<Input, Result>({ moduleId: MASK_MODULE_ID, actionId, input });
  }
}
