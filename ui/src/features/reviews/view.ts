import type { ReviewItem } from "../../api";
import type { AppController } from "../../app/controller";
import { translate } from "../../app/i18n";
import { state } from "../../app/store";
import type { ShellElements } from "../../app/shell";
import { formatDate, node } from "../../ui/dom";

const drafts = new Map<string, { reason: string; answer: string }>();

function draftFor(item: ReviewItem): { reason: string; answer: string } {
  const existing = drafts.get(item.id);
  if (existing) return existing;
  const draft = { reason: "", answer: "" };
  drafts.set(item.id, draft);
  return draft;
}

function statusLabel(status: ReviewItem["status"]): string {
  const zh = state.locale === "zh-CN";
  if (zh) return { pending: "待处理", approved: "已批准", denied: "已拒绝", answered: "已回答", cancelled: "已取消", expired: "已过期", stale: "已失效" }[status];
  return { pending: "Pending", approved: "Approved", denied: "Denied", answered: "Answered", cancelled: "Cancelled", expired: "Expired", stale: "Stale" }[status];
}

function reviewTitle(item: ReviewItem): string {
  if (item.kind === "question") return translate(state.locale, "questionRequired");
  return item.tool_name || translate(state.locale, "unknownTool");
}

function reviewSummary(item: ReviewItem): string {
  if (item.kind === "question") return item.prompt || "";
  return item.preview || item.action || translate(state.locale, "approvalDescription");
}

function appendField(root: HTMLElement, label: string, value: string | undefined): void {
  if (!value) return;
  const wrapper = node("div", "review-field");
  const caption = node("span", "field-label");
  caption.textContent = label;
  const text = node("strong", "review-field-value");
  text.textContent = value;
  wrapper.append(caption, text);
  root.appendChild(wrapper);
}

function appendActionButtons(root: HTMLElement, item: ReviewItem, controller: AppController, draft: { reason: string; answer: string }): void {
  if (item.status !== "pending") return;
  const reason = node("textarea", "review-reason text-area") as HTMLTextAreaElement;
  reason.rows = 2;
  reason.id = `review-reason-${item.id}`;
  reason.setAttribute("aria-label", translate(state.locale, "reviewReasonLabel"));
  reason.placeholder = translate(state.locale, "reviewReasonPlaceholder");
  reason.value = draft.reason;
  reason.addEventListener("input", () => { draft.reason = reason.value; });
  root.appendChild(reason);

  const actions = node("div", "task-actions review-actions");
  if (item.kind === "approval") {
    const deny = node("button", "button button-secondary") as HTMLButtonElement;
    deny.type = "button";
    deny.textContent = translate(state.locale, "deny");
    deny.setAttribute("aria-label", `${translate(state.locale, "deny")} ${reviewTitle(item)}`);
    deny.disabled = state.reviewBusyID === item.id;
    deny.addEventListener("click", () => void controller.respondReview(item, "deny", draft.reason));
    const approve = node("button", "button button-primary") as HTMLButtonElement;
    approve.type = "button";
    approve.textContent = state.reviewBusyID === item.id ? translate(state.locale, "approving") : translate(state.locale, "approve");
    approve.setAttribute("aria-label", `${translate(state.locale, "approve")} ${reviewTitle(item)}`);
    approve.disabled = state.reviewBusyID === item.id;
    approve.addEventListener("click", () => void controller.respondReview(item, "approve", draft.reason));
    actions.append(deny, approve);
  } else {
    const cancel = node("button", "button button-secondary") as HTMLButtonElement;
    cancel.type = "button";
    cancel.textContent = translate(state.locale, "cancel");
    cancel.setAttribute("aria-label", `${translate(state.locale, "cancel")} ${reviewTitle(item)}`);
    cancel.disabled = state.reviewBusyID === item.id;
    cancel.addEventListener("click", () => void controller.respondReview(item, "cancel", draft.reason));
    const answer = node("button", "button button-primary") as HTMLButtonElement;
    answer.type = "button";
    answer.textContent = state.reviewBusyID === item.id ? translate(state.locale, "answering") : translate(state.locale, "answer");
    answer.setAttribute("aria-label", `${translate(state.locale, "answer")} ${reviewTitle(item)}`);
    answer.disabled = state.reviewBusyID === item.id || !draft.answer.trim();
    answer.addEventListener("click", () => void controller.respondReview(item, "answer", "", draft.answer));
    actions.append(cancel, answer);
  }
  root.appendChild(actions);
}

