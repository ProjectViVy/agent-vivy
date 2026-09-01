/**
 * 本地演示页面类型。真实 Vivy RPC 类型位于 lib/api.ts。
 */

// ==================== 消息相关 ====================

export interface Message {
  id: string;
  role: 'user' | 'agent' | 'system' | 'tool';
  content: string;
  reasoning?: string;
  isThinking?: boolean;
  isStreaming?: boolean;
  timestamp?: number;
  emotion?: string;
  toolName?: string;
  toolArgs?: string;
  toolResult?: string;
  toolStatus?: 'running' | 'success' | 'error';
  toolCallId?: string;
  retryStatus?: { attempt: number; maxRetries: number; model?: string };
  stalled?: boolean;
  rawMeta?: Record<string, unknown>;
  fromHistory?: boolean;
  attachments?: string[];
}

export interface AgentResponse {
  content: string;
  reasoning?: string;
  emotion?: string;
  tool_calls?: ToolCall[];
}

export interface ToolCall {
  id: string;
  name: string;
  args: Record<string, unknown>;
}

// ==================== 会话相关 ====================

export interface Session {
  id: string;
  title: string;
  created_at: string;
  updated_at: string;
  message_count: number;
  last_message_preview?: string;
}

export interface SessionHistory {
  session_id: string;
  messages: Message[];
}

// ==================== 计划相关 ====================

export interface PlanSummary {
  id: string;
  title: string;
  goal: string;
  phase: string;
  status: string;
  todo_count: number;
  todo_completed: number;
  is_active: boolean;
}

export interface PlanDetail {
  id: string;
  revision?: number | null;
  title: string;
  goal: string;
  phase: string;
  status: string;
  strategy: string | null;
  summary?: string;
  markdown?: string;
  assumptions: string[];
  risks: string[];
  open_questions: string[];
  verification_verdict: string | null;
  steps: StepDetail[];
  todos: TodoDetail[];
  created_at: string;
  updated_at: string;
}

export interface StepDetail {
  id: string;
  plan_id: string;
  ordinal: number;
  title: string;
  rationale: string | null;
  expected_output: string | null;
  status: string;
  evidence_ref: string | null;
  created_at: string;
  updated_at: string;
}

export interface TodoDetail {
  id: string;
  plan_step_id: string | null;
  title: string;
  detail: string | null;
  status: string;
  priority: string;
  evidence_ref: string | null;
  block_reason: string | null;
  updated_at: string;
}

export interface PlanRuntimeStep {
  id: string;
  ordinal: number;
  title: string;
  rationale: string | null;
  expected_output: string | null;
  status: string;
}

export interface PlanRuntimeTodo {
  id: string;
  plan_step_id: string | null;
  title: string;
  detail: string | null;
  status: string;
  priority: string;
  evidence_ref: string | null;
  block_reason: string | null;
  updated_at: string;
}

export interface PlanRuntimeState {
  plan_id: string;
  revision?: number | null;
  title: string;
  goal: string;
  phase: string;
  status: string;
  strategy: string | null;
  summary: string;
  markdown?: string;
  validation_issues?: string[];
  steps: PlanRuntimeStep[];
  todos: PlanRuntimeTodo[];
  created_at: string;
  updated_at: string;
  execution_id?: string | null;
  initialization_status?: 'Pending' | 'Ready' | 'Blocked';
  initialization_error?: string | null;
}

export interface PlanApprovalReceipt {
  plan_id: string;
  revision: number;
  approved_at: string;
  todo_policy: 'Never' | 'Optional' | 'Always';
  todos_materialized: boolean;
}

export interface PlanApprovalResult {
  plan: PlanRuntimeState;
  receipt: PlanApprovalReceipt;
}

export interface PlanStreamEvent {
  plan: PlanRuntimeState;
  todo?: PlanRuntimeTodo | null;
}

// ==================== 审批相关 ====================

