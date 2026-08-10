// Entry point: wires static shell elements to the actions, subscribes the
// renderer, and boots the initial loads plus the approval poll.

import { cancelCurrentRun, createNewSession, refreshApprovals, refreshQuestions, refreshSessions, sendMessage, toggleEventLog } from "./actions";
import { renderAll } from "./render";
import { subscribe } from "./state";

const APPROVAL_POLL_MS = 5000;

function init(): void {
  subscribe(renderAll);

  document.getElementById("new-session")?.addEventListener("click", () => void createNewSession());
  document.getElementById("cancel-run")?.addEventListener("click", () => void cancelCurrentRun());
  document.getElementById("toggle-log")?.addEventListener("click", () => toggleEventLog());

  const composer = document.getElementById("composer");
  const input = document.getElementById("composer-input") as HTMLTextAreaElement | null;
  composer?.addEventListener("submit", (ev) => {
    ev.preventDefault();
    if (!input) return;
    const text = input.value.trim();
    if (text === "") return;
    input.value = "";
    void sendMessage(text);
  });

  renderAll();
  void refreshSessions();
  void refreshApprovals();
  void refreshQuestions();
  window.setInterval(() => {
    void refreshApprovals();
    void refreshQuestions();
  }, APPROVAL_POLL_MS);
}

init();