// One renderer is used by Review Center and the run inspector so the same
// review item never gets two subtly different decision experiences.
export function renderReviewCard(root: HTMLElement, item: ReviewItem, controller: AppController, context: "center" | "inline"): void {
  const card = node("article", `task-card review-card review-card-${item.kind}`);
  const titleID = `review-title-${item.id}`;
  card.setAttribute("aria-labelledby", titleID);
  const header = node("div", "review-card-header");
  const heading = node("div");
  const eyebrow = node("p", "eyebrow");
  eyebrow.textContent = item.kind === "approval" ? translate(state.locale, "approvalRequired") : translate(state.locale, "questionRequired");
    const title = node("h3");
  title.id = titleID;
  title.textContent = reviewTitle(item);
  heading.append(eyebrow, title);
  const badge = node("span", `review-status review-status-${item.status}`);
  badge.textContent = statusLabel(item.status);
  header.append(heading, badge);
  card.appendChild(header);

  const summary = node("p", "review-summary");
  summary.textContent = reviewSummary(item);
  card.appendChild(summary);

  const facts = node("div", "review-facts");
  appendField(facts, state.locale === "zh-CN" ? "影响" : "Effect", item.effect);
  appendField(facts, state.locale === "zh-CN" ? "范围" : "Scope", item.scope);
  appendField(facts, state.locale === "zh-CN" ? "可逆性" : "Reversibility", item.reversibility);
  appendField(facts, state.locale === "zh-CN" ? "信任来源" : "Trust", item.trust);
  if (item.target) appendField(facts, state.locale === "zh-CN" ? "目标" : "Target", item.target);
  if (facts.childElementCount > 0) card.appendChild(facts);

  if (item.kind === "approval" && item.preview) {
    const preview = node("pre", "review-preview");
    preview.textContent = item.preview;
    card.appendChild(preview);
  }
  if (item.kind === "approval" && item.arguments && Object.keys(item.arguments).length > 0) {
    const details = node("details", "review-arguments");
    const summaryDetails = node("summary");
    summaryDetails.textContent = translate(state.locale, "arguments");
    const args = node("pre");
    args.textContent = JSON.stringify(item.arguments, null, 2);
    details.append(summaryDetails, args);
    card.appendChild(details);
  }
  if (item.risk_findings && item.risk_findings.length > 0) {
    const risks = node("ul", "review-risks");
    for (const finding of item.risk_findings) {
      const risk = node("li");
      risk.textContent = finding;
      risks.appendChild(risk);
    }
    card.appendChild(risks);
  }

  const draft = draftFor(item);
  if (item.kind === "question" && item.status === "pending") {
    const answer = node("textarea", "text-area review-answer") as HTMLTextAreaElement;
    answer.rows = context === "inline" ? 4 : 5;
    answer.id = `review-answer-${item.id}`;
    answer.setAttribute("aria-label", translate(state.locale, "answerLabel"));
    answer.placeholder = translate(state.locale, "answerPlaceholder");
    answer.value = draft.answer;
    answer.addEventListener("input", () => {
      draft.answer = answer.value;
      const submit = answer.closest(".review-card")?.querySelector<HTMLButtonElement>(".review-actions .button-primary");
      if (submit) submit.disabled = !draft.answer.trim();
    });
    card.appendChild(answer);
  }
  if (item.status === "pending") appendActionButtons(card, item, controller, draft);
  if (item.expires_at > 0) {
    const expiry = node("p", "muted review-expiry");
    expiry.textContent = translate(state.locale, "expiresAt", { value: formatDate(item.expires_at, state.locale === "zh-CN" ? "zh-CN" : "en-US") });
    card.appendChild(expiry);
  }
  if (state.reviewsError) {
    const error = node("p", "inline-error");
    error.setAttribute("role", "alert");
    error.textContent = state.reviewsError;
    card.appendChild(error);
  }
  root.appendChild(card);
}