export type ApprovalDomain = 'command' | 'plan';
export type ApprovalStatus = 'pending' | 'allowed' | 'denied' | 'revoked' | 'consumed' | 'expired';
export type ApprovalReasonCode =
  | 'approval_not_found'
  | 'approval_version_conflict'
  | 'approval_idempotency_conflict'
  | 'approval_expired'
  | 'approval_already_resolved'
  | 'approval_already_consumed'
  | 'approval_digest_mismatch'
  | 'approval_invalid_grant'
  | 'approval_invalid_transition'
  | 'approval_payload_unavailable'
  | 'approval_persistence_failed'
  | 'approval_required_noninteractive'
  | 'approval_queue_unavailable'
  | 'approval_outcome_unknown'
  | 'approval_denied'
  | 'approval_revoked'
  | 'approval_invalid_cursor'
  | 'approval_invalid_body'
  | 'approval_invalid_query';

export interface ApprovalResource {
  workspace_id: string;
  session_id: string | null;
  kind: string;
  resource_id: string;
  boundary: string | null;
}

export interface ApprovalView {
  request_id: string;
  version: number;
  domain: ApprovalDomain;
  capability: string;
  resource: ApprovalResource;
  risk: string;
  status: ApprovalStatus;
  created_at: string;
  expires_at: string;
  subject: { kind: string; id: string };
  evidence: unknown[];
  receipt: unknown | null;
  reason_code: ApprovalReasonCode | null;
  actions: string[];
  presentation?: Record<string, unknown> | null;
}

export interface ApprovalEventView {
  event_id: string;
  cursor: string;
  request_id: string;
  version: number;
  domain: ApprovalDomain;
  status: ApprovalStatus;
  reason_code: ApprovalReasonCode | null;
  reason: ApprovalReasonCode | null;
  occurred_at: string;
  correlation: Record<string, unknown> | null;
}

export interface ApprovalListPage {
  approvals: ApprovalView[];
  next_cursor: string | null;
}

export type ApprovalGrant = 'once' | 'session' | 'rule';
export type ApprovalDecision = 'allow' | 'deny';

export interface UnifiedApprovalApiError {
  status: number;
  reason_code: ApprovalReasonCode;
  message?: string;
}

// ==================== 技能相关 ====================

export interface SkillDto {
  slug: string;
  name: string;
  description: string;
  source: 'builtin' | 'home';
  enabled: boolean;
  always: boolean;
  available: boolean;
  active: boolean;
  content_hash: string;
  updated_at: string;
  can_hard_delete: boolean;
  evolution_managed?: boolean;
  path: string;
  can_delete: boolean;
}

export interface SkillDocument extends Omit<SkillDto, 'name' | 'active' | 'path' | 'can_delete'> {
  markdown: string;
}

export interface SkillWriteOutcome {
  document: SkillDocument;
  changed: boolean;
}

export interface SkillHistoryEntry {
  revision: number;
  content_hash: string;
  updated_at: string;
}

export interface SkillHistoryDocument {
  slug: string;
  revision: number;
  content_hash: string;
  markdown: string;
}

export interface SkillEvidence {
  session_key?: string | null;
  actmem_pointer?: string | null;
  autodream_run_id?: string | null;
  tool?: string | null;
  artifact?: string | null;
}

export type SkillRequestStatus = 'pending' | 'accepted' | 'rejected' | 'stale';
export type SkillRequestSource = 'autodream' | 'distill' | 'user_request';

export interface SkillRequest {
  id: string;
  slug: string;
  title: string;
  proposed_markdown: string;
  evidence: SkillEvidence[];
  attestation?: string | null;
  base_hash: string;
  source: SkillRequestSource;
  reason: string;
  status: SkillRequestStatus;
  created_at: string;
  updated_at: string;
}

export interface CreateSkillRequestPayload {
  slug: string;
  title: string;
  proposed_markdown: string;
  evidence?: SkillEvidence[];
  attestation?: string | null;
  base_hash: string;
  reason: string;
}

// ==================== 进化 / AutoDream 相关 ====================

