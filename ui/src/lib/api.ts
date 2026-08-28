import { getRpcClient, RpcClientError, type RpcCapabilities } from './rpc';

export const RPC_METHODS = [
  'initialize', 'capabilities',
  'session/create', 'session/list', 'session/get', 'session/rename', 'session/delete', 'session/messages', 'session/todos', 'session/set_permission',
  'preflight/run', 'turn/start', 'turn/interrupt', 'run/cancel', 'run/get', 'run/subscribe', 'run/unsubscribe', 'run/log',
  'approval/list', 'approval/respond', 'question/list', 'question/respond', 'review/list', 'review/get', 'review/respond',
  'background/recover', 'background/list', 'background/attach',
  'child/start', 'child/get', 'child/list', 'child/wait', 'child/cancel',
  'generations/list', 'generations/get', 'generations/create', 'generations/reject',
  'evals/list', 'evals/record', 'evals/start', 'promotions/list', 'promotions/promote', 'species/inspect',
  'settings/get', 'settings/update',
  'settings/providers', 'settings/providers/upsert', 'settings/providers/delete',
  'stats/tokens',
  'skills/list', 'skills/get',
] as const;

export type RunStatus = 'accepted' | 'queued' | 'active' | 'completed' | 'failed' | 'cancelled';
export type RunMode = 'normal' | 'plan';
export type TodoStatus = 'pending' | 'in_progress' | 'completed' | 'cancelled';
export type PermissionPreset = 'cautious' | 'smart' | 'trusted' | 'custom';
export type SandboxMode = 'read_only' | 'workspace_write' | 'danger_full_access';
export type ApprovalPolicy = 'ask' | 'never' | 'auto';
export interface Session {
  id: string;
  title: string;
  created_at: number;
  sandbox_mode?: SandboxMode;
  approval_policy?: ApprovalPolicy;
  permission_preset?: PermissionPreset;
}
export interface Todo {
  id: string;
  session_id: string;
  subject: string;
  description: string;
  status: TodoStatus;
  blocks: string[];
  blocked_by: string[];
  active_form?: string;
  owner?: string;
  position: number;
  created_at: number;
  updated_at: number;
}
export interface Message { id: string; run_id?: string; role: 'user' | 'assistant' | 'system' | 'tool'; content: string; created_at: number }
export interface Run { id: string; session_id: string; status: RunStatus; created_at: number }
export interface RunLogEvent { run_id: string; seq: number; type: string; created_at: number; payload_version: number; payload: Record<string, unknown> }
export interface Preflight { status: 'ready' | 'warning' | 'blocked'; mode: RunMode; policy_profile: string; policy_hash?: string; selected_tools: string[]; tool_decisions: Array<{ tool_name: string; decision: string; reason: string }>; context_bytes: number; hook_ready: boolean; warnings: string[]; blockers: string[]; next_actions: string[] }
export interface BackgroundRun extends Run { workspace_id?: string }
export interface ChildRun { id: string; parent_run_id: string; root_run_id: string; session_id: string; status: RunStatus; depth: number; workspace_id?: string; result?: string; error?: string; created_at: number }
export type ReviewKind = 'approval' | 'question';
export type ReviewStatus = 'pending' | 'approved' | 'denied' | 'answered' | 'cancelled' | 'expired' | 'stale';
export interface ReviewItem { id: string; kind: ReviewKind; status: ReviewStatus; session_id: string; session_title?: string; run_id: string; tool_call_id?: string; tool_name?: string; source?: string; actor?: string; created_at: number; expires_at: number; decided_at?: number; action?: string; target?: string; precondition_hash?: string; preview?: string; risk_findings?: string[]; arguments?: Record<string, unknown>; prompt?: string; decision_reason?: string; stale_reason?: string; error?: string; effect?: string; reversibility?: string; scope?: string; trust?: string }
export interface Settings {
  provider: string;
  default_model: string;
  base_url: string;
  read_only: boolean;
  frozen?: boolean;
  config_provider: string;
  config_model: string;
  api_key_set?: boolean;
  network_search?: NetworkSearchSettingsView;
  execute_max_timeout_seconds?: number;
  config_execute_max_timeout_seconds?: number;
  sandbox?: SandboxSettingsView;
}
export interface NetworkSearchProviderInfo {
  name: string;
  keyless: boolean;
  configured: boolean;
  env_key?: string;
}
export interface NetworkSearchSettingsView {
  provider: string;
  config_provider: string;
  providers: NetworkSearchProviderInfo[];
}
/** settings/update 载荷：选模型不发送 api_key（注册表条目保留密钥）。 */
export interface SandboxSettingsView {
  default_preset: PermissionPreset;
  config_default_preset: PermissionPreset;
  deny_private_ips: boolean;
  allowed_domains: string[];
  workspace_root?: string;
  execute_allowed_commands?: string[];
}
export type SettingsUpdate = Pick<Settings, 'provider' | 'default_model' | 'base_url'> & {
  api_key?: string;
  network_search?: { provider: string };
  execute_max_timeout_seconds?: number;
  sandbox?: {
    default_preset: Exclude<PermissionPreset, 'custom'>;
    deny_private_ips: boolean;
    allowed_domains: string[];
  };
};

