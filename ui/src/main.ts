// Entry point. Commit one boots the shell and renders the session list;
// the remaining views (chat, approvals, event log) land next.

import { listSessions } from "./api";
import { notify, state, subscribe } from "./state";

function renderSessions(): void {
  const list = document.getElementById("session-list");
  if (!list) return;
  list.textContent = "";
  for (const sess of state.sessions) {
    const li = document.createElement("li");
    li.textContent = sess.title;
    if (sess.id === state.currentSessionID) li.classList.add("selected");
    list.appendChild(li);
  }
}

async function loadSessions(): Promise<void> {
  const { sessions } = await listSessions();
  state.sessions = sessions;
  notify();
}

subscribe(renderSessions);
void loadSessions();
