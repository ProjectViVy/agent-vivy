/**
 * Typed adapter over the module-action transport for the sealed notebook
 * inventory (N2, `internal/modules/notebook/actions.go`). The adapter encodes
 * the {operation_key, request} mutation envelope and decodes the bounded
 * result envelope; it never supplies scope, actor, origin, or provenance —
 * those are bound server-side from the authenticated identity.
 */
import { createModuleActionClient, type FaceClientRPC } from '@vivy/ui-sdk';
import {
  NOTEBOOK_ACTIONS, NOTEBOOK_MODULE_ID,
  REPORT_ACTIONS, REPORTS_MODULE_ID,
  type ReportAdmission, type ReportOutcome, type ReportPeriod,
  type ReportResult, type ReportSettings, type ReportWindowSelector,
  type CommentPage, type EntryPage, type EntryView, type ExportBundle,
  type MutationReceipt, type RevisionPage, type SectionPage,
  type ActionOutcome, type NotebookErrorCode, type CommentStatus,
} from './types';

export class NotebookError extends Error {
  constructor(
    public readonly code: NotebookErrorCode | string,
    message: string,
    public readonly retryable: boolean,
    public readonly currentVersion?: number,
    public readonly currentRevisionId?: string,
  ) {
    super(message);
    this.name = 'NotebookError';
  }
}

/** Caller-owned idempotency keys; identical retries reuse the stored receipt. */
export function newOperationKey(): string {
  return `nb-${crypto.randomUUID()}`;
}

export interface NotebookActionTransport {
  invoke<Input = unknown, Result = unknown>(request: {
    readonly moduleId: string;
    readonly actionId: string;
    readonly input: Input;
  }): Promise<Result>;
}

type Keyed<Request> = { readonly operationKey: string } & Request;

export class NotebookClient {
  constructor(private readonly actions: NotebookActionTransport) {}

  static fromRPC(rpc: FaceClientRPC): NotebookClient {
    return new NotebookClient(createModuleActionClient(rpc));
  }

  listSections(input: { readonly cursor?: string; readonly limit?: number } = {}): Promise<SectionPage> {
    return this.read(NOTEBOOK_ACTIONS.sectionsList, input);
  }

  listEntries(input: {
    readonly section_id?: string;
    readonly cursor?: string;
    readonly limit?: number;
    readonly include_deleted?: boolean;
  }): Promise<EntryPage> {
    return this.read(NOTEBOOK_ACTIONS.entriesList, input);
  }

  getEntry(input: { readonly id: string; readonly revision_id?: string }): Promise<EntryView> {
    return this.read(NOTEBOOK_ACTIONS.entriesGet, input);
  }

  listRevisions(input: { readonly entry_id: string; readonly cursor?: string; readonly limit?: number }): Promise<RevisionPage> {
    return this.read(NOTEBOOK_ACTIONS.revisionsList, input);
  }

  listComments(input: {
    readonly entry_id: string;
    readonly cursor?: string;
    readonly limit?: number;
    readonly status?: CommentStatus;
  }): Promise<CommentPage> {
    return this.read(NOTEBOOK_ACTIONS.commentsList, input);
  }

  exportEntry(input: { readonly entry_id: string; readonly revision_id: string }): Promise<ExportBundle> {
    return this.read(NOTEBOOK_ACTIONS.export, input);
  }

  createSection(input: Keyed<{ readonly title: string }>): Promise<MutationReceipt> {
    return this.mutate(NOTEBOOK_ACTIONS.sectionsCreate, input);
  }

  updateSection(input: Keyed<{ readonly id: string; readonly title: string; readonly expected_version: number }>): Promise<MutationReceipt> {
    return this.mutate(NOTEBOOK_ACTIONS.sectionsUpdate, input);
  }

  deleteSection(input: Keyed<{ readonly id: string; readonly expected_version: number }>): Promise<MutationReceipt> {
    return this.mutate(NOTEBOOK_ACTIONS.sectionsDelete, input);
  }

  restoreSection(input: Keyed<{ readonly id: string; readonly expected_version: number }>): Promise<MutationReceipt> {
    return this.mutate(NOTEBOOK_ACTIONS.sectionsRestore, input);
  }

  createEntry(input: Keyed<{ readonly section_id: string; readonly title: string; readonly markdown: string }>): Promise<MutationReceipt> {
    return this.mutate(NOTEBOOK_ACTIONS.entriesCreate, input);
  }

  saveEntry(input: Keyed<{
    readonly entry_id: string;
    readonly expected_version: number;
    readonly base_revision_id: string;
    readonly title: string;
    readonly markdown: string;
  }>): Promise<MutationReceipt> {
    return this.mutate(NOTEBOOK_ACTIONS.entriesSave, input);
  }

  moveEntry(input: Keyed<{ readonly entry_id: string; readonly section_id: string; readonly expected_version: number }>): Promise<MutationReceipt> {
    return this.mutate(NOTEBOOK_ACTIONS.entriesMove, input);
  }

  deleteEntry(input: Keyed<{ readonly entry_id: string; readonly expected_version: number }>): Promise<MutationReceipt> {
    return this.mutate(NOTEBOOK_ACTIONS.entriesDelete, input);
  }

