// Typed client for the Vivy HTTP API. The DTO shapes mirror
// internal/httpapi exactly; the FR-11 error envelope surfaces as
// ApiError so views can distinguish codes without parsing strings.

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

export interface Preflight {
  status: "ready" | "warning" | "blocked";
  mode: RunMode;
  selected_tools: string[];
  context_bytes: number;
  warnings: string[];
  blockers: string[];
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
  expires_at: number;
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

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      headers: body === undefined ? undefined : { "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch (err) {
    throw new ApiError(0, "network", `cannot reach the Vivy process: ${err}`);
  }
  if (!res.ok) {
    let code = "internal_error";
    let message = `HTTP ${res.status}`;
    try {
      const env = (await res.json()) as { error?: { code?: string; message?: string } };
      if (env.error?.code) code = env.error.code;
      if (env.error?.message) message = env.error.message;
    } catch {
      // Non-JSON error body: keep the generic shape.
    }
    throw new ApiError(res.status, code, message);
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return (await res.json()) as T;
}

// --- sessions ---

export function listSessions(): Promise<{ sessions: Session[] }> {
  return request("GET", "/api/sessions");
}

export function createSession(title: string): Promise<Session> {
  return request("POST", "/api/sessions", { title });
}

export function renameSession(id: string, title: string): Promise<Session> {
  return request("PATCH", `/api/sessions/${id}`, { title });
}

export function deleteSession(id: string): Promise<void> {
  return request("DELETE", `/api/sessions/${id}`);
}

export function listMessages(sessionID: string): Promise<{ messages: Message[] }> {
  return request("GET", `/api/sessions/${sessionID}/messages`);
}

// --- runs ---

export function postMessage(sessionID: string, text: string, mode: RunMode): Promise<{ run_id: string; status: RunStatus }> {
  return request("POST", `/api/sessions/${sessionID}/messages`, { text, mode });
}

export function preflight(sessionID: string, text: string, mode: RunMode): Promise<Preflight> {
  return request("POST", `/api/sessions/${sessionID}/preflight`, { text, mode });
}

export function getRun(runID: string): Promise<Run> {
  return request("GET", `/api/runs/${runID}`);
}

export function cancelRun(runID: string): Promise<{ run_id: string; status: string }> {
  return request("POST", `/api/runs/${runID}/cancel`);
}

export function listBackgroundRuns(): Promise<{ runs: BackgroundRun[] }> {
  return request("GET", "/api/background/runs");
}

export function attachBackgroundRun(runID: string): Promise<BackgroundRun> {
  return request("POST", `/api/background/runs/${runID}/attach`);
}

// --- approvals ---

export function listApprovals(): Promise<{ approvals: Approval[] }> {
  return request("GET", "/api/approvals");
}

export function decideApproval(approvalID: string, decision: Decision): Promise<{
  approval_id: string;
  run_id: string;
  decision: Decision;
}> {
  return request("POST", `/api/approvals/${approvalID}/decision`, { decision });
}

export function listQuestions(): Promise<{ questions: Question[] }> {
  return request("GET", "/api/questions");
}

export function answerQuestion(questionID: string, answer: string): Promise<{
  question_id: string;
  run_id: string;
  answer: string;
}> {
  return request("POST", `/api/questions/${questionID}/answer`, { answer });
}
