import type { AppController } from "../../app/controller";
import { translate } from "../../app/i18n";
import { state, isRunActive } from "../../app/store";
import type { ShellElements } from "../../app/shell";
import { formatDate, node } from "../../ui/dom";

let lastMessageSignature = "";

function runLabel(status: string): string {
  switch (status) {
    case "accepted": return translate(state.locale, "runAccepted");
    case "queued": return translate(state.locale, "runQueued");
    case "active": return translate(state.locale, "runActive");
    case "completed": return translate(state.locale, "runCompleted");
    case "failed": return translate(state.locale, "runFailed");
    case "cancelled": return translate(state.locale, "runCancelled");
    default: return status;
  }
}

function failureText(): string {
  const event = [...state.events].reverse().find((item) => item.type === "run.failed");
  if (!event) return translate(state.locale, "runFailed");
  const cause = String(event.payload.cause_category ?? "internal_error");
  const causeKey = cause === "provider_error" ? "providerError" : cause === "tool_error" ? "toolError" : cause === "cancelled" ? "cancelledError" : cause === "internal_error" ? "internalError" : "unknownError";
  return `${translate(state.locale, "runFailedCause", { cause: translate(state.locale, causeKey) })}: ${String(event.payload.message ?? "")}`;
}

export function renderConversation(shell: ShellElements, controller: AppController): void {
  const session = state.sessions.find((item) => item.id === state.currentSessionID);
  shell.sessionTitle.textContent = session?.title ?? translate(state.locale, "appName");
  shell.sessionMeta.textContent = session
    ? translate(state.locale, "sessionCreated", { value: formatDate(session.created_at, state.locale === "zh-CN" ? "zh-CN" : "en-US") })
    : translate(state.locale, "selectSessionHint");

  if (state.run) {
    shell.runStatus.textContent = runLabel(state.run.status);
    shell.runStatus.dataset.status = state.run.status;
  } else {
    shell.runStatus.textContent = "";
    delete shell.runStatus.dataset.status;
  }
  const active = isRunActive(state.run);
  shell.cancelRun.hidden = !active;
  shell.cancelRun.disabled = state.cancelBusy;
  shell.cancelRun.textContent = state.cancelBusy ? translate(state.locale, "cancelling") : translate(state.locale, "cancelRun");
  shell.inspectorToggle.hidden = state.run === null;
  shell.inspectorToggle.textContent = state.inspectorOpen ? translate(state.locale, "hideInspector") : translate(state.locale, "showInspector");

  const signature = [state.locale, state.currentSessionID, state.messagesPhase, state.messagesError, state.streamingText, state.run?.status, state.events.length, ...state.messages.map((item) => `${item.id}:${item.run_id ?? ""}:${item.content}`)].join("|");
  if (signature !== lastMessageSignature) {
    lastMessageSignature = signature;
    renderMessages(shell, controller);
  }
  renderComposer(shell, active);
  const appShell = shell.sidebar.closest(".app-shell");
  appShell?.classList.toggle("sidebar-open", state.sidebarOpen);
  appShell?.classList.toggle("inspector-open", state.inspectorOpen && state.run !== null);
}

function renderMessages(shell: ShellElements, controller: AppController): void {
  shell.chat.replaceChildren();
  if (!state.currentSessionID) {
    const welcome = node("div", "welcome-state");
    const title = node("h2");
    title.textContent = translate(state.locale, "selectSessionHint");
    welcome.appendChild(title);
    shell.chat.appendChild(welcome);
    return;
  }
  if (state.messagesPhase === "loading") {
    for (let index = 0; index < 3; index += 1) {
      const skeleton = node("div", "message-skeleton");
      skeleton.innerHTML = "<span></span><span></span><span></span>";
      shell.chat.appendChild(skeleton);
    }
    return;
  }
  if (state.messagesPhase === "error") {
    const error = node("div", "inline-state inline-state-error");
    const message = node("p");
    message.textContent = state.messagesError;
    const retry = node("button", "button button-secondary button-small");
    retry.type = "button";
    retry.textContent = translate(state.locale, "retry");
    retry.addEventListener("click", () => {
      if (state.currentSessionID) void controller.selectSession(state.currentSessionID);
    });
    error.append(message, retry);
    shell.chat.appendChild(error);
    return;
  }
  if (state.messages.length === 0 && !state.streamingText) {
    const empty = node("div", "empty-state");
    const title = node("h2");
    title.textContent = translate(state.locale, "noMessages");
    const hint = node("p");
    hint.textContent = translate(state.locale, "noMessagesHint");
    empty.append(title, hint);
    shell.chat.appendChild(empty);
  }
  for (const message of state.messages) {
    const article = node("article", `message message-${message.role}`);
    const content = node("div", "message-content");
    content.textContent = message.content;
    const meta = node("time", "message-meta");
    meta.dateTime = new Date(message.created_at).toISOString();
    meta.textContent = formatDate(message.created_at, state.locale === "zh-CN" ? "zh-CN" : "en-US", { dateStyle: "short", timeStyle: "short" });
    const metaRow = node("div", "message-meta-row");
    metaRow.appendChild(meta);
    if (message.run_id && state.currentSessionID) {
      const openRun = node("button", "message-run-link");
      openRun.type = "button";
      openRun.textContent = translate(state.locale, "openRun");
      openRun.addEventListener("click", () => void controller.openRun(message.run_id ?? "", state.currentSessionID ?? ""));
      metaRow.appendChild(openRun);
    }
    article.append(content, metaRow);
    shell.chat.appendChild(article);
  }
  if (state.streamingText) {
    const article = node("article", "message message-assistant message-streaming");
    const content = node("div", "message-content");
    content.textContent = state.streamingText;
    const meta = node("span", "message-meta");
    meta.textContent = translate(state.locale, "runActive");
    article.append(content, meta);
    shell.chat.appendChild(article);
  }
  if (state.run?.status === "failed") {
    const error = node("article", "message message-error");
    error.textContent = failureText();
    shell.chat.appendChild(error);
  }
  shell.chat.scrollTop = shell.chat.scrollHeight;
}

function renderComposer(shell: ShellElements, active: boolean): void {
  shell.composerInput.placeholder = translate(state.locale, "sendPlaceholder");
  shell.modeLabel.textContent = translate(state.locale, "planMode");
  shell.modeToggle.checked = state.mode === "plan";
  shell.modeToggle.disabled = !state.currentSessionID || state.sendPhase === "processing" || active;
  shell.composerInput.disabled = !state.currentSessionID || state.sendPhase === "processing" || active;
  shell.sendButton.disabled = !state.currentSessionID || !state.draft.trim() || state.sendPhase === "processing" || active;
  shell.sendButton.textContent = state.sendPhase === "processing" ? translate(state.locale, "loading") : translate(state.locale, "send");
  const error = document.getElementById("composer-error");
  if (error) {
    error.hidden = !state.sendError;
    error.textContent = state.sendError;
  }
}
