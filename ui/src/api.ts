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

export interface Approval {
  id: string;
  run_id: string;
  tool_call_id: string;
  expires_at: number;
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

export function postMessage(sessionID: string, text: string): Promise<{ run_id: string; status: RunStatus }> {
  return request("POST", `/api/sessions/${sessionID}/messages`, { text });
}

export function getRun(runID: string): Promise<Run> {
  return request("GET", `/api/runs/${runID}`);
}

export function cancelRun(runID: string): Promise<{ run_id: string; status: string }> {
  return request("POST", `/api/runs/${runID}/cancel`);
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
