import type { EventEnvelope } from "../../sse";
import type { ReviewItem } from "../../api";
import type { AppController } from "../../app/controller";
import { translate } from "../../app/i18n";
import { isRunActive, state } from "../../app/store";
import type { ShellElements } from "../../app/shell";
import { formatDate, formatTime, node } from "../../ui/dom";
import { renderReviewCard } from "../reviews/view";

let lastSignature = "";

function statusLabel(status: string): string {
  const key = status === "accepted" ? "runAccepted" : status === "queued" ? "runQueued" : status === "active" ? "runActive" : status === "completed" ? "runCompleted" : status === "failed" ? "runFailed" : "runCancelled";
  return translate(state.locale, key);
}

function eventLabel(event: EventEnvelope): string {
  const payload = event.payload;
  const chinese = state.locale === "zh-CN";
  switch (event.type) {
    case "run.started": return chinese ? "运行已开始" : "Run started";
    case "model.delta": return chinese ? "模型输出" : "Model output";
    case "model.reasoning_delta": return chinese ? "模型推理更新" : "Model reasoning updated";
    case "model.completed": return chinese ? "模型输出完成" : "Model output completed";
    case "model.request": return chinese ? "已记录模型请求" : "Model request recorded";
    case "model.usage": return chinese ? `模型用量：${String(payload.total_tokens ?? "")} tokens` : `Model usage: ${String(payload.total_tokens ?? "")} tokens`;
    case "provider.retry": return chinese ? `提供方重试（第 ${String(payload.attempt ?? "")} 次）` : `Provider retry (attempt ${String(payload.attempt ?? "")})`;
    case "provider.stall": return chinese ? "提供方暂时没有响应" : "Provider is stalled";
    case "tool.requested": return chinese ? `请求工具：${String(payload.tool_name ?? "")}` : `Tool requested: ${String(payload.tool_name ?? "")}`;
    case "tool.started": return chinese ? `工具开始：${String(payload.tool_name ?? "")}` : `Tool started: ${String(payload.tool_name ?? "")}`;
    case "tool.finished": return chinese ? `工具完成：${String(payload.tool_name ?? "")}` : `Tool finished: ${String(payload.tool_name ?? "")}`;
    case "tool.approval_required": return chinese ? `等待工具审批：${String(payload.tool_name ?? "")}` : `Approval required: ${String(payload.tool_name ?? "")}`;
    case "tool.approval_decided": return chinese ? `审批已决定：${String(payload.decision ?? "")}` : `Approval decided: ${String(payload.decision ?? "")}`;
    case "tool.approval_expired": return chinese ? "工具审批已过期" : "Tool approval expired";
    case "tool.approval_cancelled": return chinese ? "工具审批随运行取消" : "Tool approval cancelled with run";
    case "tool.proposal_stale": return chinese ? "提案已失效，操作未执行" : "Proposal became stale; action was not applied";
    case "policy.evaluated": return chinese ? `策略已评估：${String(payload.tool_name ?? "")}` : `Policy evaluated: ${String(payload.tool_name ?? "")}`;
    case "hook.started": return chinese ? `Hook 开始：${String(payload.hook_name ?? "")}` : `Hook started: ${String(payload.hook_name ?? "")}`;
    case "hook.completed": return chinese ? `Hook 完成：${String(payload.hook_name ?? "")}` : `Hook completed: ${String(payload.hook_name ?? "")}`;
    case "hook.blocked": return chinese ? `Hook 阻止了操作：${String(payload.reason ?? "")}` : `Hook blocked the action: ${String(payload.reason ?? "")}`;
    case "user.question_required": return chinese ? "等待用户回答" : "Waiting for user answer";
    case "user.question_answered": return chinese ? "用户已回答" : "User answered";
    case "user.question_cancelled": return chinese ? "用户问题已取消" : "User question cancelled";
    case "user.question_expired": return chinese ? "用户问题已过期" : "User question expired";
    case "child.requested": return chinese ? "已请求子任务" : "Child run requested";
    case "child.started": return chinese ? "子任务已开始" : "Child run started";
    case "child.suspended": return chinese ? "子任务已暂停" : "Child run suspended";
    case "child.resumed": return chinese ? "子任务已恢复" : "Child run resumed";
    case "child.completed": return chinese ? "子任务已完成" : "Child run completed";
    case "child.failed": return chinese ? `子任务失败：${String(payload.message ?? "")}` : `Child run failed: ${String(payload.message ?? "")}`;
    case "child.cancelled": return chinese ? "子任务已取消" : "Child run cancelled";
    case "run.completed": return chinese ? "运行已完成" : "Run completed";
    case "run.failed": return chinese ? `运行失败：${String(payload.message ?? "")}` : `Run failed: ${String(payload.message ?? "")}`;
    case "run.cancelled": return chinese ? "运行已取消" : "Run cancelled";
    default: return event.type;
  }
}

