import type { MaskDefinition } from './mask-client';

export interface MaskDraft {
  readonly id?: string;
  readonly expectedRevision?: number;
  readonly operationId?: string;
  readonly name: string;
  readonly description: string;
  readonly body: string;
}

export interface MaskUIError {
  readonly code?: string | number;
  readonly message: string;
  readonly currentRevision?: number;
  readonly referenceCount?: number;
  readonly reloadFailed?: boolean;
}

export interface MaskState {
  readonly openedId: string | null;
  readonly definitions: Readonly<Record<string, MaskDefinition>>;
  readonly definitionPending: readonly string[];
  readonly draft: MaskDraft | null;
  readonly draftDirty: boolean;
  readonly draftSaving: boolean;
  readonly deletingId: string | null;
  readonly error: MaskUIError | null;
}

export const initialMaskState: MaskState = Object.freeze({
  openedId: '',
  definitions: {},
  definitionPending: [],
  draft: null,
  draftDirty: false,
  draftSaving: false,
  deletingId: null,
  error: null,
});

export type MaskAction =
  | { readonly type: 'definition/load-start'; readonly id: string }
  | { readonly type: 'definition/loaded'; readonly definition: MaskDefinition }
  | { readonly type: 'definition/load-error'; readonly id: string; readonly error: MaskUIError }
  | { readonly type: 'draft/new'; readonly draft?: Pick<MaskDraft, 'name' | 'description' | 'body'> }
  | { readonly type: 'definition/open'; readonly definition: MaskDefinition }
  | { readonly type: 'definition/default' }
  | { readonly type: 'draft/cancel' }
  | { readonly type: 'error/clear' }
  | { readonly type: 'draft/change'; readonly field: 'name' | 'description' | 'body'; readonly value: string }
  | { readonly type: 'draft/save-start'; readonly operationId?: string }
  | { readonly type: 'draft/save-success'; readonly definition: MaskDefinition }
  | { readonly type: 'draft/save-error'; readonly error: MaskUIError }
  | { readonly type: 'delete/start'; readonly id: string }
  | { readonly type: 'delete/success'; readonly id: string }
  | { readonly type: 'delete/error'; readonly error: MaskUIError };

export function maskReducer(state: MaskState, action: MaskAction): MaskState {
  switch (action.type) {
    case 'error/clear':
      return { ...state, error: null };
    case 'definition/load-start':
      return { ...state, openedId: action.id, draft: null, draftDirty: false, definitionPending: unique([...state.definitionPending, action.id]), error: null };
    case 'definition/loaded': {
      const definitionPending = state.definitionPending.filter((id) => id !== action.definition.id);
      const next = {
        ...state,
        definitions: { ...state.definitions, [action.definition.id]: action.definition },
        definitionPending,
        error: state.error,
      };
      if (state.openedId === action.definition.id && !state.draftDirty && !state.draftSaving) {
        return {
          ...next,
          draft: draftFromDefinition(action.definition),
          draftDirty: false,
          draftSaving: false,
        };
      }
      return next;
    }
    case 'definition/load-error':
      return {
        ...state,
        definitionPending: state.definitionPending.filter((id) => id !== action.id),
        error: state.openedId === action.id ? action.error : state.error,
      };
    case 'draft/new':
      return {
        ...state,
        openedId: null,
        draft: action.draft ?? { name: '', description: '', body: '' },
        draftDirty: Boolean(action.draft),
        draftSaving: false,
        error: null,
      };
    case 'definition/open':
      return {
        ...state,
        openedId: action.definition.id,
        definitions: { ...state.definitions, [action.definition.id]: action.definition },
        draft: draftFromDefinition(action.definition),
        draftDirty: false,
        draftSaving: false,
        error: null,
      };
    case 'definition/default':
      return { ...state, openedId: '', draft: null, draftDirty: false, error: null };
    case 'draft/cancel': {
      const definition = state.openedId ? state.definitions[state.openedId] : undefined;
      return { ...state, openedId: definition?.id ?? '', draft: definition ? draftFromDefinition(definition) : null, draftDirty: false, error: null };
    }
    case 'draft/change':
      // A failed create may have committed. Retry its exact operation payload
      // before accepting edits that would change the idempotency identity.
      if (!state.draft || (!state.draft.id && state.draft.operationId)) return state;
      return {
        ...state,
        draft: { ...state.draft, [action.field]: action.value },
        draftDirty: true,
        error: state.error?.code === 'revision_conflict' ? state.error : null,
      };
    case 'draft/save-start':
      return {
        ...state,
        draft: state.draft && action.operationId
          ? { ...state.draft, operationId: action.operationId }
          : state.draft,
        draftSaving: true,
        error: null,
      };
    case 'draft/save-success':
      return {
        ...state,
        openedId: action.definition.id,
        definitions: { ...state.definitions, [action.definition.id]: action.definition },
        draft: draftFromDefinition(action.definition),
        draftDirty: false,
        draftSaving: false,
        error: null,
      };
    case 'draft/save-error': {
      const rejected = ['invalid_mask', 'authorization_denied'].includes(String(action.error.code));
      return {
        ...state,
        draft: rejected && state.draft && !state.draft.id
          ? { ...state.draft, operationId: undefined }
          : state.draft,
        draftSaving: false,
        error: action.error,
      };
    }
    case 'delete/start':
      return { ...state, deletingId: action.id, error: null };
    case 'delete/success': {
      const definitions = { ...state.definitions };
      delete definitions[action.id];
      return {
        ...state,
        definitions,
        openedId: state.openedId === action.id ? '' : state.openedId,
        draft: state.draft?.id === action.id ? null : state.draft,
        draftDirty: state.draft?.id === action.id ? false : state.draftDirty,
        deletingId: null,
        error: null,
      };
    }
    case 'delete/error':
      return { ...state, deletingId: null, error: action.error };
  }
}

export function draftFromDefinition(definition: MaskDefinition): MaskDraft {
  return {
    id: definition.id,
    expectedRevision: definition.revision,
    name: definition.name,
    description: definition.description,
    body: definition.body,
  };
}

function unique(values: readonly string[]): string[] {
  return [...new Set(values)];
}
