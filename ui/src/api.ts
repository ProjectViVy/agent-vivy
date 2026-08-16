// Typed client for the Vivy local JSON-RPC control plane. HTTP is used only
// by rpc.ts for the one-time WebSocket bootstrap token.

import { getRpcClient, RpcClientError } from "./rpc";

export type RunStatus =
  | "accepted"
  | "queued"
  | "active"
  | "completed"
  | "failed"
  | "cancelled";

export type RunMode = "normal" | "plan";

export interface Session {
  id: string;
  title: string;
  created_at: number;
}

export interface Message {
  id: string;
  run_id?: string;
  role: "user" | "assistant";
  content: string;
  created_at: number;
}

export interface Run {
  id: string;
  session_id: string;
  status: RunStatus;
  created_at: number;
}

export interface RunLogEvent {
  run_id: string;
  seq: number;
  type: string;
  created_at: number;
  payload_version: number;
  payload: Record<string, unknown>;
}

// Child runs are durable descendants controlled through the same local RPC
// plane. The UI keeps the wire shape backend-authoritative and exposes the
// persisted tree from the run inspector.
export interface ChildRun {
	id: string;
	parent_run_id: string;
	root_run_id: string;
	session_id: string;
	status: RunStatus;
	depth: number;
	workspace_id?: string;
	result?: string;
	error?: string;
	created_at: number;
}

export interface Preflight {
  status: "ready" | "warning" | "blocked";
  mode: RunMode;
  policy_profile: string;
  policy_hash?: string;
  selected_tools: string[];
  tool_decisions: Array<{ tool_name: string; decision: string; reason: string }>;
  context_bytes: number;
  hook_ready: boolean;
  warnings?: string[];
  blockers?: string[];
  next_actions?: string[];
}

export interface Approval {
  id: string;
  run_id: string;
  tool_call_id: string;
  expires_at: number;
}

export interface Question {
  id: string;
  run_id: string;
  tool_call_id: string;
  prompt: string;
  status?: "pending" | "answered" | "expired";
  expires_at: number;
}

export type ReviewKind = "approval" | "question";
export type ReviewStatus = "pending" | "approved" | "denied" | "answered" | "cancelled" | "expired" | "stale";

export interface ReviewItem {
  id: string;
  kind: ReviewKind;
  status: ReviewStatus;
  session_id: string;
  session_title?: string;
  run_id: string;
  tool_call_id?: string;
  tool_name?: string;
  source?: string;
  actor?: string;
  created_at: number;
  expires_at: number;
  decided_at?: number;
  action?: string;
  target?: string;
  precondition_hash?: string;
  preview?: string;
  risk_findings?: string[];
  arguments?: Record<string, unknown>;
  prompt?: string;
  decision_reason?: string;
  stale_reason?: string;
  error?: string;
  effect?: string;
  reversibility?: string;
  scope?: string;
  trust?: string;
}

export interface BackgroundRun extends Run {
  workspace_id?: string;
  events_url: string;
  logs_url: string;
}

export type Decision = "approved" | "denied";

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

function rpcStatus(code: number): { status: number; name: string } {
  switch (code) {
    case -32004:
      return { status: 404, name: "not_found" };
    case -32009:
      return { status: 409, name: "conflict" };
    case -32602:
      return { status: 400, name: "invalid_request" };
    case -32601:
      return { status: 404, name: "not_found" };
    default:
      return { status: 500, name: "internal_error" };
  }
}

async function request<T>(method: string, params?: unknown): Promise<T> {
  try {
    return await (await getRpcClient()).call<T>(method, params);
  } catch (error) {
    if (error instanceof RpcClientError) {
      const mapped = rpcStatus(error.code);
      throw new ApiError(mapped.status, mapped.name, error.message);
    }
    throw new ApiError(0, "network", `cannot reach the Vivy control plane: ${error}`);
  }
}

// --- sessions ---

export function listSessions(): Promise<{ sessions: Session[] }> {
  return request("session/list");
}

export function createSession(title: string): Promise<Session> {
  return request("session/create", { title });
}

export function renameSession(id: string, title: string): Promise<Session> {
  return request("session/rename", { session_id: id, title });
}

export function deleteSession(id: string): Promise<void> {
  return request("session/delete", { session_id: id }).then(() => undefined);
}

export function listMessages(sessionID: string): Promise<{ messages: Message[] }> {
  return request("session/messages", { session_id: sessionID });
}

// --- runs ---

export function postMessage(sessionID: string, text: string, mode: RunMode): Promise<{ run_id: string; status: RunStatus }> {
  return request("turn/start", { session_id: sessionID, text, mode });
}

export function preflight(sessionID: string, text: string, mode: RunMode): Promise<Preflight> {
  return request("preflight/run", { session_id: sessionID, text, mode });
}

export function getRun(runID: string): Promise<Run> {
  return request("run/get", { run_id: runID });
}

