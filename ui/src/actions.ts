// All user-facing actions. Mutates state, calls the API, and manages the
// single run subscription. Rendering happens via notify() -> renderAll.

import * as api from "./api";
import { ApiError, type Decision, type Message, type RunMode } from "./api";
import { notify, resetSession, state } from "./state";
import { subscribeRun, TERMINAL_EVENT_TYPES, type EventEnvelope, type RunSubscription } from "./sse";

let sub: RunSubscription | null = null;
let toastSeq = 0;

export function toast(message: string): void {
  const id = ++toastSeq;
  state.toasts.push({ id, message });
  notify();
  window.setTimeout(() => {
    state.toasts = state.toasts.filter((t) => t.id !== id);
    notify();
  }, 6000);
}

function toastOf(err: unknown): void {
  if (err instanceof ApiError) {
    toast(`${err.code}: ${err.message}`);
    return;
  }
  toast(String(err));
}

// --- sessions ---

export async function refreshSessions(): Promise<void> {
  try {
    const { sessions } = await api.listSessions();
    state.sessions = sessions;
    notify();
  } catch (err) {
    toastOf(err);
  }
}

export async function createNewSession(): Promise<void> {
  try {
    const sess = await api.createSession("");
    await refreshSessions();
    await selectSession(sess.id);
  } catch (err) {
    toastOf(err);
  }
}

export async function selectSession(id: string): Promise<void> {
  if (state.currentSessionID === id) return;
  stopSubscription();
  resetSession();
  state.currentSessionID = id;
  notify();
  try {
    const { messages } = await api.listMessages(id);
    // The user may have switched away while the request was in flight.
    if (state.currentSessionID !== id) return;
    state.messages = messages;
    notify();
    await recoverRun(messages);
    await refreshQuestions();
  } catch (err) {
    toastOf(err);
  }
}

export async function renameSessionPrompt(id: string): Promise<void> {
  const title = window.prompt("New session title");
  if (title === null || title.trim() === "") return;
  try {
    await api.renameSession(id, title.trim());
    await refreshSessions();
  } catch (err) {
    toastOf(err);
  }
}

export async function deleteSession(id: string): Promise<void> {
  if (!window.confirm("Delete this session with all its messages and runs?")) return;
  try {
    await api.deleteSession(id);
    if (state.currentSessionID === id) {
      stopSubscription();
      resetSession();
      state.currentSessionID = null;
    }
    await refreshSessions();
  } catch (err) {
    toastOf(err);
  }
}

// --- runs ---

export async function sendMessage(text: string): Promise<void> {
  const sessionID = state.currentSessionID;
  if (!sessionID) {
    toast("select a session first");
    return;
  }
  try {
    const planToggle = document.getElementById("plan-mode") as HTMLInputElement | null;
    const mode: RunMode = planToggle?.checked ? "plan" : "normal";
    const check = await api.preflight(sessionID, text, mode);
    if (check.status === "blocked") {
      toast(`preflight blocked: ${(check.blockers ?? []).join("; ")}`);
      return;
    }
    const warnings = check.warnings ?? [];
    if (warnings.length > 0 && !window.confirm(`Preflight warnings:\n\n${warnings.join("\n")}\n\nContinue?`)) {
      return;
    }
    const accepted = await api.postMessage(sessionID, text, mode);
    state.run = {
      id: accepted.run_id,
      session_id: sessionID,
      status: accepted.status,
      created_at: Date.now(),
    };
    state.streamingText = "";
    state.pendingQuestion = null;
    state.eventLog = [];
    state.eventLogVisible = true;
    notify();
    // Optimistic user bubble; the reload after the terminal replaces it
    // with the persisted copy.
    state.messages = [...state.messages, { id: "local", role: "user", content: text, created_at: Date.now() }];
    notify();
    startSubscription(accepted.run_id, 0);
  } catch (err) {
    toastOf(err);
  }
}

export async function cancelCurrentRun(): Promise<void> {
  const run = state.run;
  if (!run) return;
  try {
    await api.cancelRun(run.id);
  } catch (err) {
    toastOf(err);
  }
}

export function toggleEventLog(): void {
  state.eventLogVisible = !state.eventLogVisible;
  notify();
}

// --- JSON-RPC event wiring ---

function stopSubscription(): void {
  if (sub) {
    sub.close();
    sub = null;
  }
}

function startSubscription(runID: string, afterSeq: number): void {
  stopSubscription();
  sub = subscribeRun(runID, afterSeq, handleEvent, (message) => toast(message));
}