export function renderRunInspector(shell: ShellElements, controller: AppController): void {
  const signature = [
    state.locale,
    state.theme,
    state.inspectorOpen,
    state.inspectorTab,
    state.connection,
    state.runError,
    state.run?.id,
    state.run?.status,
    state.run?.created_at,
    state.events.length,
    state.pendingApproval?.id,
    state.approvalBusy,
    state.approvalError,
    state.pendingQuestion?.id,
    state.questionBusy,
    state.questionError,
    state.questionDraft,
    state.childrenPhase,
    state.childrenError,
    state.childCancelBusyID,
    ...state.children.map((child) => `${child.id}:${child.status}:${child.result ?? ""}:${child.error ?? ""}`),
  ].join("|");
  if (signature === lastSignature) return;
  lastSignature = signature;

  shell.inspectorTitle.textContent = translate(state.locale, "inspector");
  shell.inspectorConnection.textContent = connectionLabel();
  shell.inspectorConnection.dataset.state = state.connection;
  shell.inspectorTabs.replaceChildren();
  for (const tab of ["overview", "events"] as const) {
    const button = node("button", `inspector-tab${state.inspectorTab === tab ? " is-active" : ""}`);
    button.type = "button";
    button.textContent = translate(state.locale, tab);
    button.addEventListener("click", () => controller.setInspectorTab(tab));
    shell.inspectorTabs.appendChild(button);
  }
  shell.inspectorBody.replaceChildren();
  if (!state.run) {
    const empty = node("div", "inspector-empty");
    empty.textContent = translate(state.locale, "noRun");
    shell.inspectorBody.appendChild(empty);
    return;
  }
  if (state.inspectorTab === "events") renderEvents(shell, state.events);
  else renderOverview(shell, controller);
}

function connectionLabel(): string {
  if (state.connection === "connected") return translate(state.locale, "connected");
  if (state.connection === "reconnecting") return translate(state.locale, "reconnecting");
  return state.connection === "connecting" ? translate(state.locale, "loading") : "";
}

function renderOverview(shell: ShellElements, controller: AppController): void {
  if (!state.run) return;
  const review = currentReview();
  if (review) renderReviewCard(shell.inspectorBody, review, controller, "inline");

  const statusCard = node("section", "inspector-card run-summary-card");
  const eyebrow = node("p", "eyebrow");
  eyebrow.textContent = translate(state.locale, "runStatus");
  const status = node("div", `status-summary status-${state.run.status}`);
  const statusDot = node("span", "status-dot");
  const statusText = node("strong");
  statusText.textContent = statusLabel(state.run.status);
  status.append(statusDot, statusText);
  const date = node("p", "muted");
  date.textContent = translate(state.locale, "startedAt", { value: formatDate(state.run.created_at, state.locale === "zh-CN" ? "zh-CN" : "en-US") });
  statusCard.append(eyebrow, status, date);
  shell.inspectorBody.appendChild(statusCard);

  const start = state.events.find((event) => event.type === "run.started");
  if (start) {
    const details = node("dl", "run-details");
    addDetail(details, translate(state.locale, "provider"), String(start.payload.provider ?? "—"));
    addDetail(details, translate(state.locale, "model"), String(start.payload.model ?? "—"));
    addDetail(details, translate(state.locale, "mode"), String(start.payload.mode ?? "—"));
    addDetail(details, translate(state.locale, "events"), translate(state.locale, "eventCount", { count: state.events.length }));
    shell.inspectorBody.appendChild(details);
  }
  if (state.run.status === "failed") {
    const failure = node("section", "inspector-card inspector-card-error");
    const title = node("strong");
    title.textContent = translate(state.locale, "runFailed");
    const event = [...state.events].reverse().find((item) => item.type === "run.failed");
    const message = node("p");
    message.textContent = String(event?.payload.message ?? translate(state.locale, "unknownError"));
    failure.append(title, message);
    shell.inspectorBody.appendChild(failure);
  }
  renderChildRuns(shell.inspectorBody, controller);
  if (state.runError) {
    const error = node("p", "inline-error");
    error.textContent = state.runError;
    shell.inspectorBody.appendChild(error);
  }
}

