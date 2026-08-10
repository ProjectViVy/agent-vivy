// DOM rendering. Every function reads the shared state and rewrites its
// section; notify() after any state change triggers a full re-render.
// Event handlers live in actions.ts (no import cycle: actions never
// imports this module).

import { answerCurrentQuestion, decideApproval, deleteSession, renameSessionPrompt, selectSession } from "./actions";
import { state } from "./state";

export function renderAll(): void {
  renderSessions();
  renderRunBar();
  renderChat();
  renderEventLog();
  renderApprovalModal();
  renderQuestionModal();
  renderToasts();
}

export function renderSessions(): void {
  const list = document.getElementById("session-list");
  if (!list) return;
  list.textContent = "";
  for (const sess of state.sessions) {
    const li = document.createElement("li");
    if (sess.id === state.currentSessionID) li.classList.add("selected");

    const title = document.createElement("span");
    title.textContent = sess.title;
    title.addEventListener("click", () => void selectSession(sess.id));
    li.appendChild(title);

    const rename = document.createElement("button");
    rename.type = "button";
    rename.textContent = "rename";
    rename.addEventListener("click", () => void renameSessionPrompt(sess.id));
    li.appendChild(rename);

    const del = document.createElement("button");
    del.type = "button";
    del.textContent = "delete";
    del.addEventListener("click", () => void deleteSession(sess.id));
    li.appendChild(del);

    list.appendChild(li);
  }
}

function isTerminalStatus(status: string): boolean {
  return status === "completed" || status === "failed" || status === "cancelled";
}

function runStatusLabel(): string {
  const run = state.run;
  if (!run) return "";
  switch (run.status) {
    case "completed":
      return "run completed";
    case "failed":
      return "run failed";
    case "cancelled":
      return "run cancelled";
    default:
      return `run ${run.status}…`;
  }
}

export function renderRunBar(): void {
  const status = document.getElementById("run-status");
  const cancel = document.getElementById("cancel-run") as HTMLButtonElement | null;
  const toggle = document.getElementById("toggle-log") as HTMLButtonElement | null;
  if (!status || !cancel || !toggle) return;

  status.textContent = runStatusLabel();
  const active = state.run !== null && !isTerminalStatus(state.run.status);
  cancel.hidden = !active;
  toggle.hidden = state.run === null;
  toggle.textContent = state.eventLogVisible ? "Hide event log" : "Show event log";
}

export function renderChat(): void {
  const chat = document.getElementById("chat");
  if (!chat) return;
  chat.textContent = "";

  if (state.currentSessionID === null) {
    const hint = document.createElement("div");
    hint.className = "bubble assistant";
    hint.textContent = "Create or select a session to start.";
    chat.appendChild(hint);
  }

  for (const msg of state.messages) {
    const bubble = document.createElement("div");
    bubble.className = `bubble ${msg.role}`;
    bubble.textContent = msg.content;
    chat.appendChild(bubble);
  }
  if (state.streamingText !== "") {
    const bubble = document.createElement("div");
    bubble.className = "bubble assistant";
    bubble.textContent = state.streamingText;
    chat.appendChild(bubble);
  }
  if (state.run?.status === "failed") {
    const bubble = document.createElement("div");
    bubble.className = "bubble error";
    bubble.textContent = lastFailureText();
    chat.appendChild(bubble);
  }
  chat.scrollTop = chat.scrollHeight;
}

// lastFailureText pulls the structured cause from the run.failed event;
// FR-11 demands the category stays visible to the user.
function lastFailureText(): string {
  for (let i = state.eventLog.length - 1; i >= 0; i--) {
    const ev = state.eventLog[i];
    if (ev.type === "run.failed") {
      const cause = String(ev.payload["cause_category"] ?? "internal_error");
      const message = String(ev.payload["message"] ?? "");
      return `Run failed (${cause}): ${message}`;
    }
  }
  return "Run failed.";
}

