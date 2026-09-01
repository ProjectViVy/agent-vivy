import { getRpcClient, RpcClientError, type RpcCapabilities } from './rpc';

export const RPC_METHODS = [
  'initialize', 'capabilities',
  'session/create', 'session/list', 'session/get', 'session/rename', 'session/delete', 'session/messages', 'session/todos', 'session/set_permission',
  'session/context', 'context/compact',
  'turn/start', 'turn/interrupt', 'run/cancel', 'run/get', 'run/subscribe', 'run/unsubscribe', 'run/log',
  'approval/list', 'approval/respond', 'question/list', 'question/respond', 'review/list', 'review/get', 'review/respond',
  'background/recover', 'background/list', 'background/attach',
  'child/start', 'child/get', 'child/list', 'child/wait', 'child/cancel',
  'generations/list', 'generations/get', 'generations/create', 'generations/reject',
  'evals/list', 'evals/record', 'evals/start', 'promotions/list', 'promotions/promote', 'species/inspect',
  'settings/get', 'settings/update',
  'settings/providers', 'settings/providers/upsert', 'settings/providers/delete', 'settings/providers/refresh',
  'settings/mcp', 'settings/mcp/upsert', 'settings/mcp/delete', 'settings/mcp/probe',
  'tools/list', 'tools/set-active',
  'channel/inspect', 'channel/get', 'channel/update',
  'cron/list', 'cron/create', 'cron/update', 'cron/delete', 'cron/trigger', 'cron/stop',
  'stats/tokens',
  'skills/list', 'skills/get', 'skills/set-enabled',
  'skills/marketplace/search', 'skills/marketplace/featured', 'skills/marketplace/install',
] as const;

