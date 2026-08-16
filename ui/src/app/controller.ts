import * as api from "../api";
import { ApiError, type Approval, type Message, type RunMode } from "../api";
import {
  subscribeRun,
  type EventEnvelope,
  type RunEventType,
  type RunSubscription,
} from "../sse";
import { DialogHost } from "../ui/dialog";
import { applyTheme, loadLocale, loadTheme, nextTheme, saveLocale, saveTheme, type Locale } from "./preferences";
import { notify, resetSessionView, state, isRunActive } from "./store";
import { translate } from "./i18n";

function failureMessage(error: unknown): string {
  if (error instanceof ApiError) return `${error.code}: ${error.message}`;
  if (error instanceof Error) return error.message;
  return String(error);
}

function eventFromLog(value: api.RunLogEvent): EventEnvelope | null {
  return {
    run_id: value.run_id,
    seq: value.seq,
    type: value.type as RunEventType,
    created_at: value.created_at,
    payload_version: value.payload_version,
    payload: value.payload,
  };
}

function lastRunID(messages: Message[]): string | null {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    if (messages[index].run_id) return messages[index].run_id ?? null;
  }
  return null;
}

function streamedText(events: EventEnvelope[]): string {
  return events
    .filter((event) => event.type === "model.delta")
    .map((event) => String(event.payload.delta ?? ""))
    .join("");
}

function approvalFromEvent(approval: Approval, events: EventEnvelope[]): typeof state.pendingApproval {
  const event = [...events].reverse().find(
    (candidate) => candidate.type === "tool.approval_required" && candidate.payload.approval_id === approval.id,
  );
  return {
    id: approval.id,
    runId: approval.run_id,
    toolCallId: approval.tool_call_id,
    toolName: String(event?.payload.tool_name ?? translate(state.locale, "unknownTool")),
    args: (event?.payload.args as Record<string, unknown> | undefined) ?? {},
    expiresAt: approval.expires_at,
  };
}

export class AppController {
  private subscription: RunSubscription | null = null;
  private pollTimer: number | null = null;

