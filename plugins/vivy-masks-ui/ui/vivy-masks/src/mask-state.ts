import type { MaskDefinition, MaskMetadata, MaskSelection } from './mask-client';

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
  readonly epoch: number;
  readonly activeSessionId: string | null;
  readonly catalog: readonly MaskMetadata[];
  readonly definitions: Readonly<Record<string, MaskDefinition>>;
  readonly selection: MaskSelection | null;
  readonly selectionPending: boolean;
  readonly catalogPending: boolean;
  readonly definitionPending: readonly string[];
  readonly draft: MaskDraft | null;
  readonly draftDirty: boolean;
  readonly draftSaving: boolean;
  readonly deletingId: string | null;
  readonly error: MaskUIError | null;
}

export const initialMaskState: MaskState = Object.freeze({
  epoch: 0,
  activeSessionId: null,
  catalog: [],
  definitions: {},
  selection: null,
  selectionPending: false,
  catalogPending: false,
  definitionPending: [],
  draft: null,
  draftDirty: false,
  draftSaving: false,
  deletingId: null,
  error: null,
});

export type MaskAction =
  | { readonly type: 'catalog/load-start' }
  | { readonly type: 'catalog/load-success'; readonly items: readonly MaskMetadata[] }
  | { readonly type: 'catalog/load-error'; readonly error: MaskUIError }
  | { readonly type: 'session/change'; readonly epoch: number; readonly sessionId: string | null }
  | { readonly type: 'session/load-start'; readonly epoch: number; readonly sessionId: string }
  | { readonly type: 'session/load-success'; readonly epoch: number; readonly selection: MaskSelection }
  | { readonly type: 'session/load-error'; readonly epoch: number; readonly sessionId: string; readonly error: MaskUIError }
  | { readonly type: 'selection/save-start'; readonly epoch: number; readonly sessionId: string }
  | { readonly type: 'selection/save-success'; readonly epoch: number; readonly sessionId: string; readonly selection: MaskSelection }
  | { readonly type: 'selection/save-error'; readonly epoch: number; readonly sessionId: string; readonly error: MaskUIError }
  | { readonly type: 'definition/load-start'; readonly id: string }
  | { readonly type: 'definition/loaded'; readonly definition: MaskDefinition }
  | { readonly type: 'definition/load-error'; readonly id: string; readonly error: MaskUIError }
  | { readonly type: 'draft/new' }
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
    case 'catalog/load-start':
      return { ...state, catalogPending: true, error: null };
    case 'catalog/load-success':
      return { ...state, catalogPending: false, catalog: [...action.items], error: null };
    case 'catalog/load-error':
      return { ...state, catalogPending: false, error: action.error };
    case 'session/change':
      return {
        ...state,
        epoch: action.epoch,
        activeSessionId: action.sessionId,
        selection: null,
        selectionPending: Boolean(action.sessionId),
        error: null,
      };
    case 'session/load-start':
      if (!matchesSession(state, action.epoch, action.sessionId)) return state;
      return { ...state, selectionPending: true, error: null };
    case 'session/load-success':
      if (state.epoch !== action.epoch || state.activeSessionId !== action.selection.session_id) return state;
      return { ...state, selection: action.selection, selectionPending: false, error: null };
    case 'session/load-error':
      if (!matchesSession(state, action.epoch, action.sessionId)) return state;
      return { ...state, selectionPending: false, error: action.error };
    case 'selection/save-start':
      if (!matchesSession(state, action.epoch, action.sessionId)) return state;
      return {
        ...state,
        selectionPending: true,
        error: null,
      };
    case 'selection/save-success':
      if (!matchesSession(state, action.epoch, action.sessionId) || action.selection.session_id !== action.sessionId) return state;
      return { ...state, selection: action.selection, selectionPending: false, error: null };
    case 'selection/save-error':
      if (!matchesSession(state, action.epoch, action.sessionId)) return state;
      return { ...state, selectionPending: false, error: action.error };
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
        draft: { name: '', description: '', body: '' },
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
        catalog: state.catalog.filter((item) => item.id !== action.id),
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

function matchesSession(state: MaskState, epoch: number, sessionId: string): boolean {
  return state.epoch === epoch && state.activeSessionId === sessionId;
}

function unique(values: readonly string[]): string[] {
  return [...new Set(values)];
}