function renderChildRuns(root: HTMLElement, controller: AppController): void {
  const card = node("section", "inspector-card child-runs-card");
  const header = node("div", "subsection-heading");
  const title = node("div");
  const eyebrow = node("p", "eyebrow");
  eyebrow.textContent = translate(state.locale, "childRuns");
  title.appendChild(eyebrow);
  const refresh = node("button", "button button-secondary button-small");
  refresh.type = "button";
  refresh.textContent = state.childrenPhase === "refreshing" ? translate(state.locale, "refreshing") : translate(state.locale, "refreshChildren");
  refresh.disabled = state.childrenPhase === "loading" || state.childrenPhase === "refreshing";
  refresh.addEventListener("click", () => void controller.refreshChildren());
  header.append(title, refresh);
  card.appendChild(header);

  if (state.childrenPhase === "loading") {
    const loading = node("p", "muted");
    loading.textContent = translate(state.locale, "childLoading");
    card.appendChild(loading);
  } else if (state.childrenPhase === "error") {
    const error = node("p", "inline-error");
    error.textContent = `${translate(state.locale, "childLoadError")}: ${state.childrenError}`;
    card.appendChild(error);
    const retry = node("button", "button button-secondary button-small");
    retry.type = "button";
    retry.textContent = translate(state.locale, "retry");
    retry.addEventListener("click", () => void controller.refreshChildren());
    card.appendChild(retry);
  } else if (state.children.length === 0) {
    const empty = node("p", "muted");
    empty.textContent = translate(state.locale, "noChildRuns");
    card.appendChild(empty);
  } else {
    const list = node("ul", "child-run-list");
    for (const child of state.children) {
      const item = node("li", "child-run-item");
      item.style.setProperty("--child-depth", String(Math.min(child.depth, 6)));
      const info = node("div", "child-run-info");
      const name = node("strong");
      name.textContent = child.id;
      const meta = node("span", "child-run-meta");
      meta.textContent = `${statusLabel(child.status)} · ${translate(state.locale, "depth")}: ${child.depth}`;
      info.append(name, meta);
      if (child.result || child.error) {
        const outcome = node("span", child.error ? "child-run-outcome child-run-outcome-error" : "child-run-outcome");
        outcome.textContent = `${child.error ? translate(state.locale, "childError") : translate(state.locale, "childResult")}: ${child.error || child.result}`;
        info.appendChild(outcome);
      }
      const actions = node("div", "child-run-actions");
      const open = node("button", "button button-secondary button-small");
      open.type = "button";
      open.textContent = translate(state.locale, "openRun");
      open.addEventListener("click", () => void controller.openRun(child.id, child.session_id));
      actions.appendChild(open);
      if (isRunActive(child)) {
        const cancel = node("button", "button button-danger button-small");
        cancel.type = "button";
        cancel.textContent = state.childCancelBusyID === child.id ? translate(state.locale, "childCancelling") : translate(state.locale, "childCancel");
        cancel.disabled = state.childCancelBusyID !== null;
        cancel.addEventListener("click", () => void controller.cancelChild(child.id));
        actions.appendChild(cancel);
      }
      item.append(info, actions);
      list.appendChild(item);
    }
    card.appendChild(list);
  }
  if (state.childrenError && state.childrenPhase !== "error") {
    const error = node("p", "inline-error");
    error.textContent = state.childrenError;
    card.appendChild(error);
  }
  root.appendChild(card);
}

function addDetail(list: HTMLDListElement, term: string, value: string): void {
  const dt = node("dt");
  dt.textContent = term;
  const dd = node("dd");
  dd.textContent = value;
  list.append(dt, dd);
}

function currentReview(): ReviewItem | null {
  if (!state.run) return null;
  const durable = state.reviews.find((item) => item.run_id === state.run?.id && item.status === "pending");
  if (durable) return durable;
  if (state.pendingApproval) {
    return {
      id: state.pendingApproval.id, kind: "approval", status: "pending", session_id: state.run.session_id,
      run_id: state.run.id, tool_call_id: state.pendingApproval.toolCallId, tool_name: state.pendingApproval.toolName,
      created_at: state.run.created_at, expires_at: state.pendingApproval.expiresAt, arguments: state.pendingApproval.args,
      effect: "external side effect", scope: "current run", trust: "policy gated",
    };
  }
  if (state.pendingQuestion) {
    return {
      id: state.pendingQuestion.id, kind: "question", status: "pending", session_id: state.run.session_id,
      run_id: state.run.id, tool_call_id: state.pendingQuestion.tool_call_id, tool_name: "ask_user",
      created_at: state.run.created_at, expires_at: state.pendingQuestion.expires_at, prompt: state.pendingQuestion.prompt,
      effect: "input", scope: "current run", trust: "model requested",
    };
  }
  return null;
}

function renderEvents(shell: ShellElements, events: EventEnvelope[]): void {
  if (events.length === 0) {
    const empty = node("div", "inspector-empty");
    empty.textContent = translate(state.locale, "noEvents");
    shell.inspectorBody.appendChild(empty);
    return;
  }
  const list = node("ol", "event-timeline");
  for (const event of events) {
    const item = node("li", `event-item event-${event.type.replaceAll(".", "-")}`);
    const marker = node("span", "event-marker");
    const content = node("div", "event-content");
    const heading = node("div", "event-heading");
    const label = node("strong");
    label.textContent = eventLabel(event);
    const time = node("time");
    time.textContent = formatTime(event.created_at, state.locale === "zh-CN" ? "zh-CN" : "en-US");
    heading.append(label, time);
    const details = node("details", "event-details");
    const summary = node("summary");
    summary.textContent = `${translate(state.locale, "eventDetails")} · #${event.seq}`;
    const raw = node("pre");
    raw.textContent = JSON.stringify(event.payload, null, 2);
    details.append(summary, raw);
    content.append(heading, details);
    item.append(marker, content);
    list.appendChild(item);
  }
  shell.inspectorBody.appendChild(list);
}