  constructor(private readonly dialog: DialogHost) {
    state.locale = loadLocale();
    state.theme = loadTheme();
    applyTheme(state.theme);
    window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
      if (state.theme === "system") applyTheme("system");
    });
  }

  async boot(): Promise<void> {
    await Promise.all([this.refreshSessions(), this.refreshActiveRuns(), this.refreshAttention()]);
    this.pollTimer = window.setInterval(() => {
      void this.refreshAttention();
      void this.refreshActiveRuns();
    }, 5000);
    document.addEventListener("visibilitychange", () => {
      if (document.visibilityState === "visible") {
        void this.refreshAttention();
        void this.refreshActiveRuns();
      }
    });
  }

  dispose(): void {
    if (this.pollTimer !== null) window.clearInterval(this.pollTimer);
    this.stopSubscription();
  }

  async refreshSessions(): Promise<void> {
    const hasExisting = state.sessions.length > 0;
    state.sessionsPhase = hasExisting ? "refreshing" : "loading";
    notify();
    try {
      const result = await api.listSessions();
      state.sessions = result.sessions;
      state.sessionsPhase = state.sessions.length > 0 ? "ready" : "empty";
      state.sessionsError = "";
    } catch (error) {
      state.sessionsError = failureMessage(error);
      state.sessionsPhase = state.sessions.length > 0 ? "ready" : "error";
    }
    notify();
  }

  async refreshActiveRuns(): Promise<void> {
    try {
      const result = await api.listBackgroundRuns();
      state.activeRuns = result.runs;
      notify();
    } catch {
      // The next poll retries; an active run already on screen remains usable.
    }
  }

  async attachBackgroundRun(runID: string): Promise<void> {
    if (state.attachBusyRunID) return;
    state.attachBusyRunID = runID;
    state.runError = "";
    notify();
    try {
      const run = await api.attachBackgroundRun(runID);
      if (state.currentSessionID !== run.session_id) await this.selectSession(run.session_id);
      if (state.currentSessionID === run.session_id) await this.loadRun(run.id, run.session_id);
    } catch (error) {
      state.runError = failureMessage(error);
      notify();
    } finally {
      state.attachBusyRunID = null;
      notify();
    }
  }

  async openRun(runID: string, sessionID: string): Promise<void> {
    if (state.currentSessionID !== sessionID) await this.selectSession(sessionID);
    if (state.currentSessionID === sessionID) await this.loadRun(runID, sessionID);
  }

  async createNewSession(): Promise<void> {
    if (state.sessionsPhase === "processing") return;
    state.sessionsPhase = "processing";
    notify();
    try {
      const session = await api.createSession(translate(state.locale, "sessionUntitled"));
      await this.refreshSessions();
      await this.selectSession(session.id);
    } catch (error) {
      state.sessionsError = failureMessage(error);
      state.sessionsPhase = state.sessions.length > 0 ? "ready" : "error";
      notify();
    }
  }

  async selectSession(id: string): Promise<void> {
    if (state.currentSessionID === id && state.messagesPhase === "ready") return;
    this.stopSubscription();
    resetSessionView();
    state.currentSessionID = id;
    notify();
    try {
      const [messageResult, activeResult] = await Promise.all([api.listMessages(id), api.listBackgroundRuns()]);
      if (state.currentSessionID !== id) return;
      state.messages = messageResult.messages;
      state.messagesPhase = "ready";
      state.activeRuns = activeResult.runs;
      const activeForSession = activeResult.runs
        .filter((run) => run.session_id === id)
        .sort((a, b) => b.created_at - a.created_at);
      const runID = activeForSession[0]?.id ?? lastRunID(messageResult.messages);
      if (runID) await this.loadRun(runID, id);
      await this.refreshAttention();
      notify();
    } catch (error) {
      if (state.currentSessionID !== id) return;
      state.messagesPhase = "error";
      state.messagesError = failureMessage(error);
      notify();
    }
  }

  async renameSession(id: string): Promise<void> {
    const session = state.sessions.find((item) => item.id === id);
    if (!session) return;
    await this.dialog.prompt({
      title: translate(state.locale, "renameSession"),
      description: translate(state.locale, "renameSessionDescription"),
      label: translate(state.locale, "sessionTitle"),
      initialValue: session.title,
      confirmLabel: translate(state.locale, "save"),
      cancelLabel: translate(state.locale, "cancel"),
      onConfirm: async (title) => {
        if (!title) return;
        const renamed = await api.renameSession(id, title);
        const index = state.sessions.findIndex((item) => item.id === id);
        if (index >= 0) state.sessions[index] = renamed;
        notify();
      },
    });
  }

  async deleteSession(id: string): Promise<void> {
    if (state.activeRuns.some((run) => run.session_id === id)) return;
    await this.dialog.confirm({
      title: translate(state.locale, "deleteSession"),
      description: translate(state.locale, "deleteSessionDescription"),
      confirmLabel: translate(state.locale, "confirmDelete"),
      cancelLabel: translate(state.locale, "cancel"),
      danger: true,
      onConfirm: async () => {
        await api.deleteSession(id);
        if (state.currentSessionID === id) {
          this.stopSubscription();
          resetSessionView();
          state.currentSessionID = null;
        }
        await this.refreshSessions();
        notify();
      },
    });
  }

  setDraft(value: string): void {
    state.draft = value;
  }

  setMode(mode: RunMode): void {
    state.mode = mode;
    notify();
  }

  async sendMessage(): Promise<void> {
    const sessionID = state.currentSessionID;
    const text = state.draft.trim();
    if (!sessionID || !text || state.sendPhase === "processing" || isRunActive(state.run)) return;
    const mode = state.mode;
    state.sendPhase = "processing";
    state.sendError = "";
    notify();
    try {
      const check = await api.preflight(sessionID, text, mode);
      if (check.status === "blocked") {
        state.sendError = `${translate(state.locale, "blocked")}: ${(check.blockers ?? []).join("; ")}`;
        state.sendPhase = "error";
        notify();
        return;
      }
      if ((check.warnings ?? []).length > 0) {
        const continued = await this.dialog.confirm({
          title: translate(state.locale, "preflightWarnings"),
          description: `${translate(state.locale, "preflightDescription")}\n\n${check.warnings?.join("\n") ?? ""}`,
          confirmLabel: translate(state.locale, "continue"),
          cancelLabel: translate(state.locale, "cancel"),
          onConfirm: async () => this.startAccepted(sessionID, text, mode),
        });
        if (!continued) return;
      } else {
        await this.startAccepted(sessionID, text, mode);
      }
    } catch (error) {
      state.sendError = failureMessage(error);
    } finally {
      if (state.sendPhase === "processing") state.sendPhase = "idle";
      notify();
    }
  }

  private async startAccepted(sessionID: string, text: string, mode: RunMode): Promise<void> {
    const accepted = await api.postMessage(sessionID, text, mode);
    state.run = {
      id: accepted.run_id,
      session_id: sessionID,
      status: accepted.status,
      created_at: Date.now(),
    };
    state.activeRuns = [
      {
        id: accepted.run_id,
        session_id: sessionID,
        status: accepted.status,
        created_at: state.run.created_at,
        events_url: "",
        logs_url: "",
      },
      ...state.activeRuns.filter((run) => run.id !== accepted.run_id),
    ];
    state.messages = [
      ...state.messages,
      { id: `local-${accepted.run_id}`, run_id: accepted.run_id, role: "user", content: text, created_at: Date.now() },
    ];
    state.draft = "";
    state.sendError = "";
    state.streamingText = "";
    state.events = [];
    state.children = [];
    state.childrenPhase = "idle";
    state.childrenError = "";
    state.childCancelBusyID = null;
    state.eventLogVisible = true;
    state.inspectorOpen = true;
    state.inspectorTab = "overview";
    state.connection = "connecting";
    notify();
    this.startSubscription(accepted.run_id, 0);
  }

  async cancelCurrentRun(): Promise<void> {
    if (!state.run || state.cancelBusy) return;
    state.cancelBusy = true;
    notify();
    try {
      await api.cancelRun(state.run.id);
    } catch (error) {
      state.runError = failureMessage(error);
    } finally {
      state.cancelBusy = false;
      notify();
    }
  }

  toggleInspector(): void {
    state.inspectorOpen = !state.inspectorOpen;
    notify();
  }

  async openReviewCenter(): Promise<void> {
    state.studioOpen = false;
    state.reviewCenterOpen = true;
    // Review Center is the active full-width work surface. Do not leave the
    // live run inspector as a fixed overlay that can intercept its controls
    // on narrow viewports.
    state.inspectorOpen = false;
    state.reviewsPhase = state.reviews.length > 0 ? "refreshing" : "loading";
    notify();
    await this.refreshReviews();
    if (!state.selectedReviewID && state.reviews[0]) state.selectedReviewID = state.reviews[0].id;
    notify();
    requestAnimationFrame(() => document.querySelector<HTMLElement>("#review-center-title")?.focus());
  }

  closeReviewCenter(): void {
    state.reviewCenterOpen = false;
    notify();
    requestAnimationFrame(() => document.querySelector<HTMLButtonElement>("#review-center-button")?.focus());
  }

  async openStudio(): Promise<void> {
    state.reviewCenterOpen = false;
    state.studioOpen = true;
    state.inspectorOpen = false;
    state.studioPhase = state.studioInspect ? "refreshing" : "loading";
    notify();
    await this.refreshStudio();
    requestAnimationFrame(() => document.querySelector<HTMLElement>("#studio-title")?.focus());
  }

  closeStudio(): void {
    state.studioOpen = false;
    notify();
    requestAnimationFrame(() => document.querySelector<HTMLButtonElement>("#studio-button")?.focus());
  }

  async refreshStudio(): Promise<void> {
    if (!state.studioOpen && state.studioPhase === "idle") return;
    try {
      const [inspect, generations, evals, promotions] = await Promise.all([
        api.inspectSpecies(),
        api.listGenerations(),
        api.listEvals(),
        api.listPromotions(),
      ]);
      state.studioInspect = inspect;
      state.studioGenerations = generations.generations ?? [];
      state.studioEvals = evals.evals ?? [];
      state.studioPromotions = promotions.promotions ?? [];
      if (state.selectedGenerationID && !state.studioGenerations.some((item) => item.id === state.selectedGenerationID)) {
        state.selectedGenerationID = null;
      }
      if (!state.selectedGenerationID && state.studioGenerations[0]) state.selectedGenerationID = state.studioGenerations[0].id;
      state.studioError = "";
      state.studioPhase = state.studioGenerations.length === 0 && !state.studioInspect ? "empty" : "ready";
    } catch (error) {
      state.studioError = failureMessage(error);
      state.studioPhase = "error";
    }
    notify();
  }

  selectGeneration(id: string): void {
    state.selectedGenerationID = id;
    notify();
  }

  resolvePromoteFromID(toID: string): string | null {
    const inspectID = state.studioInspect?.generation_id;
    if (inspectID && inspectID !== "builtin" && inspectID !== toID && state.studioGenerations.some((item) => item.id === inspectID)) {
      return inspectID;
    }
    const other = state.studioGenerations.find((item) => item.id !== toID);
    return other?.id ?? null;
  }

  async promoteSelected(): Promise<void> {
    const toID = state.selectedGenerationID;
    if (!toID || state.studioBusy) return;
    const fromID = this.resolvePromoteFromID(toID);
    if (!fromID) {
      state.studioError = translate(state.locale, "studioLoadError");
      notify();
      return;
    }
    state.studioBusy = true;
    notify();
    try {
      await api.promoteGeneration({ from_id: fromID, to_id: toID, actor: "human" });
      await this.refreshStudio();
    } catch (error) {
      state.studioError = failureMessage(error);
    } finally {
      state.studioBusy = false;
      notify();
    }
  }

  async rejectSelected(): Promise<void> {
    const id = state.selectedGenerationID;
    if (!id || state.studioBusy) return;
    state.studioBusy = true;
    notify();
    try {
      await api.rejectGeneration(id);
      await this.refreshStudio();
    } catch (error) {
      state.studioError = failureMessage(error);
    } finally {
      state.studioBusy = false;
      notify();
    }
  }

  async selectReview(reviewID: string): Promise<void> {
    const review = state.reviews.find((item) => item.id === reviewID);
    if (!review) return;
    state.selectedReviewID = reviewID;
    notify();
    if (state.currentSessionID !== review.session_id) await this.selectSession(review.session_id);
    if (state.currentSessionID === review.session_id && state.run?.id !== review.run_id) await this.loadRun(review.run_id, review.session_id);
    state.reviewCenterOpen = true;
    state.studioOpen = false;
    state.inspectorOpen = false;
    notify();
  }

  async respondReview(review: api.ReviewItem, action: "approve" | "deny" | "answer" | "cancel", reason = "", answer = ""): Promise<void> {
    if (state.reviewBusyID) return;
    state.reviewBusyID = review.id;
    state.reviewsError = "";
    notify();
    try {
      await api.respondReview(review.id, { action, reason: reason.trim() || undefined, answer: answer.trim() || undefined });
      await Promise.all([this.refreshReviews(), this.refreshAttention()]);
    } catch (error) {
      state.reviewsError = failureMessage(error);
    } finally {
      state.reviewBusyID = null;
      notify();
    }
  }

  closeInspector(): void {
    state.inspectorOpen = false;
    notify();
  }

  setInspectorTab(tab: "overview" | "events"): void {
    state.inspectorTab = tab;
    state.eventLogVisible = tab === "events";
    notify();
  }

  toggleSidebar(): void {
    state.sidebarOpen = !state.sidebarOpen;
    notify();
  }

  setLocale(locale: Locale): void {
    state.locale = locale;
    saveLocale(locale);
    notify();
  }

  cycleTheme(): void {
    state.theme = nextTheme(state.theme);
    saveTheme(state.theme);
    applyTheme(state.theme);
    notify();
  }

  async decideApproval(decision: "approved" | "denied"): Promise<void> {
    const approval = state.pendingApproval;
    if (!approval || state.approvalBusy) return;
    state.approvalBusy = true;
    state.approvalError = "";
    notify();
    try {
      await api.decideApproval(approval.id, decision);
      state.pendingApproval = null;
      await this.refreshAttention();
    } catch (error) {
      if (error instanceof ApiError && (error.status === 404 || error.status === 409)) state.pendingApproval = null;
      state.approvalError = failureMessage(error);
    } finally {
      state.approvalBusy = false;
      notify();
    }
  }

  async answerQuestion(answer: string): Promise<void> {
    const question = state.pendingQuestion;
    if (!question || state.questionBusy || !answer.trim()) return;
    state.questionBusy = true;
    state.questionError = "";
    notify();
    try {
      await api.answerQuestion(question.id, answer.trim());
      state.pendingQuestion = null;
      state.questionDraft = "";
      await this.refreshAttention();
    } catch (error) {
      if (error instanceof ApiError && (error.status === 404 || error.status === 409)) state.pendingQuestion = null;
      state.questionError = failureMessage(error);
    } finally {
      state.questionBusy = false;
      notify();
    }
  }

  async refreshAttention(): Promise<void> {
    try {
      const [approvalResult, questionResult, reviewResult] = await Promise.all([api.listApprovals(), api.listQuestions(), api.listReviews({ status: "pending", limit: 100 })]);
      state.approvals = approvalResult.approvals;
      state.questions = questionResult.questions;
      state.reviews = reviewResult.reviews;
      state.reviewsPhase = state.reviews.length > 0 ? "ready" : "empty";
      this.rebuildAttention();
      notify();
    } catch {
      // Polling hiccups do not replace the current task with an empty state.
    }
  }

  async refreshReviews(): Promise<void> {
    try {
      const result = await api.listReviews({ status: "pending", limit: 100 });
      state.reviews = result.reviews;
      state.reviewsPhase = state.reviews.length > 0 ? "ready" : "empty";
      if (state.selectedReviewID && !state.reviews.some((item) => item.id === state.selectedReviewID)) {
        state.selectedReviewID = state.reviews[0]?.id ?? null;
      }
      state.reviewsError = "";
    } catch (error) {
      state.reviewsError = failureMessage(error);
      state.reviewsPhase = state.reviews.length > 0 ? "ready" : "error";
    }
    notify();
  }

  private async loadRun(runID: string, sessionID: string): Promise<void> {
    const [run, log] = await Promise.all([api.getRun(runID), api.getRunLog(runID)]);
    if (state.currentSessionID !== sessionID) return;
    state.run = run;
    state.events = log.events.map(eventFromLog).filter((event): event is EventEnvelope => event !== null);
    state.streamingText = isRunActive(run) ? streamedText(state.events) : "";
    state.inspectorOpen = true;
    state.inspectorTab = "overview";
    state.eventLogVisible = false;
    state.connection = isRunActive(run) ? "connecting" : "connected";
    this.rebuildAttention();
    notify();
    void this.refreshChildren(run.id);
    if (isRunActive(run)) this.startSubscription(run.id, this.lastSeq());
  }

  async refreshChildren(runID = state.run?.id ?? ""): Promise<void> {
    if (!runID) return;
    if (state.childrenPhase === "idle" || state.children.length === 0) state.childrenPhase = "loading";
    else state.childrenPhase = "refreshing";
    state.childrenError = "";
    notify();
    try {
      const result = await api.listChildren(runID, true);
      if (state.run?.id !== runID) return;
      state.children = result.children;
      state.childrenPhase = "ready";
    } catch (error) {
      if (state.run?.id !== runID) return;
      state.childrenError = failureMessage(error);
      state.childrenPhase = state.children.length > 0 ? "ready" : "error";
    }
    notify();
  }

  async cancelChild(runID: string): Promise<void> {
    const child = state.children.find((item) => item.id === runID);
    if (!child || !isRunActive(child) || state.childCancelBusyID) return;
    state.childCancelBusyID = runID;
    state.childrenError = "";
    notify();
    try {
      await api.cancelChild(runID);
      await this.refreshChildren(state.run?.id ?? child.parent_run_id);
    } catch (error) {
      state.childrenError = failureMessage(error);
      notify();
    } finally {
      state.childCancelBusyID = null;
      notify();
    }
  }

  private lastSeq(): number {
    return state.events.reduce((max, event) => Math.max(max, event.seq), 0);
  }

  private rebuildAttention(): void {
    if (!state.run) {
      state.pendingApproval = null;
      state.pendingQuestion = null;
      state.questionDraft = "";
      return;
    }
    const approval = state.approvals.find((item) => item.run_id === state.run?.id);
    state.pendingApproval = approval ? approvalFromEvent(approval, state.events) : null;
    state.pendingQuestion = state.questions.find((item) => item.run_id === state.run?.id) ?? null;
    if (state.pendingApproval || state.pendingQuestion) state.inspectorOpen = true;
  }

  private startSubscription(runID: string, afterSeq: number): void {
    this.stopSubscription();
    state.connection = "connecting";
    notify();
    this.subscription = subscribeRun(
      runID,
      afterSeq,
      (event) => this.handleEvent(event),
      (message) => {
        state.connection = "reconnecting";
        state.runError = message;
        notify();
      },
    );
  }

  private handleEvent(event: EventEnvelope): void {
    if (!state.run || state.run.id !== event.run_id) return;
    if (state.events.some((item) => item.seq === event.seq)) return;
    state.events = [...state.events, event].sort((a, b) => a.seq - b.seq);
    state.connection = "connected";
    state.runError = "";
    switch (event.type) {
      case "run.started":
        state.run = { ...state.run, status: "active" };
        state.activeRuns = state.activeRuns.map((run) => run.id === event.run_id ? { ...run, status: "active" } : run);
        break;
      case "model.delta":
        state.streamingText += String(event.payload.delta ?? "");
        break;
      case "tool.approval_required":
        state.approvalError = "";
        this.rebuildAttentionFromEvent(event);
        break;
      case "user.question_required":
        state.questionError = "";
        state.pendingQuestion = {
          id: String(event.payload.question_id ?? ""),
          run_id: event.run_id,
          tool_call_id: String(event.payload.tool_call_id ?? ""),
          prompt: String(event.payload.prompt ?? ""),
          expires_at: Number(event.payload.expires_at ?? 0),
        };
        state.questionDraft = "";
        break;
      case "user.question_answered":
        if (state.pendingQuestion?.id === String(event.payload.question_id ?? "")) {
          state.pendingQuestion = null;
          state.questionDraft = "";
        }
        break;
      case "run.completed":
        state.run = { ...state.run, status: "completed" };
        state.activeRuns = state.activeRuns.filter((run) => run.id !== event.run_id);
        state.streamingText = "";
        void this.refreshChildren(state.run.id);
        void this.refreshMessages();
        void this.refreshActiveRuns();
        void this.refreshAttention();
        break;
      case "run.failed":
        state.run = { ...state.run, status: "failed" };
        state.activeRuns = state.activeRuns.filter((run) => run.id !== event.run_id);
        state.streamingText = "";
        void this.refreshChildren(state.run.id);
        void this.refreshMessages();
        void this.refreshActiveRuns();
        void this.refreshAttention();
        break;
      case "run.cancelled":
        state.run = { ...state.run, status: "cancelled" };
        state.activeRuns = state.activeRuns.filter((run) => run.id !== event.run_id);
        state.streamingText = "";
        void this.refreshChildren(state.run.id);
        void this.refreshMessages();
        void this.refreshActiveRuns();
        void this.refreshAttention();
        break;
      default:
        if (event.type.startsWith("child.")) void this.refreshChildren(event.run_id);
        break;
    }
    notify();
  }

  private rebuildAttentionFromEvent(event: EventEnvelope): void {
    if (event.type !== "tool.approval_required") return;
    state.pendingApproval = {
      id: String(event.payload.approval_id ?? ""),
      runId: event.run_id,
      toolCallId: String(event.payload.tool_call_id ?? ""),
      toolName: String(event.payload.tool_name ?? translate(state.locale, "unknownTool")),
      args: (event.payload.args as Record<string, unknown> | undefined) ?? {},
      expiresAt: Number(event.payload.expires_at ?? 0),
    };
    state.inspectorOpen = true;
  }

  private async refreshMessages(): Promise<void> {
    const sessionID = state.currentSessionID;
    if (!sessionID) return;
    try {
      const result = await api.listMessages(sessionID);
      if (state.currentSessionID !== sessionID) return;
      state.messages = result.messages;
      state.messagesPhase = "ready";
      notify();
    } catch (error) {
      state.messagesError = failureMessage(error);
      notify();
    }
  }

  private stopSubscription(): void {
    this.subscription?.close();
    this.subscription = null;
  }
}
