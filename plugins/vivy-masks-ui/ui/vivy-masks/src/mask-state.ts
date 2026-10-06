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
}

export interface MaskState {
  readonly definitions: Readonly<Record<string, MaskDefinition>>;
  readonly definitionPending: readonly string[];
  readonly draft: MaskDraft | null;
  readonly draftDirty: boolean;
  readonly draftSaving: boolean;
  readonly deletingId: string | null;
  readonly error: MaskUIError | null;
}

export const initialMaskState: MaskState = Object.freeze({
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
  | { readonly type: 'draft/change'; readonly field: 'name' | 'description' | 'body'; readonly value: string }
  | { readonly type: 'draft/save-start'; readonly operationId?: string }
  | { readonly type: 'draft/save-success'; readonly definition: MaskDefinition }
  | { readonly type: 'draft/save-error'; readonly error: MaskUIError }
  | { readonly type: 'delete/start'; readonly id: string }
  | { readonly type: 'delete/success'; readonly id: string }
  | { readonly type: 'delete/error'; readonly error: MaskUIError };

export function maskReducer(state: MaskState, action: MaskAction): MaskState {
  switch (action.type) {
    case 'definition/load-start':
      return { ...state, definitionPending: unique([...state.definitionPending, action.id]), error: null };
    case 'definition/loaded': {
      const definitionPending = state.definitionPending.filter((id) => id !== action.definition.id);
      const next = {
        ...state,
        definitions: { ...state.definitions, [action.definition.id]: action.definition },
        definitionPending,
        error: state.error,
      };
      if (!state.draftDirty || state.draft?.id !== action.definition.id) {
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
        error: action.error,
      };
    case 'draft/new':
      return {
        ...state,
        draft: action.draft ?? { name: '', description: '', body: '' },
        draftDirty: false,
        draftSaving: false,
        error: null,
      };
    case 'definition/open':
      return {
        ...state,
        definitions: { ...state.definitions, [action.definition.id]: action.definition },
        draft: draftFromDefinition(action.definition),
        draftDirty: false,
        draftSaving: false,
        error: null,
      };
    case 'draft/change':
      if (!state.draft) return state;
      return {
        ...state,
        draft: { ...state.draft, [action.field]: action.value },
        draftDirty: true,
        error: null,
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
        definitions: { ...state.definitions, [action.definition.id]: action.definition },
        draft: draftFromDefinition(action.definition),
        draftDirty: false,
        draftSaving: false,
        error: null,
      };
    case 'draft/save-error':
      return { ...state, draftSaving: false, error: action.error };
    case 'delete/start':
      return { ...state, deletingId: action.id, error: null };
    case 'delete/success': {
      const definitions = { ...state.definitions };
      delete definitions[action.id];
      return {
        ...state,
        definitions,
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
