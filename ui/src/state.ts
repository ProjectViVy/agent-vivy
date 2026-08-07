// Minimal state container: one mutable record plus a listener list. Every
// view re-renders from this state, and every field is reconstructable
// from the API, which keeps refreshes lossless (FR-9, AS-7).

import type { Approval, Message, Run, Session } from "./api";
import type { EventEnvelope } from "./sse";

export interface AppState {
  sessions: Session[];
  currentSessionID: string | null;
  messages: Message[];
  // Live streaming text of the in-flight assistant reply, if any.
  streamingText: string;
  run: Run | null;
  approvals: Approval[];
  eventLog: EventEnvelope[];
}

export const state: AppState = {
  sessions: [],
  currentSessionID: null,
  messages: [],
  streamingText: "",
  run: null,
  approvals: [],
  eventLog: [],
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
  state.eventLog = [];
}
