import { describe, expect, it } from 'vitest';
import type { FaceStoreState } from '@vivy/ui-sdk';
import { workflowLaunchParent } from './launch-context';

function state(overrides: Partial<FaceStoreState> = {}): FaceStoreState {
  return {
    activeSessionId: 'sess-1',
    currentRun: { id: 'run-1', session_id: 'sess-1', status: 'active', created_at: 1 },
    connection: 'ready',
    ...overrides,
  } as unknown as FaceStoreState;
}

describe('workflowLaunchParent', () => {
  it('names the active run of the active session', () => {
    expect(workflowLaunchParent(state())).toBe('run-1');
  });

  it('has no parent without an active session', () => {
    expect(workflowLaunchParent(state({ activeSessionId: null }))).toBeNull();
    expect(workflowLaunchParent(state({ activeSessionId: '' }))).toBeNull();
  });

  it('has no parent without a current run', () => {
    expect(workflowLaunchParent(state({ currentRun: null }))).toBeNull();
  });

  it('has no parent for a terminal or otherwise inactive run', () => {
    for (const status of ['accepted', 'queued', 'completed', 'failed', 'cancelled'] as const) {
      expect(workflowLaunchParent(state({
        currentRun: { id: 'run-1', session_id: 'sess-1', status, created_at: 1 },
      }))).toBeNull();
    }
  });

  it('never borrows a run from another session', () => {
    expect(workflowLaunchParent(state({
      currentRun: { id: 'run-1', session_id: 'sess-2', status: 'active', created_at: 1 },
    }))).toBeNull();
  });

  it('treats a run without an id as no parent', () => {
    expect(workflowLaunchParent(state({
      currentRun: { id: '', session_id: 'sess-1', status: 'active', created_at: 1 },
    }))).toBeNull();
  });
});