/** settings/update 是整文档替换：未发送的分区会被清掉，调用方必须带上未改动的 overlay。 */
export function settingsUpdateFrom(settings: Settings | null, patch: Partial<SettingsUpdate> = {}): SettingsUpdate {
  const sandbox = patch.sandbox ?? (settings?.sandbox && settings.sandbox.default_preset !== 'custom'
    ? {
      default_preset: settings.sandbox.default_preset,
      deny_private_ips: settings.sandbox.deny_private_ips,
      allowed_domains: settings.sandbox.allowed_domains ?? [],
    }
    : undefined);
  return {
    provider: patch.provider ?? settings?.provider ?? '',
    default_model: patch.default_model ?? settings?.default_model ?? '',
    base_url: patch.base_url ?? settings?.base_url ?? '',
    network_search: patch.network_search ?? { provider: settings?.network_search?.provider ?? '' },
    execute_max_timeout_seconds: patch.execute_max_timeout_seconds ?? settings?.execute_max_timeout_seconds ?? 0,
    ...(sandbox ? { sandbox } : {}),
  };
}
export interface SpeciesInspect { protocol_version: string; binary_id: string; generation_id: string; artifact_sha256?: string; recipe: Recipe; policy_profile: string; policy_hash: string; tools: Array<{ name: string; readonly: boolean }>; grants: string[] }
export interface Recipe { loop?: string; world?: string; providers?: string[]; tools?: string[]; plugins?: string[] }
export type GenerationPhase = 'built' | 'eval_pending' | 'evaluated' | 'promoted' | 'released' | 'rejected';
export interface Generation { id: string; parent_id?: string; artifact_sha256: string; source_ref?: string; recipe: Recipe; phase: GenerationPhase; created_at: number }
export type EvalVerdict = 'better' | 'worse' | 'mixed' | 'failed_to_run';
export interface EvalRun { id: string; candidate_id: string; baseline_id?: string; suite: string; verdict: EvalVerdict; journal_ref?: string; created_at: number }
export interface Promotion { id: string; from_id: string; to_id: string; eval_id?: string; actor: string; phase: string; applies_at: string; created_at: number }
export interface Approval { id: string; run_id: string; tool_call_id: string; decision?: string; expires_at: number }
export interface Question { id: string; run_id: string; tool_call_id: string; prompt: string; status: string; expires_at: number }

export class ApiError extends Error {
  constructor(public readonly status: number, public readonly code: string, message: string) { super(message); this.name = 'ApiError'; }
}

function mapCode(code: number) { return code === -32004 || code === -32601 ? [404, 'not_found'] as const : code === -32009 ? [409, 'conflict'] as const : code === -32602 ? [400, 'invalid_request'] as const : [500, 'internal_error'] as const; }
export async function request<T>(method: string, params?: unknown): Promise<T> {
  try { return await (await getRpcClient()).call<T>(method, params); }
  catch (error) {
    if (error instanceof RpcClientError) { const [status, code] = mapCode(error.code); throw new ApiError(status, code, error.message); }
    throw new ApiError(0, 'network', error instanceof Error ? error.message : String(error));
  }
}