export type RunStatus = 'accepted' | 'queued' | 'active' | 'completed' | 'failed' | 'cancelled';
export type RunMode = 'normal' | 'plan';
/** Entry assembly serving the run; omitted means 'web'. */
export type Face = 'web' | 'tui' | 'code';
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
/** 消息世界入口出处（CH-C1）：仅 channel 轮携带，ui 轮不出该字段。 */
export interface MessageProvenance { source: string; channel?: string; chat_id?: string; channel_message_id?: string }
export interface Message { id: string; run_id?: string; role: 'user' | 'assistant' | 'system' | 'tool'; content: string; created_at: number; attachments?: MessageAttachment[]; provenance?: MessageProvenance }
/** turn/start 附件输入：data 为原始 base64（不带 data: 前缀），服务端做类型/大小校验。 */
export interface AttachmentInput { name?: string; mime_type: string; data: string }
/** session/messages 返回的用户消息附件：data_url 为服务端拼好的 data URL。 */
export interface MessageAttachment { name?: string; mime_type: string; data_url: string }
export interface Run { id: string; session_id: string; status: RunStatus; created_at: number }
export interface RunLogEvent { run_id: string; seq: number; type: string; created_at: number; payload_version: number; payload: Record<string, unknown> }
/** session/context — 真实上下文压力（服务端装配口径）。 */
export interface SessionContext {
  session_id: string;
  total_messages: number;
  feed_messages: number;
  feed_bytes: number;
  feed_tokens: number;
  limit_bytes: number;
  model_limit_tokens: number;
  compaction_enabled: boolean;
  trigger_tokens: number;
  would_compact: boolean;
  has_compaction_summary: boolean;
  last_compaction?: { mode: 'reduction' | 'summarization' | 'session'; before_tokens: number; after_tokens: number; at: number } | null;
}
/** context/compact 结果。 */
export interface CompactResult { before_tokens: number; after_tokens: number; folded_messages: number; skipped: boolean }
export interface BackgroundRun extends Run { workspace_id?: string }
export interface ChildRun { id: string; parent_run_id: string; root_run_id: string; session_id: string; status: RunStatus; depth: number; workspace_id?: string; result?: string; error?: string; created_at: number }
export type ReviewKind = 'approval' | 'question';
export type ReviewStatus = 'pending' | 'approved' | 'denied' | 'answered' | 'cancelled' | 'expired' | 'stale';
export interface ReviewItem { id: string; kind: ReviewKind; status: ReviewStatus; session_id: string; session_title?: string; run_id: string; tool_call_id?: string; tool_name?: string; source?: string; actor?: string; created_at: number; expires_at: number; decided_at?: number; action?: string; target?: string; precondition_hash?: string; preview?: string; risk_findings?: string[]; arguments?: Record<string, unknown>; prompt?: string; decision_reason?: string; stale_reason?: string; error?: string; effect?: string; reversibility?: string; scope?: string; trust?: string }
export interface WorkspaceFile { path: string; size: number }
export interface WorkspaceFileContent { path: string; content: string; size: number; truncated: boolean; binary: boolean }
export const listWorkspaceFiles = (runId: string) => request<{ files: WorkspaceFile[]; truncated: boolean }>('workspace/list', { run_id: runId });
export const readWorkspaceFile = (runId: string, path: string) => request<WorkspaceFileContent>('workspace/read', { run_id: runId, path });
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
  compaction?: CompactionSettingsView;
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
/** settings/get compaction 段：有效值 + 配置回退。 */
export interface CompactionSettingsView {
  enabled: boolean;
  max_tokens: number;
  trigger_percent: number;
  keep_recent: number;
  config_enabled: boolean;
  config_max_tokens: number;
  config_trigger_percent: number;
  config_keep_recent: number;
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
  compaction?: {
    enabled: boolean;
    max_tokens: number;
    trigger_percent: number;
    keep_recent: number;
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
  // compaction 段同样随整文档保存（避免其他分区保存时把压缩设置清掉）；
  // 只带有效字段，防 config_* 回退键进入写载荷。
  const compaction = patch.compaction ?? (settings?.compaction
    ? {
      enabled: settings.compaction.enabled,
      max_tokens: settings.compaction.max_tokens,
      trigger_percent: settings.compaction.trigger_percent,
      keep_recent: settings.compaction.keep_recent,
    }
    : undefined);
  return {
    provider: patch.provider ?? settings?.provider ?? '',
    default_model: patch.default_model ?? settings?.default_model ?? '',
    base_url: patch.base_url ?? settings?.base_url ?? '',
    network_search: patch.network_search ?? { provider: settings?.network_search?.provider ?? '' },
    execute_max_timeout_seconds: patch.execute_max_timeout_seconds ?? settings?.execute_max_timeout_seconds ?? 0,
    ...(sandbox ? { sandbox } : {}),
    ...(compaction ? { compaction } : {}),
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

function mapCode(code: number) { return code === -32004 || code === -32601 ? [404, 'not_found'] as const : code === -32009 ? [409, 'conflict'] as const : code === -32602 ? [400, 'invalid_request'] as const : code === -32010 ? [502, 'bad_gateway'] as const : [500, 'internal_error'] as const; }
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
export const getSessionContext = (sessionId: string) => request<SessionContext>('session/context', { session_id: sessionId });
export const compactSession = (sessionId: string) => request<CompactResult>('context/compact', { session_id: sessionId });
export const listTodos = (sessionId: string) => request<{ todos: Todo[] }>('session/todos', { session_id: sessionId });
export const startTurn = (sessionId: string, text: string, mode: RunMode = 'normal', face?: Face, attachments?: AttachmentInput[]) => request<{ run_id: string; status: RunStatus }>('turn/start', { session_id: sessionId, text, mode, face, attachments });
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

/** tools/list 目录项：内置注册表全量（active + hidden）。 */
export interface ToolCatalogEntry { name: string; description: string; readonly: boolean; active: boolean }
/** tools/list 视图：active 为生效激活集，config_enabled 为配置默认回退。 */
export interface ToolsCatalogView { tools: ToolCatalogEntry[]; active: string[]; config_enabled: string[]; overlay_written: boolean }

export const listTools = () => request<ToolsCatalogView>('tools/list');
/** 整表替换 settings.yaml 的 tools_enabled 覆盖层；空表 = 纯对话模式。 */
export const setActiveTools = (tools: string[]) => request<ToolsCatalogView>('tools/set-active', { tools });
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

/** settings/providers/refresh 载荷：按 id 或 (bundle, base_url) 定位条目；目录厂商无注册表行时克隆成自定义条目以持久化。密钥不参与请求（后端按注册表解析）。 */
export interface ProviderRefreshInput {
  id?: string;
  bundle?: 'openai';
  base_url?: string;
  display_name?: string;
  default_model?: string;
}

/** 从上游 GET /models 同步模型列表并持久化到注册表；返回脱敏后的保存条目。 */
export const refreshProviderModels = (input: ProviderRefreshInput) => request<ProviderEntry>('settings/providers/refresh', input);

export type McpStatus = 'idle' | 'ok' | 'error';
export interface McpServer {
  name: string;
  endpoint: string;
  auth_env?: string;
  auth_env_set: boolean;
  enabled: boolean;
  tool_count: number;
  status: McpStatus;
  error?: string;
}
export interface McpServerInput {
  name: string;
  endpoint: string;
  auth_env?: string;
  enabled?: boolean;
}
export interface McpServersView {
  servers: McpServer[];
  read_only: boolean;
}
export const listMcpServers = () => request<McpServersView>('settings/mcp');
export const upsertMcpServer = (input: McpServerInput) => request<McpServer>('settings/mcp/upsert', input);
export const deleteMcpServer = (name: string) => request<{ deleted: boolean; name: string }>('settings/mcp/delete', { name });
export const probeMcpServer = (name: string) => request<McpServer>('settings/mcp/probe', { name });

// ==================== Channels (ears) ====================

/** 通道可选 ABI 能力位（后端 channelhost.Discover 的 wire 形态）。 */
export interface ChannelCapabilities {
  typing: boolean;
  edit: boolean;
  delete: boolean;
  reaction: boolean;
  placeholder: boolean;
  media: boolean;
  media_store: boolean;
  webhook: boolean;
  listen: boolean;
  stream: boolean;
  health: boolean;
}

/**
 * channel/inspect 条目：进程真值（上次 StartAll 的决策）。
 * 通道写入在进程重启后生效，UI 用它与 channel/get 对比得出"待重启"。
 */
export interface ChannelStatus {
  name: string;
  capabilities: ChannelCapabilities;
  configured: boolean;
  enabled: boolean;
  /** 启动生效 envelope 的 allow_from 摘要（进程真值），恒为数组；发送者
   *  ID 不是密钥（D-010）。与 channel/get 的文档真值对比可发现纯
   *  allow_from 编辑（待重启）。 */
  allow_from: string[];
  started: boolean;
  /** 仅环境变量名（D-010）；密钥值永不在线。 */
  token_env: string;
  token_env_set: boolean;
  /** 启动跳过/失败原因；空串 = 已启动（或尚未启动过）。 */
  note: string;
}

/**
 * channel/get 结果：文档真值 = config.yaml envelope ⊕ settings overlay。
 * allow_from 恒为数组；空数组 = 拒绝启动（fail-closed）。
 */
export interface ChannelEnvelope {
  name: string;
  enabled: boolean;
  allow_from: string[];
  token_env: string;
  configured: boolean;
}

/**
 * channel/update 载荷：指针语义，未携带的字段回落 config.yaml 值。
 * token_env 只接受环境变量名；密钥值永不发送（D-010）。
 */
export interface ChannelUpdateInput {
  enabled?: boolean;
  allow_from?: string[];
  token_env?: string;
}

/** 编译进当前代的全部通道（无论是否配置），按名称排序。 */
export const inspectChannels = () => request<ChannelStatus[]>('channel/inspect');
export const getChannel = (name: string) => request<ChannelEnvelope>('channel/get', { name });
export const updateChannel = (name: string, patch: ChannelUpdateInput) =>
  request<ChannelEnvelope>('channel/update', { name, ...patch });

// ==================== Token Usage Stats ====================

export type TokenUsagePeriod = '1d' | '3d' | '1w' | '1m' | '6m' | '1y';

export interface TokenUsageTotal {
  total_input: number;
  total_output: number;
  total_tokens: number;
  total_reasoning: number;
  total_cached: number;
  request_count: number;
  total_cost_usd: number;
  cost_known: boolean;
}

export interface TokenModelShare {
  model: string;
  percentage: number;
  total_tokens: number;
  cost_usd: number;
  cost_known: boolean;
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
  cost_usd: number;
  cost_known: boolean;
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
  enabled: boolean;
  hash: string;
  warnings: string[];
}

export interface SkillView extends SkillSummary {
  content: string;
  relative_path: string;
  supporting_files: string[];
}

/** skills/marketplace/* — skills.sh 目录条目；id 形如 owner/repo/slug。 */
export interface MarketplaceSkill { id: string; name: string; source: string; installs: number }
export interface MarketplaceFeatured { generated_at: string; source: string; metric: string; skills: MarketplaceSkill[] }
export interface MarketplaceInstallResult { skill: SkillView; skipped_files?: string[]; warnings?: string[] }

export const listSkills = () => request<{ skills: SkillSummary[] }>('skills/list');
export const getSkill = (name: string, path?: string) =>
  request<SkillView>('skills/get', path ? { name, path } : { name });
export const setSkillEnabled = (name: string, enabled: boolean, base_hash: string) =>
  request<SkillSummary>('skills/set-enabled', { name, enabled, base_hash });
export const searchMarketplaceSkills = (q: string, limit?: number) =>
  request<{ skills: MarketplaceSkill[] }>('skills/marketplace/search', limit ? { q, limit } : { q });
export const featuredMarketplaceSkills = () => request<MarketplaceFeatured>('skills/marketplace/featured');
export const installMarketplaceSkill = (id: string) =>
  request<MarketplaceInstallResult>('skills/marketplace/install', { id });

// ==================== Cron（定时任务，后端真实 RPC） ====================

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
  /** 任务专属会话（首次触发时由后端创建并绑定）。 */
  sessionId?: string | null;
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

/** cron/create、cron/update 的写入载荷（整对象语义）。 */
export interface CronJobInput {
  name: string;
  enabled: boolean;
  schedule: CronSchedule;
  payload: CronPayload;
  delete_after_run?: boolean;
}

export const listCronJobs = () => request<{ jobs: CronJobDto[] }>('cron/list');
export const createCronJob = (input: CronJobInput) => request<{ job: CronJobDto }>('cron/create', input);
export const updateCronJob = (id: string, input: CronJobInput) => request<{ job: CronJobDto }>('cron/update', { id, ...input });
export const deleteCronJob = (id: string) => request<{ deleted: boolean }>('cron/delete', { id });
export const triggerCronJob = (id: string) => request<{ job: CronJobDto }>('cron/trigger', { id });
export const stopCronJob = (id: string) => request<{ stopped: boolean }>('cron/stop', { id });
