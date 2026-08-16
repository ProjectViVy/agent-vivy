import { AppController } from "./app/controller";
import { createShell } from "./app/shell";
import { notify, state, subscribe } from "./app/store";
import { DialogHost } from "./ui/dialog";
import { renderSessions } from "./features/sessions/view";
import { renderConversation } from "./features/conversation/view";
import { renderRunInspector } from "./features/runs/view";
import { renderReviewCenter } from "./features/reviews/view";
import { renderStudio } from "./features/studio/view";
import "./styles.css";

const appRoot = document.getElementById("app");
if (!appRoot) throw new Error("Vivy UI root is missing");

const shell = createShell(appRoot);
const dialogRoot = document.getElementById("dialog-root");
if (!dialogRoot) throw new Error("Vivy dialog root is missing");
const controller = new AppController(new DialogHost(dialogRoot));

function renderAll(): void {
  renderSessions(shell, controller);
  renderConversation(shell, controller);
  renderRunInspector(shell, controller);
  renderReviewCenter(shell, controller);
  renderStudio(shell, controller);
  document.documentElement.lang = state.locale;
  document.title = "Vivy";
}

shell.newSession.addEventListener("click", () => void controller.createNewSession());
shell.reviewCenterButton.addEventListener("click", () => void controller.openReviewCenter());
shell.studioButton.addEventListener("click", () => void controller.openStudio());
shell.cancelRun.addEventListener("click", () => void controller.cancelCurrentRun());
shell.inspectorToggle.addEventListener("click", () => controller.toggleInspector());
shell.inspectorClose.addEventListener("click", () => controller.closeInspector());
shell.mobileSidebarButton.addEventListener("click", () => controller.toggleSidebar());
shell.sidebarScrim.addEventListener("click", () => controller.toggleSidebar());
shell.sidebar.querySelector("#mobile-sidebar-close")?.addEventListener("click", () => controller.toggleSidebar());
shell.localeButton.addEventListener("click", () => controller.setLocale(state.locale === "zh-CN" ? "en" : "zh-CN"));
shell.themeButton.addEventListener("click", () => controller.cycleTheme());

shell.composerInput.addEventListener("input", () => {
  controller.setDraft(shell.composerInput.value);
  notify();
});
shell.modeToggle.addEventListener("change", () => controller.setMode(shell.modeToggle.checked ? "plan" : "normal"));
shell.composer.addEventListener("submit", (event) => {
  event.preventDefault();
  controller.setDraft(shell.composerInput.value);
  void controller.sendMessage();
});

// The state store intentionally has no framework dependency; this one
// subscription updates only the feature regions that own their data.
const removeSubscription = subscribe(renderAll);
window.addEventListener("beforeunload", () => {
  removeSubscription();
  controller.dispose();
}, { once: true });

renderAll();
void controller.boot();
