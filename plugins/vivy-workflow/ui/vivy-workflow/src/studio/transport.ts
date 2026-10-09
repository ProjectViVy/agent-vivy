// StudioTransport is the single seam between the editor UI and any
// host (App HTTP, ViVy RPC). The UI never calls fetch itself; hosts
// supply one implementation. All methods return typed promises and
// surface backend errors as TransportError carrying the §11.4
// envelope verbatim.

import type {
  Artifact,
  ApiError,
  ConnectionView,
  DraftView,
  JsonValue,
  NodeDescriptor,
  PublishView,
  RevisionView,
  RunDetail,
  RunEvent,
  RunSummary,
  WorkflowSummary,
} from "./schema";

export class TransportError extends Error {
  readonly code: string;
  readonly diagnostics?: ApiError["diagnostics"];
  readonly status: number;

  constructor(status: number, body: ApiError) {
    super(body.message);
    this.status = status;
    this.code = body.code;
    this.diagnostics = body.diagnostics;
  }
}

export interface EventPage {
  events: RunEvent[];
  next_cursor: string | null;
}

export type WorkflowRunIntent = {
  workflow: string;
  revision?: number;
  draft_etag?: string;
  input?: JsonValue;
};

export type WorkflowStartRequest = Readonly<WorkflowRunIntent & {
  session_id: string;
  parent_run_id: string;
  operation_id: string;
}>;

// EventSubscription streams committed run events. close() detaches;
// callers resume by re-calling events() with the last seq seen.
export interface EventSubscription {
  close(): void;
}

export interface StudioTransport {
  capabilities(): Promise<Record<string, unknown>>;
  nodeTypes(): Promise<NodeDescriptor[]>;

  listWorkflows(cursor?: string): Promise<{ items: WorkflowSummary[]; next_cursor: string | null }>;
  loadDraft(id: string): Promise<DraftView>;
  saveDraft(id: string, artifact: Artifact, etag: string | null): Promise<DraftView>;
  validate(id: string, etag: string): Promise<{ valid?: boolean; diagnostics?: ApiError["diagnostics"] }>;
  publish(id: string, etag: string): Promise<PublishView>;
  getRevision(id: string, revision: number): Promise<RevisionView>;

  prepareStartRun(intent: WorkflowRunIntent): WorkflowStartRequest;
  startRun(request: WorkflowStartRequest): Promise<{ run_id: string }>;
  listRuns(cursor?: string): Promise<{ items: RunSummary[]; next_cursor: string | null }>;
  getRun(id: string): Promise<RunDetail>;
  nodeOutput(id: string, node: string): Promise<{ output?: unknown }>;
  cancelRun(id: string): Promise<{ ok: boolean }>;
  resumeRun(id: string, answers: Record<string, unknown>): Promise<{ run_id: string; status: string }>;

  listConnections(): Promise<ConnectionView[]>;
  putConnection(
    id: string,
    body: { kind: string; base_url: string; model: string; api_key?: string },
  ): Promise<{ ok: boolean }>;
  deleteConnection(id: string): Promise<{ ok: boolean }>;
  events(id: string, afterSeq?: number): Promise<EventPage>;
  subscribeEvents(
    id: string,
    afterSeq: number | undefined,
    onEvent: (e: RunEvent) => void,
    onError?: (err: unknown) => void,
  ): EventSubscription;
}