export function getRunLog(runID: string, afterSeq = 0): Promise<{ events: RunLogEvent[] }> {
  return request("run/log", { run_id: runID, after_seq: afterSeq });
}

export function cancelRun(runID: string): Promise<{ run_id: string; status: string }> {
  return request("run/cancel", { run_id: runID });
}

function withBackgroundLinks<T extends Run & { workspace_id?: string }>(run: T): BackgroundRun {
  return { ...run, events_url: "", logs_url: "" };
}

export async function listBackgroundRuns(): Promise<{ runs: BackgroundRun[] }> {
  const result = await request<{ runs: Array<Run & { workspace_id?: string }> }>("background/list");
  return { runs: result.runs.map(withBackgroundLinks) };
}

export async function attachBackgroundRun(runID: string): Promise<BackgroundRun> {
  return withBackgroundLinks(await request<Run & { workspace_id?: string }>("background/attach", { run_id: runID }));
}

export function startChild(childRequest: { parent_run_id: string; text: string; policy_profile?: string; tool_names?: string[] }): Promise<ChildRun> {
  return request("child/start", childRequest);
}

export function getChild(runID: string): Promise<ChildRun> {
  return request("child/get", { run_id: runID });
}

export function listChildren(parentRunID: string, tree = false): Promise<{ children: ChildRun[] }> {
  return request("child/list", { parent_run_id: parentRunID, tree });
}

export function waitChild(runID: string): Promise<ChildRun> {
  return request("child/wait", { run_id: runID });
}

export function cancelChild(runID: string): Promise<ChildRun> {
  return request("child/cancel", { run_id: runID });
}

// --- approvals ---

export function listApprovals(): Promise<{ approvals: Approval[] }> {
  return request("approval/list");
}

export function decideApproval(approvalID: string, decision: Decision): Promise<{
  approval_id: string;
  run_id: string;
  decision: Decision;
}> {
  return request("approval/respond", { approval_id: approvalID, decision });
}

export function listQuestions(): Promise<{ questions: Question[] }> {
  return request("question/list");
}

export function answerQuestion(questionID: string, answer: string): Promise<{
  question_id: string;
  run_id: string;
  answer: string;
}> {
  return request("question/respond", { question_id: questionID, answer });
}

export function listReviews(params: { kind?: ReviewKind; status?: ReviewStatus; session_id?: string; limit?: number } = {}): Promise<{ reviews: ReviewItem[] }> {
  return request("review/list", params);
}

export function getReview(reviewID: string): Promise<ReviewItem> {
  return request("review/get", { review_id: reviewID });
}

export function respondReview(reviewID: string, response: { action: "approve" | "deny" | "answer" | "cancel"; reason?: string; answer?: string }): Promise<{ review_id: string; status: string }> {
  return request("review/respond", { review_id: reviewID, ...response });
}

export interface SpeciesInspect {
  protocol_version: string;
  binary_id: string;
  generation_id: string;
  artifact_sha256?: string;
  recipe: { loop?: string; world?: string; providers?: string[]; tools?: string[]; plugins?: string[] };
  policy_profile: string;
  policy_hash: string;
  tools: Array<{ name: string; readonly: boolean }>;
  grants: string[];
}

export interface Generation {
  id: string;
  parent_id?: string;
  artifact_sha256: string;
  source_ref?: string;
  recipe: SpeciesInspect["recipe"];
  phase: "built" | "evaluated" | "promoted" | "rejected";
  created_at: number;
}

export interface EvalRun {
  id: string;
  candidate_id: string;
  baseline_id?: string;
  suite: string;
  verdict: "better" | "worse" | "mixed" | "failed_to_run";
  journal_ref?: string;
  created_at: number;
}

export interface Promotion {
  id: string;
  from_id: string;
  to_id: string;
  eval_id: string;
  actor: string;
  phase: string;
  applies_at: string;
  created_at: number;
}

export function inspectSpecies(): Promise<SpeciesInspect> {
  return request("species/inspect");
}

export function listGenerations(): Promise<{ generations: Generation[] }> {
  return request("generations/list");
}

export function createGeneration(params: { artifact_sha256: string; parent_id?: string; recipe?: Generation["recipe"] }): Promise<Generation> {
  return request("generations/create", params);
}

export function rejectGeneration(id: string): Promise<Generation> {
  return request("generations/reject", { id });
}

export function listEvals(): Promise<{ evals: EvalRun[] }> {
  return request("evals/list");
}

export function recordEval(params: { candidate_id: string; baseline_id?: string; suite: string; verdict: EvalRun["verdict"] }): Promise<EvalRun> {
  return request("evals/record", params);
}

export function listPromotions(): Promise<{ promotions: Promotion[] }> {
  return request("promotions/list");
}

export function promoteGeneration(params: { from_id: string; to_id: string; actor?: string }): Promise<Promotion> {
  return request("promotions/promote", params);
}