export type AutoDreamRunState = 'pending' | 'running' | 'cancelled' | 'completed' | 'failed';

export type AutoDreamOrchestrationPhase =
  | 'queued'
  | 'gathering'
  | 'reflecting'
  | 'validating'
  | 'publishing'
  | 'completed'
  | 'failed'
  | 'cancelled';

export type AutoDreamFailureCode =
  | 'cancelled'
  | 'input_unavailable'
  | 'worker_timeout'
  | 'worker_failed'
  | 'report_generation_failed'
  | 'provider_unavailable'
  | 'provider_timeout'
  | 'provider_failed'
  | 'invalid_candidate';

export interface AutoDreamInputSourceSummary {
  source: string;
  included_items: number;
  total_bytes: number;
  truncated: boolean;
}

export interface AutoDreamInputSummary {
  total_items: number;
  included_sources: AutoDreamInputSourceSummary[];
  total_bytes: number;
  truncated: boolean;
}

export interface AutoDreamOrchestrationRecord {
  schema_version: number;
  phase: AutoDreamOrchestrationPhase;
  attempt: number;
  deadline_at: string;
  updated_at: string;
}

export interface AutoDreamRunRecord {
  id: string;
  started_at: string;
  completed_at: string | null;
  state: AutoDreamRunState;
  trigger: string;
  summary: string | null;
  input_summary: AutoDreamInputSummary | null;
  proposal_ids: string[];
  orchestration: AutoDreamOrchestrationRecord | null;
  failure_code: AutoDreamFailureCode | null;
  error: string | null;
}

export interface AutoDreamRunEvent {
  id: string;
  run_id: string;
  kind: string;
  message: string;
  created_at: string;
}

// ==================== 文件附件相关 ====================

export interface FileAttachmentDto {
  file_id: string;
  filename: string;
  size: number;
  mime_type?: string | null;
  channel: string;
  message_id?: string | null;
  uploaded_by?: string | null;
  stored_at: string;
  ref_count: number;
}

// ==================== 配置相关 ====================

export interface RuntimeConfig {
  provider?: string;
  model?: string;
  api_base?: string;
  api_key?: string;
  temperature?: number;
  max_tokens?: number;
  [key: string]: unknown;
}

export interface ToolsConfigShape {
  sandbox_enabled?: boolean;
  command_rules?: CommandRule[];
  [key: string]: unknown;
}

export interface CommandRule {
  command: string;
  enabled: boolean;
  approval_required: boolean;
}

export interface GatewayProcessStatus {
  running: boolean;
  pid?: number | null;
  executable_path?: string | null;
  details?: string | null;
}

// ==================== Token 统计相关 ====================

export interface TokenStatsSnapshot {
  session_id: string;
  total_tokens: number;
  prompt_tokens: number;
  completion_tokens: number;
  cached_tokens: number;
  updated_at: string;
}

// ==================== Checklist 相关 ====================

export interface ChecklistItem {
  id: string;
  title: string;
  completed: boolean;
}

// ==================== Notebook 相关 ====================

export type ReportPeriod = 'daily' | 'weekly' | 'monthly';

export interface NotebookReport {
  id: string;
  period: ReportPeriod;
  date: string;
  title: string;
  summary: string;
  content: string;
  generatedAt?: string | null;
  generatedBy?: string | null;
  schemaVersion?: string | null;
  generationMode?: string | null;
  coverageStatus?: string | null;
  sourcePath?: string;
  isTruncated?: boolean;
  originalLineCount?: number;
  displayedLineCount?: number;
}

export interface SessionSearchHit {
  session_id: string;
  timestamp: string;
  snippet: string;
  source_uri: string;
  hash: string;
  source: 'session';
  snippet_truncated: boolean;
  message_index: number;
}

export interface SessionSearchResponse {
  hits: SessionSearchHit[];
  diagnostics: unknown[];
  scanned_files: number;
  skipped_files: number;
  total_response_bytes: number;
  file_limit_reached: boolean;
  result_limit_reached: boolean;
  byte_limit_reached: boolean;
}

