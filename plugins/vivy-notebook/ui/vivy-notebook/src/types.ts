/**
 * Module-local mirrors of the sealed notebook DTOs owned by
 * `internal/notebookcontract`. Field names match the backend JSON exactly;
 * tests pin representative serialized requests to that contract. The wire
 * carries no scope/actor/origin/provenance fields — the ActionHost binds those
 * from the authenticated identity, so no interface below may carry them.
 */

export const NOTEBOOK_MODULE_ID = 'vivy/notebook-core' as const;

export const NOTEBOOK_ACTIONS = Object.freeze({
  sectionsList: 'vivy.notebook.sections.list',
  sectionsCreate: 'vivy.notebook.sections.create',
  sectionsUpdate: 'vivy.notebook.sections.update',
  sectionsDelete: 'vivy.notebook.sections.delete',
  sectionsRestore: 'vivy.notebook.sections.restore',
  entriesList: 'vivy.notebook.entries.list',
  entriesGet: 'vivy.notebook.entries.get',
  entriesCreate: 'vivy.notebook.entries.create',
  entriesSave: 'vivy.notebook.entries.save',
  entriesMove: 'vivy.notebook.entries.move',
  entriesDelete: 'vivy.notebook.entries.delete',
  entriesRestore: 'vivy.notebook.entries.restore',
  revisionsList: 'vivy.notebook.revisions.list',
  revisionsAdopt: 'vivy.notebook.revisions.adopt',
  commentsList: 'vivy.notebook.comments.list',
  commentsCreate: 'vivy.notebook.comments.create',
  commentsUpdate: 'vivy.notebook.comments.update',
  export: 'vivy.notebook.export',
} as const);

/** Design bounds mirrored for byte counters (UTF-8 bytes, not runes). */
export const NOTEBOOK_MAX_BODY_BYTES = 256 * 1024;
export const NOTEBOOK_MAX_COMMENT_BYTES = 16 * 1024;
export const NOTEBOOK_MAX_PAGE_ROWS = 100;

/** Stable domain error codes of the bounded result envelope. */
export const NOTEBOOK_ERROR_CODES = Object.freeze({
  invalidRequest: 'invalid_request',
  notFound: 'not_found',
  revisionConflict: 'revision_conflict',
  idempotencyConflict: 'idempotency_conflict',
  sectionNotEmpty: 'section_not_empty',
  sectionInUse: 'section_in_use',
  limitExceeded: 'limit_exceeded',
  capabilityUnavailable: 'capability_unavailable',
  storageUnavailable: 'storage_unavailable',
  cancelled: 'cancelled',
  recoveryRequired: 'recovery_required',
  outcomeUnknown: 'outcome_unknown',
  destinationDeleted: 'destination_deleted',
} as const);

export type NotebookErrorCode = (typeof NOTEBOOK_ERROR_CODES)[keyof typeof NOTEBOOK_ERROR_CODES];

export type SystemRole = 'notes' | 'daily' | 'weekly' | 'monthly';
export type EntryKind = 'note' | 'report';
export type RevisionOrigin = 'legacy' | 'human' | 'agent' | 'generated';
export type CommentStatus = 'active' | 'resolved' | 'deleted';

export interface Section {
  readonly id: string;
  readonly title: string;
  readonly system_role?: SystemRole | '';
  readonly version: number;
  readonly created_at: number;
  readonly updated_at: number;
  readonly deleted_at?: number;
}

export interface Entry {
  readonly id: string;
  readonly section_id: string;
  readonly kind: EntryKind;
  readonly title: string;
  readonly head_revision_id: string;
  readonly version: number;
  readonly report_series_id?: string;
  readonly report_window_id?: string;
  readonly created_at: number;
  readonly updated_at: number;
  readonly deleted_at?: number;
}

