import type { AppController } from "../../app/controller";
import { translate } from "../../app/i18n";
import { state } from "../../app/store";
import { formatDate, node } from "../../ui/dom";
import { FloatingMenu } from "../../ui/menu";
import type { ShellElements } from "../../app/shell";

let lastSignature = "";
const menu = new FloatingMenu();

export function renderSessions(shell: ShellElements, controller: AppController): void {
  const locale = state.locale === "zh-CN" ? "zh-CN" : "en-US";
  const activeKey = state.activeRuns.map((run) => `${run.id}:${run.session_id}:${run.status}:${run.created_at}`).join(",");
  const signature = [state.locale, state.theme, state.currentSessionID, state.sessionsPhase, state.sessionsError, state.attachBusyRunID, activeKey, ...state.sessions.map((item) => `${item.id}:${item.title}:${item.created_at}`)].join("|");
  if (signature === lastSignature) return;
  lastSignature = signature;

  menu.close();
  shell.newSession.textContent = translate(state.locale, "newSession");
  shell.newSession.disabled = state.sessionsPhase === "processing";
  shell.localeButton.textContent = `${translate(state.locale, "locale")}: ${state.locale === "zh-CN" ? translate(state.locale, "languageChinese") : translate(state.locale, "languageEnglish")}`;
  const themeLabel = state.theme === "system" ? translate(state.locale, "themeSystem") : state.theme === "light" ? translate(state.locale, "themeLight") : translate(state.locale, "themeDark");
  shell.themeButton.textContent = `${translate(state.locale, "theme")}: ${themeLabel}`;
  renderActivity(shell, controller, locale);

  shell.sessionList.replaceChildren();
  if (state.sessionsPhase === "loading") {
    for (let index = 0; index < 4; index += 1) {
      const skeleton = node("li", "session-skeleton");
      skeleton.innerHTML = "<span></span><span></span>";
      shell.sessionList.appendChild(skeleton);
    }
    return;
  }
  if (state.sessionsPhase === "error") {
    const item = node("li", "list-state list-state-error");
    const message = node("p");
    message.textContent = `${translate(state.locale, "sessionLoadError")}: ${state.sessionsError}`;
    const retry = node("button", "button button-secondary button-small");
    retry.type = "button";
    retry.textContent = translate(state.locale, "retry");
    retry.addEventListener("click", () => void controller.refreshSessions());
    item.append(message, retry);
    shell.sessionList.appendChild(item);
    return;
  }
  if (state.sessions.length === 0) {
    const item = node("li", "list-state list-state-empty");
    const message = node("p");
    message.textContent = translate(state.locale, "noSessions");
    const hint = node("span");
    hint.textContent = translate(state.locale, "sessionEmptyHint");
    item.append(message, hint);
    shell.sessionList.appendChild(item);
    return;
  }
  for (const session of state.sessions) {
    const item = node("li", `session-item${session.id === state.currentSessionID ? " is-selected" : ""}`);
    const main = node("button", "session-row-main");
    main.type = "button";
    main.addEventListener("click", () => {
      menu.close();
      void controller.selectSession(session.id);
    });
    const title = node("strong");
    title.textContent = session.title || translate(state.locale, "sessionUntitled");
    const meta = node("span", "session-row-meta");
    meta.textContent = formatDate(session.created_at, locale, { dateStyle: "medium", timeStyle: "short" });
    main.append(title, meta);
    const active = state.activeRuns.some((run) => run.session_id === session.id);
    if (active) {
      const status = node("span", "session-status-dot");
      status.title = translate(state.locale, "runActive");
      main.appendChild(status);
    }
    const more = node("button", "icon-button session-more");
    more.type = "button";
    more.textContent = "⋯";
    more.title = translate(state.locale, "more");
    more.addEventListener("click", (event) => {
      event.stopPropagation();
      menu.open(more, [
        { label: translate(state.locale, "rename"), onSelect: () => void controller.renameSession(session.id) },
        ...(active ? [] : [{ label: translate(state.locale, "delete"), danger: true, onSelect: () => void controller.deleteSession(session.id) }]),
      ]);
    });
    item.append(main, more);
    shell.sessionList.appendChild(item);
  }
}

function renderActivity(shell: ShellElements, controller: AppController, locale: string): void {
  const runs = [...state.activeRuns].sort((a, b) => b.created_at - a.created_at);
  shell.activityHeading.hidden = runs.length === 0;
  shell.activityList.hidden = runs.length === 0;
  shell.activityList.replaceChildren();
  for (const run of runs) {
    const item = node("li", "activity-item");
    const open = node("button", "activity-row");
    open.type = "button";
    open.disabled = state.attachBusyRunID === run.id;
    open.title = translate(state.locale, "attachRun");
    const session = state.sessions.find((candidate) => candidate.id === run.session_id);
    const title = node("strong");
    title.textContent = session?.title || translate(state.locale, "sessionUntitled");
    const meta = node("span", "activity-row-meta");
    const status = run.status === "active" ? translate(state.locale, "runActive") : run.status === "queued" ? translate(state.locale, "runQueued") : translate(state.locale, "runAccepted");
    meta.textContent = `${status} · ${formatDate(run.created_at, locale, { dateStyle: "short", timeStyle: "short" })}`;
    const dot = node("span", `activity-status-dot activity-status-${run.status}`);
    const copy = node("span", "activity-row-copy");
    copy.append(title, meta);
    open.append(dot, copy);
    open.addEventListener("click", () => void controller.attachBackgroundRun(run.id));
    item.appendChild(open);
    shell.activityList.appendChild(item);
  }
}