function handleEvent(env: EventEnvelope): void {
  // Ignore frames of a run the user navigated away from.
  if (state.run === null || state.run.id !== env.run_id) return;
  state.eventLog.push(env);
  switch (env.type) {
    case "run.started":
      state.run = { ...state.run, status: "active" };
      break;
    case "model.delta":
      state.streamingText += String(env.payload["delta"] ?? "");
      break;
    case "tool.approval_required":
      state.pendingApproval = {
        approval_id: String(env.payload["approval_id"] ?? ""),
        run_id: env.run_id,
        tool_name: String(env.payload["tool_name"] ?? ""),
        args: (env.payload["args"] as Record<string, unknown> | undefined) ?? {},
        expires_at: Number(env.payload["expires_at"] ?? 0),
      };
      void refreshApprovals();
      break;
    case "user.question_required":
      state.pendingQuestion = {
        id: String(env.payload["question_id"] ?? ""),
        run_id: env.run_id,
        tool_call_id: String(env.payload["tool_call_id"] ?? ""),
        prompt: String(env.payload["prompt"] ?? ""),
        expires_at: Number(env.payload["expires_at"] ?? 0),
      };
      void refreshQuestions();
      break;
    case "user.question_answered":
      if (state.pendingQuestion?.id === String(env.payload["question_id"] ?? "")) {
        state.pendingQuestion = null;
      }
      void refreshQuestions();
      break;
    case "run.completed":
      state.run = { ...state.run, status: "completed" };
      state.streamingText = "";
      void refreshMessages();
      void refreshApprovals();
      void refreshQuestions();
      break;
    case "run.failed":
      state.run = { ...state.run, status: "failed" };
      state.streamingText = "";
      void refreshApprovals();
      void refreshQuestions();
      break;
    case "run.cancelled":
      state.run = { ...state.run, status: "cancelled" };
      state.streamingText = "";
      void refreshApprovals();
      void refreshQuestions();
      break;
    default:
      break;
  }
  notify();
}

async function refreshMessages(): Promise<void> {
  const sessionID = state.currentSessionID;
  if (!sessionID) return;
  try {
    const { messages } = await api.listMessages(sessionID);
    if (state.currentSessionID !== sessionID) return;
    state.messages = messages;
    notify();
  } catch (err) {
    toastOf(err);
  }
}

// recoverRun reattaches after a refresh: the last message carrying a
// run_id identifies the newest run; a non-terminal run gets a full RPC
// replay (after_seq=0) so the event log rebuilds and live frames resume.
async function recoverRun(messages: Message[]): Promise<void> {
  let runID = "";
  for (let i = messages.length - 1; i >= 0; i--) {
    if (messages[i].run_id) {
      runID = messages[i].run_id!;
      break;
    }
  }
  if (!runID) return;
  try {
    const run = await api.getRun(runID);
    if (state.currentSessionID !== run.session_id) return;
    state.run = run;
    await refreshQuestions();
    notify();
    if (!TERMINAL_EVENT_TYPES.has(`run.${run.status}`)) {
      state.eventLogVisible = true;
      startSubscription(runID, 0);
    }
  } catch (err) {
    toastOf(err);
  }
}

// --- approvals ---

export async function decideApproval(approvalID: string, decision: Decision): Promise<void> {
  try {
    await api.decideApproval(approvalID, decision);
    state.pendingApproval = null;
    notify();
  } catch (err) {
    if (err instanceof ApiError && (err.status === 404 || err.status === 409)) {
      // Already decided or expired server-side: the modal is stale.
      toast(`${err.code}: ${err.message}`);
      state.pendingApproval = null;
      notify();
      return;
    }
    toastOf(err);
  }
  void refreshApprovals();
}

export async function refreshApprovals(): Promise<void> {
  try {
    const { approvals } = await api.listApprovals();
    state.approvals = approvals;
    notify();
  } catch {
    // Polling hiccup: the next tick retries; no toast spam.
  }
}

export async function answerCurrentQuestion(answer: string): Promise<void> {
  const question = state.pendingQuestion;
  if (!question) return;
  try {
    await api.answerQuestion(question.id, answer);
    state.pendingQuestion = null;
    notify();
  } catch (err) {
    if (err instanceof ApiError && (err.status === 404 || err.status === 409)) {
      state.pendingQuestion = null;
      notify();
    }
    toastOf(err);
  }
  void refreshQuestions();
}

export async function refreshQuestions(): Promise<void> {
  try {
    const { questions } = await api.listQuestions();
    const runID = state.run?.id;
    state.pendingQuestion = runID ? questions.find((q) => q.run_id === runID) ?? null : null;
    notify();
  } catch {
    // Polling hiccup: RPC notifications or the next tick will repair the modal.
  }
}