export interface Revision {
  readonly id: string;
  readonly entry_id: string;
  readonly sequence: number;
  readonly parent_revision_id?: string;
  readonly title: string;
  readonly markdown: string;
  readonly origin: RevisionOrigin;
  readonly base_revision_id?: string;
  readonly actor: string;
  readonly created_at: number;
}

export interface Comment {
  readonly id: string;
  readonly entry_id: string;
  readonly anchor_revision_id?: string;
  readonly body: string;
  readonly version: number;
  readonly author: string;
  readonly status: CommentStatus;
  readonly created_at: number;
  readonly updated_at: number;
}

export interface MutationReceipt {
  readonly resource_id: string;
  readonly version: number;
  readonly revision_id?: string;
  readonly replayed: boolean;
}

export interface SectionPage {
  readonly sections: readonly Section[];
  readonly next_cursor?: string;
}

export interface EntryPage {
  readonly entries: readonly Entry[];
  readonly next_cursor?: string;
}

export interface EntryView {
  readonly entry: Entry;
  readonly revision: Revision;
}

export interface RevisionPage {
  readonly revisions: readonly Revision[];
  readonly next_cursor?: string;
}

export interface CommentPage {
  readonly comments: readonly Comment[];
  readonly next_cursor?: string;
}

export interface ExportBundle {
  readonly entry: Entry;
  readonly revision: Revision;
  readonly comments: readonly Comment[];
}

/** The bounded result envelope every notebook action returns. */
export interface ActionOutcome<T> {
  readonly status: 'ok' | 'error' | string;
  readonly data?: T;
  readonly error?: {
    readonly code: NotebookErrorCode | string;
    readonly message: string;
    readonly retryable: boolean;
    readonly current_version?: number;
    readonly current_revision_id?: string;
  };
}

/* --- Reports (R1 `vivy.reports.*` actions, owner `vivy/reports`) --- */

export const REPORTS_MODULE_ID = 'vivy/reports' as const;

export const REPORT_ACTIONS = Object.freeze({
  generate: 'vivy.reports.generate',
  get: 'vivy.reports.get',
  cancel: 'vivy.reports.cancel',
  settingsRead: 'vivy.reports.settings.read',
  settingsWrite: 'vivy.reports.settings.write',
} as const);

export type ReportPeriod = 'daily' | 'weekly' | 'monthly';
export type ReportWindowSelector = 'current' | 'completed';

export interface ReportAdmission {
  readonly run_id: string;
  readonly created: boolean;
  readonly rejoined: boolean;
  readonly busy: boolean;
}

export interface ReportSettings {
  readonly scope: string;
  readonly period: ReportPeriod;
  readonly timezone: string;
  readonly section_id: string;
  readonly provider?: string;
  readonly model_id?: string;
  readonly enabled: boolean;
  readonly revision: number;
  readonly schedule_expr?: string;
}

export interface ReportSettingsWriteResult {
  readonly settings: ReportSettings;
  readonly replayed: boolean;
}

export interface GenerationProvenance {
  readonly run_id: string;
  readonly scope: string;
  readonly series_id: string;
  readonly window_id: string;
  readonly config_revision: number;
  readonly timezone: string;
  readonly start_ms: number;
  readonly end_ms: number;
  readonly as_of_ms: number;
  readonly input_digest: string;
  readonly facts_digest: string;
  readonly provider?: string;
  readonly model_id?: string;
  readonly outcome_mode: string;
  readonly outcome_reason?: string;
  readonly entry_id: string;
  readonly revision_id: string;
}

export interface ReportResult {
  readonly run_id: string;
  readonly status: string;
  readonly generation?: GenerationProvenance;
}

/** The bounded report outcome envelope — `result`, not `data`. */
export interface ReportOutcome<T> {
  readonly status: 'ok' | 'error' | string;
  readonly result?: T;
  readonly error?: {
    readonly code: NotebookErrorCode | string;
    readonly message: string;
    readonly retryable: boolean;
  };
}