export function renderEventLog(): void {
  const panel = document.getElementById("event-log");
  const list = document.getElementById("event-list");
  if (!panel || !list) return;
  panel.hidden = !state.eventLogVisible || state.run === null;
  if (panel.hidden) return;
  list.textContent = "";
  for (const ev of state.eventLog) {
    const li = document.createElement("li");
    li.textContent = `#${ev.seq} ${ev.type} ${payloadSummary(ev.payload)}`;
    list.appendChild(li);
  }
}

function payloadSummary(payload: Record<string, unknown>): string {
  const parts: string[] = [];
  for (const [key, value] of Object.entries(payload)) {
    let text: string;
    if (typeof value === "string") {
      text = value.length > 60 ? `${value.slice(0, 57)}...` : value;
    } else {
      text = JSON.stringify(value);
      if (text.length > 60) text = `${text.slice(0, 57)}...`;
    }
    parts.push(`${key}=${text}`);
  }
  return parts.join(" ");
}

export function renderApprovalModal(): void {
  const root = document.getElementById("modal-root");
  if (!root) return;
  root.textContent = "";
  const approval = state.pendingApproval;
  if (!approval) return;

  const backdrop = document.createElement("div");
  backdrop.className = "modal-backdrop";
  const modal = document.createElement("div");
  modal.className = "modal";

  const heading = document.createElement("h2");
  heading.textContent = "Approval required";
  modal.appendChild(heading);

  const detail = document.createElement("p");
  detail.textContent = `Tool "${approval.tool_name}" wants to run.`;
  modal.appendChild(detail);

  const args = document.createElement("pre");
  args.textContent = JSON.stringify(approval.args, null, 2);
  modal.appendChild(args);

  const expiry = document.createElement("p");
  expiry.textContent = `Expires ${new Date(approval.expires_at).toLocaleTimeString()}.`;
  modal.appendChild(expiry);

  const approve = document.createElement("button");
  approve.type = "button";
  approve.textContent = "Approve";
  approve.addEventListener("click", () => void decideApproval(approval.approval_id, "approved"));
  modal.appendChild(approve);

  const deny = document.createElement("button");
  deny.type = "button";
  deny.textContent = "Deny";
  deny.addEventListener("click", () => void decideApproval(approval.approval_id, "denied"));
  modal.appendChild(deny);

  backdrop.appendChild(modal);
  root.appendChild(backdrop);
}

export function renderQuestionModal(): void {
  const root = document.getElementById("modal-root");
  if (!root) return;
  if (state.pendingApproval) return;
  // Preserve an approval modal if it is present; otherwise rebuild this
  // independent user-input suspension from durable question state.
  const existing = root.querySelector(".question-modal");
  if (existing) existing.remove();
  const question = state.pendingQuestion;
  if (!question) return;

  const backdrop = document.createElement("div");
  backdrop.className = "modal-backdrop question-modal";
  const modal = document.createElement("div");
  modal.className = "modal";
  const heading = document.createElement("h2");
  heading.textContent = "Vivy needs an answer";
  modal.appendChild(heading);
  const prompt = document.createElement("p");
  prompt.textContent = question.prompt;
  modal.appendChild(prompt);
  const form = document.createElement("form");
  const input = document.createElement("textarea");
  input.rows = 3;
  input.required = true;
  input.placeholder = "Type your answer";
  form.appendChild(input);
  const submit = document.createElement("button");
  submit.type = "submit";
  submit.textContent = "Answer";
  form.appendChild(submit);
  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    const answer = input.value.trim();
    if (answer !== "") void answerCurrentQuestion(answer);
  });
  modal.appendChild(form);
  const expiry = document.createElement("p");
  expiry.textContent = `Expires ${new Date(question.expires_at).toLocaleTimeString()}.`;
  modal.appendChild(expiry);
  backdrop.appendChild(modal);
  root.appendChild(backdrop);
}

export function renderToasts(): void {
  const root = document.getElementById("toast-root");
  if (!root) return;
  root.textContent = "";
  for (const toast of state.toasts) {
    const el = document.createElement("div");
    el.className = "toast";
    el.textContent = toast.message;
    root.appendChild(el);
  }
}