  restoreEntry(input: Keyed<{ readonly entry_id: string; readonly expected_version: number }>): Promise<MutationReceipt> {
    return this.mutate(NOTEBOOK_ACTIONS.entriesRestore, input);
  }

  adoptRevision(input: Keyed<{ readonly entry_id: string; readonly revision_id: string; readonly expected_version: number }>): Promise<MutationReceipt> {
    return this.mutate(NOTEBOOK_ACTIONS.revisionsAdopt, input);
  }

  createComment(input: Keyed<{ readonly entry_id: string; readonly anchor_revision_id?: string; readonly body: string }>): Promise<MutationReceipt> {
    return this.mutate(NOTEBOOK_ACTIONS.commentsCreate, input);
  }

  updateComment(input: Keyed<{
    readonly comment_id: string;
    readonly expected_version: number;
    readonly body?: string;
    readonly status?: CommentStatus;
  }>): Promise<MutationReceipt> {
    return this.mutate(NOTEBOOK_ACTIONS.commentsUpdate, input);
  }

  private read<Request extends object, Result>(actionId: string, input: Request): Promise<Result> {
    return this.invoke<Request, Result>(actionId, input);
  }

  private mutate<Request extends object, Result>(actionId: string, input: Keyed<Request>): Promise<Result> {
    const { operationKey, ...request } = input;
    return this.invoke(actionId, { operation_key: operationKey, request });
  }

  private async invoke<Request, Result>(actionId: string, input: Request): Promise<Result> {
    let outcome: ActionOutcome<Result> | undefined;
    try {
      outcome = await this.actions.invoke<Request, ActionOutcome<Result>>({
        moduleId: NOTEBOOK_MODULE_ID,
        actionId,
        input,
      });
    } catch (cause) {
      // A generation without the notebook module answers -32004
      // (action/module not found) or an unavailable message at the RPC layer —
      // before the action ever runs. Surface that as capability_unavailable so
      // the view can render its omitted-generation state instead of an error.
      const code = (cause as { code?: unknown })?.code;
      const message = cause instanceof Error ? cause.message : String(cause);
      if (code === -32004 || code === -32601 || /not configured|not found|unavailable/i.test(message)) {
        throw new NotebookError('capability_unavailable', message, false);
      }
      throw cause;
    }
    if (outcome != null && outcome.status === 'ok') {
      return outcome.data as Result;
    }
    if (outcome != null && outcome.status === 'error' && outcome.error) {
      throw new NotebookError(
        outcome.error.code, outcome.error.message, outcome.error.retryable,
        outcome.error.current_version, outcome.error.current_revision_id,
      );
    }
    throw new NotebookError('outcome_unknown', 'notebook action returned an unrecognized outcome', true);
  }
}

/**
 * Reports adapter over the sealed `vivy.reports.*` inventory (R1). Same
 * authority rule as the notebook adapter: scope/actor/origin are bound
 * server-side; the wire carries only period/window/target/operation_key.
 * The outcome envelope names its success field `result`, not `data`.
 */
export class ReportsClient {
  constructor(private readonly actions: NotebookActionTransport) {}

  static fromRPC(rpc: FaceClientRPC): ReportsClient {
    return new ReportsClient(createModuleActionClient(rpc));
  }

  generate(input: {
    readonly period: ReportPeriod;
    readonly window: ReportWindowSelector;
    readonly operationKey: string;
    readonly target?: { readonly section_id?: string; readonly entry_id?: string };
  }): Promise<ReportAdmission> {
    const { operationKey, ...request } = input;
    return this.invoke(REPORT_ACTIONS.generate, { operation_key: operationKey, ...request });
  }

  get(input: { readonly run_id: string }): Promise<ReportResult> {
    return this.invoke(REPORT_ACTIONS.get, input);
  }

  cancel(input: { readonly run_id: string }): Promise<void> {
    return this.invoke(REPORT_ACTIONS.cancel, input).then(() => undefined);
  }

  readSettings(input: { readonly period: ReportPeriod }): Promise<ReportSettings> {
    return this.invoke(REPORT_ACTIONS.settingsRead, input);
  }

  private async invoke<Request extends object, Result>(actionId: string, input: Request): Promise<Result> {
    let outcome: ReportOutcome<Result> | undefined;
    try {
      outcome = await this.actions.invoke<Request, ReportOutcome<Result>>({
        moduleId: REPORTS_MODULE_ID,
        actionId,
        input,
      });
    } catch (cause) {
      const code = (cause as { code?: unknown })?.code;
      const message = cause instanceof Error ? cause.message : String(cause);
      if (code === -32004 || code === -32601 || /not configured|not found|unavailable/i.test(message)) {
        throw new NotebookError('capability_unavailable', message, false);
      }
      throw cause;
    }
    if (outcome != null && outcome.status === 'ok') {
      return outcome.result as Result;
    }
    if (outcome != null && outcome.status === 'error' && outcome.error) {
      throw new NotebookError(outcome.error.code, outcome.error.message, outcome.error.retryable);
    }
    throw new NotebookError('outcome_unknown', 'reports action returned an unrecognized outcome', true);
  }
}
