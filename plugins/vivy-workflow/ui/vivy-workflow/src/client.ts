/**
 * Typed `inofy.*` client over the VIVY host bridge.
 *
 * Every method maps onto one `inofy.<name>` host action; failures arrive as
 * TransportError carrying the host error code verbatim. `statusFor` maps the
 * codes onto the App §11.4 HTTP statuses so the editor's branch checks
 * (no-draft-yet, CAS conflict, unsupported) keep their meaning.
 *
 * Paged `events` rows are normalized through the same journal->engine
 * vocabulary map used by the live subscription, so the UI renders committed
 * host facts in one shape regardless of source.
 */
import type { HostBridge } from './face-bridge';
import { normalizeJournalEvent } from './face-bridge';
import type {
  ApiError,
  Artifact,
  ConnectionView,
  DraftView,
  NodeDescriptor,
  PublishView,
  RevisionView,
  RunDetail,
  RunSummary,
  WorkflowSummary,
} from './studio/schema';
import type { EventPage, EventSubscription, WorkflowRunIntent, WorkflowStartRequest } from './studio/transport';
import { TransportError } from './studio/transport';

function statusFor(code: string): number {
  switch (code) {
    case 'not_found':
    case 'revision_conflict':
      return 412;
    case 'validation_failed':
    case 'missing_binding':
    case 'missing_tool_args':
    case 'missing_keys':
    case 'invalid_input':
      return 422;
    case 'unsupported_feature':
      return 501;
    case 'unavailable':
      return 503;
    case 'idempotency_conflict':
      return 409;
    case 'unauthenticated':
      return 401;
    default:
      return 0;
  }
}

interface RPCErrorShape {
  code?: string;
  message?: string;
  data?: ApiError;
}

export class WorkflowClient {
  constructor(private bridge: HostBridge) {}

  private async call<T>(method: string, params?: unknown): Promise<T> {
    try {
      return await this.bridge.call<T>(`inofy.${method}`, params);
    } catch (e) {
      const err = e as RPCErrorShape;
      const code = err.data?.code ?? err.code ?? 'rpc_error';
      throw new TransportError(statusFor(code), {
        code,
        message: err.data?.message ?? err.message ?? String(e),
        diagnostics: err.data?.diagnostics,
      });
    }
  }

  capabilities() {
    return this.call<Record<string, unknown>>('capabilities');
  }
  nodeTypes() {
    return this.call<NodeDescriptor[]>('nodeTypes');
  }
  listWorkflows(cursor?: string) {
    return this.call<{ items: WorkflowSummary[]; next_cursor: string | null }>('listWorkflows', { cursor });
  }
  loadDraft(id: string) {
    return this.call<DraftView>('loadDraft', { workflow: id });
  }
  async saveDraft(id: string, artifact: Artifact, etag: string | null) {
    if (etag === '') throw new TypeError('workflow draft etag must not be empty');
    const params = etag === null
      ? { workflow: id, artifact, create: true }
      : { workflow: id, artifact, etag };
    return this.call<DraftView>('saveDraft', params);
  }
  validate(id: string, etag: string) {
    return this.call<{ valid?: boolean; diagnostics?: ApiError['diagnostics'] }>('validate', { workflow: id, etag });
  }
  publish(id: string, etag: string) {
    return this.call<PublishView>('publish', { workflow: id, etag });
  }
  getRevision(id: string, revision: number) {
    return this.call<RevisionView>('getRevision', { workflow: id, revision });
  }
  prepareStartRun(intent: WorkflowRunIntent) {
    return this.bridge.prepareStartRun(intent);
  }
  startRun(request: WorkflowStartRequest) {
    return this.call<{ run_id: string }>('startRun', request);
  }
  listRuns(cursor?: string) {
    return this.call<{ items: RunSummary[]; next_cursor: string | null }>('listRuns', { cursor });
  }
  getRun(id: string) {
    return this.call<RunDetail>('getRun', { run_id: id });
  }
  nodeOutput(id: string, node: string) {
    return this.call<{ output?: unknown }>('nodeOutput', { run_id: id, node });
  }
  cancelRun(id: string) {
    return this.call<{ ok: boolean }>('cancelRun', { run_id: id });
  }
  resumeRun(id: string, answers: Record<string, unknown>) {
    return this.call<{ run_id: string; status: string }>('resumeRun', { run_id: id, answers });
  }
  listConnections() {
    return this.call<ConnectionView[]>('listConnections');
  }

  async events(id: string, afterSeq?: number): Promise<EventPage> {
    const page = await this.call<EventPage>('events', { run_id: id, after: afterSeq });
    return {
      ...page,
      events: page.events.map((e) => normalizeJournalEvent(e as unknown as Parameters<typeof normalizeJournalEvent>[0])),
    };
  }

  subscribeEvents(
    id: string,
    afterSeq: number | undefined,
    onEvent: (data: unknown) => void,
    onError?: (err: unknown) => void,
  ): EventSubscription {
    return this.bridge.subscribe('inofy.events', { run_id: id, after: afterSeq }, onEvent, onError);
  }
}
