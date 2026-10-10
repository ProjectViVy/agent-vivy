import { describe, expect, it } from 'vitest';
import {
  initialMaskState,
  maskReducer,
  type MaskState,
} from './mask-state';

describe('maskReducer editor draft safety', () => {
  it('does not replace a new draft with a late definition response', () => {
    const definition = { id: 'custom/old', name: 'Old', description: '', body: 'old', revision: 1, digest: 'a'.repeat(64), built_in: false, generation_id: 'generation' };
    let state = maskReducer(initialMaskState, { type: 'definition/load-start', id: definition.id });
    state = maskReducer(state, { type: 'draft/new' });
    state = maskReducer(state, { type: 'draft/change', field: 'name', value: 'My new mask' });
    state = maskReducer(state, { type: 'definition/loaded', definition });
    expect(state.draft?.name).toBe('My new mask');
    expect(state.draft?.id).toBeUndefined();
    expect(state.definitions[definition.id]).toEqual(definition);
  });

  it('only opens the most recently requested definition', () => {
    const definition = { id: 'custom/old', name: 'Old', description: '', body: 'old', revision: 1, digest: 'a'.repeat(64), built_in: false, generation_id: 'generation' };
    let state = maskReducer(initialMaskState, { type: 'definition/load-start', id: definition.id });
    state = maskReducer(state, { type: 'definition/load-start', id: 'custom/new' });
    state = maskReducer(state, { type: 'definition/loaded', definition });
    expect(state.draft?.id).not.toBe(definition.id);
  });
  it('freezes an unacknowledged create payload until its operation is resolved', () => {
    let state = maskReducer(initialMaskState, { type: 'draft/new', draft: { name: 'One', description: '', body: 'Original' } });
    state = maskReducer(state, { type: 'draft/save-start', operationId: 'stable-operation' });
    state = maskReducer(state, { type: 'draft/save-error', error: { message: 'Connection lost' } });
    state = maskReducer(state, { type: 'draft/change', field: 'body', value: 'Changed' });
    expect(state.draft?.operationId).toBe('stable-operation');
    expect(state.draft?.body).toBe('Original');
  });
  it.each(['invalid_mask', 'authorization_denied'])('allows correcting a definitively rejected create: %s', (code) => {
    let state = maskReducer(initialMaskState, { type: 'draft/new', draft: { name: 'One', description: '', body: 'Original' } });
    state = maskReducer(state, { type: 'draft/save-start', operationId: 'rejected-operation' });
    state = maskReducer(state, { type: 'draft/save-error', error: { code, message: 'Rejected before write' } });
    state = maskReducer(state, { type: 'draft/change', field: 'body', value: 'Corrected' });
    expect(state.draft?.operationId).toBeUndefined();
    expect(state.draft?.body).toBe('Corrected');
  });
  it('preserves a stale editor draft when rereading a revision conflict', () => {
    const committed = {
      id: 'custom/one', name: 'One', description: '', body: 'server body', revision: 2,
      digest: 'b'.repeat(64), built_in: false, generation_id: 'generation',
    };
    let state: MaskState = maskReducer(initialMaskState, { type: 'definition/open', definition: {
      ...committed, body: 'old body', revision: 1, digest: 'a'.repeat(64),
    } });
    state = maskReducer(state, { type: 'draft/change', field: 'body', value: 'local draft' });
    state = maskReducer(state, { type: 'draft/save-start' });
    state = maskReducer(state, { type: 'draft/save-error', error: { code: 'revision_conflict', message: 'revision conflict' } });
    state = maskReducer(state, { type: 'definition/loaded', definition: committed });
    state = maskReducer(state, { type: 'draft/change', field: 'body', value: 'continued local draft' });

    expect(state.draft?.body).toBe('continued local draft');
    expect(state.draftDirty).toBe(true);
    expect(state.definitions['custom/one']).toEqual(committed);
    expect(state.error?.code).toBe('revision_conflict');
  });
});