export function renderReviewCenter(shell: ShellElements, controller: AppController): void {
  const overlayOpen = state.reviewCenterOpen || state.studioOpen;
  shell.reviewCenter.closest<HTMLElement>(".app-shell")?.classList.toggle("review-center-open", overlayOpen);
  shell.reviewCenterButton.textContent = translate(state.locale, "reviewCenter");
  shell.reviewCenter.hidden = !state.reviewCenterOpen;
  shell.chat.hidden = overlayOpen;
  shell.composer.hidden = overlayOpen;
  if (!state.reviewCenterOpen) return;
  const focusedID = document.activeElement instanceof HTMLElement && shell.reviewCenter.contains(document.activeElement)
    ? document.activeElement.id
    : "";
  const restoreFocus = (): void => {
    if (!focusedID) return;
    requestAnimationFrame(() => document.getElementById(focusedID)?.focus());
  };
  shell.reviewCenterBody.replaceChildren();

  const header = node("header", "review-center-header");
  const heading = node("div");
  const eyebrow = node("p", "eyebrow");
  eyebrow.textContent = translate(state.locale, "reviewCenter");
  const title = node("h2");
  title.id = "review-center-title";
  title.tabIndex = -1;
  title.textContent = translate(state.locale, "reviewCenterTitle");
  const count = node("span", "review-count");
  count.textContent = String(state.reviews.length);
  heading.append(eyebrow, title, count);
  const actions = node("div", "review-center-actions");
  const refresh = node("button", "button button-secondary button-small") as HTMLButtonElement;
  refresh.type = "button";
  refresh.textContent = translate(state.locale, "refreshReviews");
  refresh.disabled = state.reviewsPhase === "loading" || state.reviewsPhase === "refreshing";
  refresh.addEventListener("click", () => void controller.refreshReviews());
  const close = node("button", "button button-secondary button-small") as HTMLButtonElement;
  close.type = "button";
  close.textContent = translate(state.locale, "backToChat");
  close.addEventListener("click", () => controller.closeReviewCenter());
  actions.append(refresh, close);
  header.append(heading, actions);
  shell.reviewCenterBody.appendChild(header);

  if (state.reviewsPhase === "loading") {
    const loading = node("p", "list-state");
    loading.textContent = translate(state.locale, "loading");
    shell.reviewCenterBody.appendChild(loading);
    restoreFocus();
    return;
  }
  if (state.reviewsError && state.reviews.length === 0) {
    const error = node("p", "list-state inline-error");
    error.setAttribute("role", "alert");
    error.textContent = state.reviewsError;
    shell.reviewCenterBody.appendChild(error);
    restoreFocus();
    return;
  }
  if (state.reviews.length === 0) {
    const empty = node("div", "review-empty");
    const titleEmpty = node("h3");
    titleEmpty.textContent = translate(state.locale, "noReviews");
    const hint = node("p", "muted");
    hint.textContent = translate(state.locale, "noReviewsHint");
    empty.append(titleEmpty, hint);
    shell.reviewCenterBody.appendChild(empty);
    restoreFocus();
    return;
  }

  const grid = node("div", "review-center-grid");
  const list = node("div", "review-list");
  list.setAttribute("role", "listbox");
  list.setAttribute("aria-label", translate(state.locale, "reviewCenter"));
  list.setAttribute("aria-describedby", "review-center-title");
  for (const item of state.reviews) {
    const row = node("button", `review-list-item${state.selectedReviewID === item.id ? " is-selected" : ""}`) as HTMLButtonElement;
    row.type = "button";
    row.setAttribute("role", "option");
    row.setAttribute("aria-selected", String(state.selectedReviewID === item.id));
    row.id = `review-option-${item.id}`;
    row.setAttribute("aria-label", `${reviewTitle(item)} — ${statusLabel(item.status)}`);
    const rowTop = node("span", "review-list-top");
    const rowTitle = node("strong");
    rowTitle.textContent = reviewTitle(item);
    const rowStatus = node("span", `review-status review-status-${item.status}`);
    rowStatus.textContent = statusLabel(item.status);
    rowTop.append(rowTitle, rowStatus);
    const rowMeta = node("span", "review-list-meta");
    rowMeta.textContent = `${item.session_title || item.session_id} · ${item.kind === "approval" ? (item.action || translate(state.locale, "approvalRequired")) : translate(state.locale, "questionRequired")}`;
    const rowPreview = node("span", "review-list-preview");
    rowPreview.textContent = reviewSummary(item);
    row.append(rowTop, rowMeta, rowPreview);
    row.addEventListener("click", () => void controller.selectReview(item.id));
    list.appendChild(row);
  }
  const detail = node("div", "review-detail");
  detail.setAttribute("aria-live", "polite");
  detail.setAttribute("aria-atomic", "true");
  const selected = state.reviews.find((item) => item.id === state.selectedReviewID) ?? state.reviews[0];
  if (selected) renderReviewCard(detail, selected, controller, "center");
  grid.append(list, detail);
  shell.reviewCenterBody.appendChild(grid);
  restoreFocus();
}