// ==================== Persona Memory 相关 ====================

export type PersonaKind = 'identity' | 'relationship' | 'redline' | 'user' | 'world' | 'dream' | 'dark';

export interface PersonaDocument {
  kind: PersonaKind;
  content: string;
  revision: number;
  updated_at: string;
}

export interface PersonaChangeRequest {
  id: string;
  kind: PersonaKind;
  proposed_content: string;
  reason: string;
  state: 'pending' | 'accepted' | 'rejected';
  created_at: string;
}

export interface PersonaHistoryEntry {
  revision: number;
  updated_at: string;
  content_hash: string;
}

export interface PersonaHistoryRevision {
  revision: number;
  content: string;
  updated_at: string;
}

// ==================== Cron Task 相关 ====================

export type ScheduleKind = 'at' | 'every' | 'cron';
export type CronStatus = 'running' | 'scheduled' | 'paused' | 'completed' | 'failed';

export interface CronSchedule {
  kind: ScheduleKind;
  atMs?: number;
  everyMs?: number;
  expr?: string;
  tz?: string | null;
}

export interface CronPayload {
  kind: string;
  message: string;
  deliver: boolean;
  channel?: string | null;
  to?: string | null;
}

export interface CronRunSnapshot {
  run_id: string;
  job_id: string;
  startedAtMs: number;
  lastHeartbeatAtMs: number;
  trigger: 'scheduled' | 'manual';
  cancelable: boolean;
}

export interface CronJobDto {
  id: string;
  name: string;
  enabled: boolean;
  schedule: CronSchedule;
  payload: CronPayload;
  state: {
    nextRunAtMs?: number | null;
    lastRunAtMs?: number | null;
    lastStatus?: string | null;
    lastError?: string | null;
  };
  createdAtMs: number;
  updatedAtMs: number;
  deleteAfterRun: boolean;
  isRunning: boolean;
  activeRun?: CronRunSnapshot | null;
  computedStatus: CronStatus;
}

// ==================== Plan Sidebar 相关 ====================

export interface PlanSidebarData {
  plan: PlanRuntimeState | null;
  todos: PlanRuntimeTodo[];
  validation_issues: string[];
}

// ==================== Restored local demo surfaces ====================

/** 单个模型独立保存的演示生成参数（vivy.demo.gen-params，绝不传给真实 Provider）。 */
export interface DemoGenParams {
  temperature: number;
  max_tokens: number;
}

export type DemoTokenPeriod = '1d' | '3d' | '1w' | '1m' | '6m' | '1y';

export interface DemoTokenUsageTotal {
  total_input: number;
  total_output: number;
  total_tokens: number;
  total_cache_creation: number;
  total_cache_read: number;
  request_count: number;
  total_cost: number;
}

export interface DemoTokenUsageGroup {
  key: string;
  total_tokens: number;
  total_cost: number;
  request_count: number;
}

export interface DemoTokenTimelinePoint {
  time_bucket: string;
  label: string;
  total_input: number;
  total_output: number;
  total_tokens: number;
}

export interface DemoTokenSessionUsage {
  id: string;
  title: string;
  model: string;
  request_count: number;
  total_input: number;
  total_output: number;
  total_tokens: number;
  total_cost: number;
}

export interface DemoTokenModelShare {
  model: string;
  percentage: number;
  total_tokens: number;
}

export interface DemoTokenUsageSnapshot {
  period: DemoTokenPeriod;
  total: DemoTokenUsageTotal;
  models: DemoTokenModelShare[];
  endpoints: DemoTokenUsageGroup[];
  timeline: DemoTokenTimelinePoint[];
  sessions: DemoTokenSessionUsage[];
}

export interface DemoMemoryItem {
  id: string;
  title: string;
  category: 'preference' | 'project' | 'decision';
  content: string;
  updatedAt: string;
}

export interface DemoComposerState {
  mode: 'agent' | 'focused';
  secure: boolean;
  recording: boolean;
}
