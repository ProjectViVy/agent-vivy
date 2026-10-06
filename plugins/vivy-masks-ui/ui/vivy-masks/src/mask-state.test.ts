import { describe, expect, it } from 'vitest';
import {
  initialMaskState,
  maskReducer,
  type MaskState,
} from './mask-state';

describe('maskReducer editor draft safety', () => {
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

    expect(state.draft?.body).toBe('local draft');
    expect(state.draftDirty).toBe(true);
    expect(state.definitions['custom/one']).toEqual(committed);
    expect(state.error?.code).toBe('revision_conflict');
  });
});
