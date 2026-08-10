// Minimal state container: one mutable record plus a listener list. Every
// view re-renders from this state, and every field is reconstructable
// from the API, which keeps refreshes lossless (FR-9, AS-7).

import type { Approval, Message, Question, Run, Session } from "./api";
import type { EventEnvelope } from "./sse";

// PendingApproval carries the fields of a tool.approval_required payload
// the modal needs; the authoritative row stays server-side (D-009).
export interface PendingApproval {
  approval_id: string;
  run_id: string;
  tool_name: string;
  args: Record<string, unknown>;
  expires_at: number;
}

export interface Toast {
  id: number;
  message: string;
}

export interface AppState {
  sessions: Session[];
  currentSessionID: string | null;
  messages: Message[];
  // Live streaming text of the in-flight assistant reply, if any.
  streamingText: string;
  run: Run | null;
  approvals: Approval[];
  // Modal shown for the run the UI is currently watching, if any.
  pendingApproval: PendingApproval | null;
  pendingQuestion: Question | null;
  eventLog: EventEnvelope[];
  eventLogVisible: boolean;
  toasts: Toast[];
}

export const state: AppState = {
  sessions: [],
  currentSessionID: null,
  messages: [],
  streamingText: "",
  run: null,
  approvals: [],
  pendingApproval: null,
  pendingQuestion: null,
  eventLog: [],
  eventLogVisible: false,
  toasts: [],
};

type Listener = () => void;

const listeners = new Set<Listener>();

export function subscribe(fn: Listener): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

export function notify(): void {
  for (const fn of listeners) fn();
}

// resetSession clears everything bound to the previously selected session.
export function resetSession(): void {
  state.messages = [];
  state.streamingText = "";
  state.run = null;
  state.pendingApproval = null;
  state.pendingQuestion = null;
  state.eventLog = [];
  state.eventLogVisible = false;
}