export const initialize = async (): Promise<RpcCapabilities> => (await getRpcClient()).capabilities;
export const listSessions = () => request<{ sessions: Session[] }>('session/list');
export const getSession = (id: string) => request<{ session: Session; messages: Message[] }>('session/get', { session_id: id });
export const createSession = (title: string) => request<Session>('session/create', { title });
export const renameSession = (id: string, title: string) => request<Session>('session/rename', { session_id: id, title });
export const setSessionPermission = (id: string, preset: Exclude<PermissionPreset, 'custom'>) => request<Session>('session/set_permission', { session_id: id, preset });
export const deleteSession = (id: string) => request<unknown>('session/delete', { session_id: id }).then(() => undefined);
export const listMessages = (sessionId: string) => request<{ messages: Message[] }>('session/messages', { session_id: sessionId });
export const listTodos = (sessionId: string) => request<{ todos: Todo[] }>('session/todos', { session_id: sessionId });
export const preflight = (sessionId: string, text: string, mode: RunMode) => request<Preflight>('preflight/run', { session_id: sessionId, text, mode });
export const startTurn = (sessionId: string, text: string, mode: RunMode = 'normal') => request<{ run_id: string; status: RunStatus }>('turn/start', { session_id: sessionId, text, mode });
export const interruptRun = (runId: string) => request<{ run_id: string; status: string }>('turn/interrupt', { run_id: runId });
export const cancelRun = (runId: string) => request<{ run_id: string; status: string }>('run/cancel', { run_id: runId });
export const getRun = (runId: string) => request<Run>('run/get', { run_id: runId });
export const getRunLog = (runId: string, afterSeq = 0) => request<{ events: RunLogEvent[] }>('run/log', { run_id: runId, after_seq: afterSeq });
export const recoverBackgroundRuns = () => request<{ recovered: boolean }>('background/recover');
export const listBackgroundRuns = () => request<{ runs: BackgroundRun[] }>('background/list');
export const attachBackgroundRun = (runId: string) => request<BackgroundRun>('background/attach', { run_id: runId });
export const startChild = (params: { parent_run_id: string; text: string; policy_profile?: string; tool_names?: string[] }) => request<ChildRun>('child/start', params);
export const getChild = (runId: string) => request<ChildRun>('child/get', { run_id: runId });
export const listChildren = (parentRunId: string, tree = true) => request<{ children: ChildRun[] }>('child/list', { parent_run_id: parentRunId, tree });
export const waitChild = (runId: string) => request<ChildRun>('child/wait', { run_id: runId });
export const cancelChild = (runId: string) => request<ChildRun>('child/cancel', { run_id: runId });
export const listApprovals = () => request<{ approvals: Approval[] }>('approval/list');
export const respondApproval = (approvalId: string, decision: 'approved' | 'denied', reason?: string) => request('approval/respond', { approval_id: approvalId, decision, reason });
export const listQuestions = () => request<{ questions: Question[] }>('question/list');
export const respondQuestion = (questionId: string, answer: string) => request('question/respond', { question_id: questionId, answer });
export const listReviews = (params: { kind?: ReviewKind; status?: ReviewStatus; session_id?: string; limit?: number } = {}) => request<{ reviews: ReviewItem[] }>('review/list', params);
export const getReview = (reviewId: string) => request<ReviewItem>('review/get', { review_id: reviewId });
export const respondReview = (reviewId: string, response: { action: 'approve' | 'deny' | 'answer' | 'cancel'; reason?: string; answer?: string }) => request<{ review_id: string; status: string }>('review/respond', { review_id: reviewId, ...response });
export const inspectSpecies = () => request<SpeciesInspect>('species/inspect');
export const listGenerations = () => request<{ generations: Generation[] }>('generations/list');
export const getGeneration = (id: string) => request<Generation>('generations/get', { id });
export const createGeneration = (params: { id?: string; parent_id?: string; artifact_sha256: string; source_ref?: string; recipe: Recipe }) => request<Generation>('generations/create', params);
export const rejectGeneration = (id: string) => request<Generation>('generations/reject', { id });
export const listEvals = () => request<{ evals: EvalRun[] }>('evals/list');
export const recordEval = (params: { candidate_id: string; baseline_id?: string; suite: string; verdict: EvalVerdict; journal_ref?: string }) => request<EvalRun>('evals/record', params);
export const startEval = (params: { candidate_id: string; baseline_id?: string; suite: string }) => request<EvalRun>('evals/start', params);
export const listPromotions = () => request<{ promotions: Promotion[] }>('promotions/list');
export const promoteGeneration = (params: { from_id: string; to_id: string; eval_id?: string; actor?: string }) => request<Promotion>('promotions/promote', params);
export const getSettings = () => request<Settings>('settings/get');
export const updateSettings = (params: SettingsUpdate) => {
  const payload: Record<string, unknown> = { ...params };
  if (params.api_key === undefined) delete payload.api_key;
  return request<Settings>('settings/update', payload);
};

