import type { EvalRun, Generation } from "../../api";
import type { AppController } from "../../app/controller";
import { translate } from "../../app/i18n";
import { state } from "../../app/store";
import type { ShellElements } from "../../app/shell";
import { node } from "../../ui/dom";

function evalsFor(id: string): EvalRun[] {
  return state.studioEvals.filter((item) => item.candidate_id === id);
}

function canPromote(gen: Generation): boolean {
  if (gen.phase === "promoted" || gen.phase === "rejected") return false;
  return evalsFor(gen.id).length > 0;
}

function canReject(gen: Generation): boolean {
  return gen.phase !== "promoted" && gen.phase !== "rejected";
}

function recipeText(gen: Generation): string {
  const recipe = gen.recipe ?? {};
  const parts = [recipe.loop, recipe.world, ...(recipe.plugins ?? []), ...(recipe.tools ?? [])].filter(Boolean);
  return parts.join(" · ");
}

export function renderStudio(shell: ShellElements, controller: AppController): void {
  shell.studioButton.textContent = translate(state.locale, "studio");
  shell.studio.hidden = !state.studioOpen;
  if (!state.studioOpen) return;

  const focused = document.activeElement instanceof HTMLElement && shell.studio.contains(document.activeElement)
    ? document.activeElement.id
    : "";
  shell.studioBody.replaceChildren();

  const header = node("header", "review-center-header");
  const heading = node("div");
  const eyebrow = node("p", "eyebrow");
  eyebrow.textContent = translate(state.locale, "studio");
  const title = node("h2");
  title.id = "studio-title";
  title.tabIndex = -1;
  title.textContent = translate(state.locale, "studioTitle");
  heading.append(eyebrow, title);
  const actions = node("div", "review-center-actions");
  const refresh = node("button", "button button-secondary") as HTMLButtonElement;
  refresh.type = "button";
  refresh.textContent = translate(state.locale, "refreshStudio");
  refresh.addEventListener("click", () => void controller.refreshStudio());
  const back = node("button", "button button-secondary") as HTMLButtonElement;
  back.type = "button";
  back.textContent = translate(state.locale, "backToChat");
  back.addEventListener("click", () => controller.closeStudio());
  actions.append(refresh, back);
  header.append(heading, actions);
  shell.studioBody.appendChild(header);

  const accepted = state.studioPromotions[0];
  if (accepted) {
    const banner = node("p", "studio-banner");
    banner.id = "studio-next-launch";
    banner.textContent = `${translate(state.locale, "studioAppliesNextLaunch")} → ${accepted.to_id}`;
    shell.studioBody.appendChild(banner);
  }

  const inspect = state.studioInspect;
  if (inspect) {
    const identity = node("section", "studio-identity");
    identity.id = "studio-identity";
    const heading2 = node("h3");
    heading2.textContent = translate(state.locale, "studioIdentity");
    identity.appendChild(heading2);
    const dl = node("dl", "studio-facts");
    const add = (label: string, value: string, id?: string) => {
      const dt = node("dt");
      dt.textContent = label;
      const dd = node("dd");
      if (id) dd.id = id;
      dd.textContent = value;
      dl.append(dt, dd);
    };
    add(translate(state.locale, "studioBinary"), inspect.binary_id, "studio-binary-id");
    add(translate(state.locale, "studioGeneration"), inspect.generation_id, "studio-generation-id");
    add(translate(state.locale, "studioPolicy"), `${inspect.policy_profile} ${inspect.policy_hash}`.trim());
    add(translate(state.locale, "studioTools"), inspect.tools.map((tool) => tool.name).join(", ") || "—", "studio-tool-list");
    identity.appendChild(dl);
    shell.studioBody.appendChild(identity);
  }

  if (state.studioPhase === "loading" || state.studioPhase === "refreshing") {
    const loading = node("p", "empty-copy");
    loading.textContent = translate(state.locale, state.studioPhase === "refreshing" ? "refreshing" : "loading");
    shell.studioBody.appendChild(loading);
  }
  if (state.studioError) {
    const error = node("p", "inline-error");
    error.id = "studio-error";
    error.textContent = state.studioError;
    shell.studioBody.appendChild(error);
  }

  if (state.studioGenerations.length === 0) {
    const empty = node("div", "empty-state");
    const emptyTitle = node("h3");
    emptyTitle.textContent = translate(state.locale, "studioNoGenerations");
    const hint = node("p", "empty-copy");
    hint.textContent = translate(state.locale, "studioNoGenerationsHint");
    empty.append(emptyTitle, hint);
    shell.studioBody.appendChild(empty);
    return;
  }

  const grid = node("div", "review-center-grid");
  const list = node("ul", "review-list");
  list.setAttribute("aria-label", translate(state.locale, "studioGenerations"));
  for (const gen of state.studioGenerations) {
    const item = node("li");
    const button = node("button", `review-list-item${gen.id === state.selectedGenerationID ? " is-selected" : ""}`) as HTMLButtonElement;
    button.type = "button";
    button.dataset.generationId = gen.id;
    button.textContent = `${gen.id} · ${gen.phase}`;
    button.addEventListener("click", () => controller.selectGeneration(gen.id));
    item.appendChild(button);
    list.appendChild(item);
  }
  const detail = node("article", "review-card");
  const selected = state.studioGenerations.find((item) => item.id === state.selectedGenerationID);
  if (selected) {
    const name = node("h3");
    name.id = "studio-selected-id";
    name.textContent = selected.id;
    const phase = node("p");
    phase.id = "studio-selected-phase";
    phase.textContent = `${translate(state.locale, "studioPhase")}: ${selected.phase}`;
    const recipe = node("p");
    recipe.textContent = `${translate(state.locale, "studioRecipe")}: ${recipeText(selected) || "—"}`;
    const evalLine = node("p");
    const latest = evalsFor(selected.id)[0];
    evalLine.textContent = latest
      ? `${translate(state.locale, "studioEval")}: ${latest.verdict} (${latest.suite})`
      : `${translate(state.locale, "studioEval")}: —`;
    detail.append(name, phase, recipe, evalLine);
    const buttons = node("div", "task-actions review-actions");
    const promote = node("button", "button button-primary") as HTMLButtonElement;
    promote.type = "button";
    promote.id = "studio-promote";
    promote.textContent = state.studioBusy ? translate(state.locale, "studioPromoting") : translate(state.locale, "studioPromote");
    promote.disabled = state.studioBusy || !canPromote(selected) || !controller.resolvePromoteFromID(selected.id);
    promote.addEventListener("click", () => void controller.promoteSelected());
    const reject = node("button", "button button-secondary") as HTMLButtonElement;
    reject.type = "button";
    reject.id = "studio-reject";
    reject.textContent = state.studioBusy ? translate(state.locale, "studioRejecting") : translate(state.locale, "studioReject");
    reject.disabled = state.studioBusy || !canReject(selected);
    reject.addEventListener("click", () => void controller.rejectSelected());
    buttons.append(promote, reject);
    detail.appendChild(buttons);
  }
  grid.append(list, detail);
  shell.studioBody.appendChild(grid);
  if (focused) document.getElementById(focused)?.focus();
}
