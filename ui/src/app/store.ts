import type { Approval, BackgroundRun, ChildRun, EvalRun, Generation, Message, Promotion, Question, ReviewItem, Run, Session, SpeciesInspect } from "../api";
import type { EventEnvelope } from "../sse";
import type { Locale, ThemeMode } from "./preferences";

export type AsyncPhase = "idle" | "loading" | "refreshing" | "ready" | "empty" | "error" | "processing";
export type ConnectionState = "idle" | "connecting" | "connected" | "reconnecting" | "error";
export type InspectorTab = "overview" | "events";

export interface PendingApprovalView {
  id: string;
  runId: string;
  toolCallId: string;
  toolName: string;
  args: Record<string, unknown>;
  expiresAt: number;
}

export interface AppState {
  sessions: Session[];
  sessionsPhase: AsyncPhase;
  sessionsError: string;
  activeRuns: BackgroundRun[];
  attachBusyRunID: string | null;
  currentSessionID: string | null;
  messages: Message[];
  messagesPhase: AsyncPhase;
  messagesError: string;
  draft: string;
  mode: "normal" | "plan";
  sendPhase: AsyncPhase;
  sendError: string;
  run: Run | null;
  streamingText: string;
  events: EventEnvelope[];
  eventLogVisible: boolean;
  inspectorOpen: boolean;
  inspectorTab: InspectorTab;
  connection: ConnectionState;
  runError: string;
  cancelBusy: boolean;
  children: ChildRun[];
  childrenPhase: AsyncPhase;
  childrenError: string;
  childCancelBusyID: string | null;
  approvals: Approval[];
  questions: Question[];
  pendingApproval: PendingApprovalView | null;
  approvalBusy: boolean;
  approvalError: string;
  pendingQuestion: Question | null;
  questionDraft: string;
  questionBusy: boolean;
  questionError: string;
  sidebarOpen: boolean;
  locale: Locale;
  theme: ThemeMode;
  reviews: ReviewItem[];
  reviewsPhase: AsyncPhase;
  reviewsError: string;
  reviewCenterOpen: boolean;
  selectedReviewID: string | null;
  reviewBusyID: string | null;
  studioOpen: boolean;
  studioInspect: SpeciesInspect | null;
  studioGenerations: Generation[];
  studioEvals: EvalRun[];
  studioPromotions: Promotion[];
  studioPhase: AsyncPhase;
  studioError: string;
  selectedGenerationID: string | null;
  studioBusy: boolean;
}

export const state: AppState = {
  sessions: [],
  sessionsPhase: "loading",
  sessionsError: "",
  activeRuns: [],
  attachBusyRunID: null,
  currentSessionID: null,
  messages: [],
  messagesPhase: "idle",
  messagesError: "",
  draft: "",
  mode: "normal",
  sendPhase: "idle",
  sendError: "",
  run: null,
  streamingText: "",
  events: [],
  eventLogVisible: false,
  inspectorOpen: false,
  inspectorTab: "overview",
  connection: "idle",
  runError: "",
  cancelBusy: false,
  children: [],
  childrenPhase: "idle",
  childrenError: "",
  childCancelBusyID: null,
  approvals: [],
  questions: [],
  pendingApproval: null,
  approvalBusy: false,
  approvalError: "",
  pendingQuestion: null,
  questionDraft: "",
  questionBusy: false,
  questionError: "",
  sidebarOpen: false,
  locale: "en",
  theme: "system",
  reviews: [],
  reviewsPhase: "idle",
  reviewsError: "",
  reviewCenterOpen: false,
  selectedReviewID: null,
  reviewBusyID: null,
  studioOpen: false,
  studioInspect: null,
  studioGenerations: [],
  studioEvals: [],
  studioPromotions: [],
  studioPhase: "idle",
  studioError: "",
  selectedGenerationID: null,
  studioBusy: false,
};

type Listener = () => void;
const listeners = new Set<Listener>();

export function subscribe(listener: Listener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function notify(): void {
  for (const listener of listeners) listener();
}

export function resetSessionView(): void {
  state.messages = [];
  state.messagesPhase = "loading";
  state.messagesError = "";
  state.draft = "";
  state.sendPhase = "idle";
  state.sendError = "";
  state.run = null;
  state.streamingText = "";
  state.events = [];
  state.eventLogVisible = false;
  state.inspectorOpen = false;
  state.inspectorTab = "overview";
  state.connection = "idle";
  state.runError = "";
  state.cancelBusy = false;
  state.children = [];
  state.childrenPhase = "idle";
  state.childrenError = "";
  state.childCancelBusyID = null;
  state.pendingApproval = null;
  state.approvalBusy = false;
  state.approvalError = "";
  state.pendingQuestion = null;
  state.questionDraft = "";
  state.questionBusy = false;
  state.questionError = "";
}

export function isRunActive(run: Run | null): boolean {
  return run !== null && !["completed", "failed", "cancelled"].includes(run.status);
}