/** 注册表供应商（wire 形态）：密钥永不在线，只回 api_key_set。 */
export interface ProviderEntry {
  id: string;
  display_name: string;
  bundle: 'openai' | 'anthropic';
  base_url: string;
  default_model: string;
  models: string[];
  api_key_set: boolean;
}

/** settings/providers/upsert 载荷：api_key 写-only（空串=清除该条目密钥）。 */
export interface ProviderEntryInput {
  id?: string;
  display_name: string;
  bundle: 'openai' | 'anthropic';
  base_url: string;
  default_model: string;
  models: string[];
  api_key?: string;
}

export interface ProvidersView {
  entries: ProviderEntry[];
  active_provider: string;
  active_model: string;
  active_base_url: string;
  read_only: boolean;
  frozen?: boolean;
  config_provider: string;
  config_model: string;
}

export const listProviders = () => request<ProvidersView>('settings/providers');
export const upsertProvider = (input: ProviderEntryInput) => request<ProviderEntry>('settings/providers/upsert', { ...input });
export const deleteProvider = (id: string) => request<{ deleted: boolean; id: string }>('settings/providers/delete', { id });

// ==================== Token Usage Stats ====================

export type TokenUsagePeriod = '1d' | '3d' | '1w' | '1m' | '6m' | '1y';

export interface TokenUsageTotal {
  total_input: number;
  total_output: number;
  total_tokens: number;
  total_reasoning: number;
  request_count: number;
}

export interface TokenModelShare {
  model: string;
  percentage: number;
  total_tokens: number;
}

export interface TokenProviderGroup {
  key: string;
  total_tokens: number;
  request_count: number;
}

export interface TokenTimelinePoint {
  time_bucket: string;
  label: string;
  total_input: number;
  total_output: number;
  total_tokens: number;
}

export interface TokenSessionUsage {
  id: string;
  title: string;
  model: string;
  request_count: number;
  total_input: number;
  total_output: number;
  total_tokens: number;
}

export interface TokenUsageSnapshot {
  period: TokenUsagePeriod;
  total: TokenUsageTotal;
  models: TokenModelShare[];
  providers: TokenProviderGroup[];
  timeline: TokenTimelinePoint[];
  sessions: TokenSessionUsage[];
}

export interface TokenUsageParams {
  period: TokenUsagePeriod;
  tz_offset_minutes: number;
  session_limit?: number;
}

export const getTokenUsage = (params: TokenUsageParams) =>
  request<TokenUsageSnapshot>('stats/tokens', params);

export interface SkillSummary {
  name: string;
  description: string;
  context?: string;
  agent?: string;
  model?: string;
  hash: string;
  warnings: string[];
}

export interface SkillView extends SkillSummary {
  content: string;
  relative_path: string;
  supporting_files: string[];
}

export const listSkills = () => request<{ skills: SkillSummary[] }>('skills/list');
export const getSkill = (name: string, path?: string) =>
  request<SkillView>('skills/get', path ? { name, path } : { name });
