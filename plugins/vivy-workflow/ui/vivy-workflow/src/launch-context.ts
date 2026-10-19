/**
 * Workflow launch eligibility, derived from host state only.
 *
 * `inofy.startRun` is admitted by the backend only when the caller names a
 * parent Run that is `active`, belongs to the live session, and still holds a
 * tool ceiling. The editor must not advertise a launch the backend will refuse,
 * so the button state and the bridge use this one predicate. It is advisory:
 * the backend stays authoritative, and its refusal is surfaced, never hidden.
 */
import type { FaceStoreState } from '@vivy/ui-sdk';

/**
 * The parent Run id a workflow launch may name, or null when the host has no
 * eligible parent. A run whose session differs from the active session is
 * treated as no parent instead of being borrowed across sessions.
 */
export function workflowLaunchParent(state: FaceStoreState): string | null {
  const sessionId = state.activeSessionId;
  if (typeof sessionId !== 'string' || sessionId === '') return null;
  const run = state.currentRun;
  if (!run || typeof run.id !== 'string' || run.id === '') return null;
  if (run.status !== 'active') return null;
  if (run.session_id !== sessionId) return null;
  return run.id;
}
