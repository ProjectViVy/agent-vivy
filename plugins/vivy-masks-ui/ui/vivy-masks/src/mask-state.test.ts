import { describe, expect, it } from 'vitest';
import {
  initialMaskState,
  maskReducer,
  type MaskState,
} from './mask-state';

describe('maskReducer session epoch and draft safety', () => {
  it('ignores a late session A response after the host switched to session B', () => {
    let state = initialMaskState;
    state = maskReducer(state, { type: 'session/change', epoch: 1, sessionId: 'session-a' });
    state = maskReducer(state, { type: 'session/load-start', epoch: 1, sessionId: 'session-a' });
    state = maskReducer(state, { type: 'session/change', epoch: 2, sessionId: 'session-b' });
    state = maskReducer(state, { type: 'session/load-success', epoch: 2, selection: {
      session_id: 'session-b', mask_id: 'builtin/writer', revision: 7, available: true, inactive_reason: '',
    } });
    state = maskReducer(state, { type: 'session/load-success', epoch: 1, selection: {
      session_id: 'session-a', mask_id: 'builtin/programmer', revision: 41, available: true, inactive_reason: '',
    } });

    expect(state.activeSessionId).toBe('session-b');
    expect(state.selection).toMatchObject({ session_id: 'session-b', revision: 7 });
    expect(state.selection?.revision).not.toBe(41);
  });

  it('keeps the current session revision as the CAS input and ignores stale selection writes', () => {
    let state = sessionState('session-b');
    state = maskReducer(state, { type: 'session/load-success', epoch: 3, selection: {
      session_id: 'session-b', mask_id: 'builtin/writer', revision: 7, available: true, inactive_reason: '',
    } });
    state = maskReducer(state, { type: 'selection/save-start', epoch: 3, sessionId: 'session-b' });
    state = maskReducer(state, { type: 'session/change', epoch: 4, sessionId: 'session-c' });
    state = maskReducer(state, { type: 'selection/save-success', epoch: 3, sessionId: 'session-b', selection: {
      session_id: 'session-b', mask_id: 'custom/one', revision: 8, available: true, inactive_reason: '',
    } });

    expect(state.activeSessionId).toBe('session-c');
    expect(state.selection).toBeNull();
    expect(state.selectionPending).toBe(true);
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

    expect(state.draft?.body).toBe('local draft');
    expect(state.draftDirty).toBe(true);
    expect(state.definitions['custom/one']).toEqual(committed);
    expect(state.error?.code).toBe('revision_conflict');
  });
});

function sessionState(sessionId: string): MaskState {
  return maskReducer(initialMaskState, { type: 'session/change', epoch: 3, sessionId });
}
