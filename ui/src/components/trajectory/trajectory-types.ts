/**
 * 轨迹面板的展示层类型（面板 / 工具栏 / 时间轴 / 账本 / 详情共用）。
 * 字段语义沿用 DeepSeek Harness `packages/client/ui-trajectory` 的
 * `trajectory-record.ts` 子集；真实数据由 `trajectory-session.ts` 从
 * `trajectory/session` RPC 的 snake_case wire 形态映射而来。
 */

import { t } from '@/i18n';

/** 轨迹记录类型的闭合集合（与 DSH TrajectoryCellKind 一致）。 */
export type TrajectoryCellKind =
  | 'system'
  | 'user'
  | 'context'
  | 'compacted'
  | 'message'
  | 'tool'
  | 'subtool';

/** Host-owned badge labels; record kind values remain protocol identifiers. */
export const TRAJECTORY_KIND_LABEL: Record<TrajectoryCellKind, string> = {
  get system() { return t('trajectory.kinds.system'); },
  get user() { return t('trajectory.kinds.user'); },
  get context() { return t('trajectory.kinds.context'); },
  get compacted() { return t('trajectory.kinds.compacted'); },
  get message() { return t('trajectory.kinds.message'); },
  get tool() { return t('trajectory.kinds.tool'); },
  get subtool() { return t('trajectory.kinds.subtool'); },
};

/** Token 用量明细。 */
export interface TrajectoryTokens {
  input?: number;
  output?: number;
  think?: number;
  cacheRead?: number;
  cacheWrite?: number;
}

/** 一条轨迹记录（账本行 + 时间轴条带 + 详情面板的共同数据源）。 */
export interface TrajectoryRecord {
  /** 全局唯一记录索引（1 起）。 */
  index: number;
  /** 稳定身份，供折叠与选中状态引用。 */
  id: string;
  /** 所属回合：null 表示会话起始区段。 */
  turn: number | null;
  /** 步骤分组名，如 `Step 1`；会话区段为 `Session`。 */
  group: string;
  kind: TrajectoryCellKind;
  /** 账本行主文案。 */
  text: string;
  /** 自身耗时（秒），未知为 null。 */
  timeSeconds: number | null;
  /** 操作实际开始的 Unix 毫秒，未知为 null。 */
  startedAt: number | null;
  tokens?: TrajectoryTokens;
  /** 工具/助理行的内联结果摘要；失败时结合 isError 渲染为红色。 */
  result?: string;
  isError?: boolean;
  /** 助理行：首 Token 耗时（毫秒），用于时间轴 TTFT 分段。 */
  ttftMs?: number;
  /** 详情面板：请求入参 / 完整输入。 */
  inputDetail?: string;
  /** 详情面板：完整输出 / 工具结果 / 系统提示正文。 */
  outputDetail?: string;
  /** 详情面板：完整思考内容。 */
  thinkingDetail?: string;
  callId?: string;
  provider?: string;
  model?: string;
  /** 该行是否开启一个新的模型回合。 */
  opensTurn?: boolean;
}

// OBS-04 (D4) 调用生命周期词表（展示层，与 api.ts wire 类型对齐）。
export type TrajectoryCallStatus = 'active' | 'completed' | 'failed' | 'cancelled' | 'interrupted' | 'legacy';
export type TrajectoryUsageState = 'missing' | 'reported' | 'partial' | 'active' | 'legacy';
export type TrajectoryActivityState = 'queued' | 'active' | 'waiting' | 'completed' | 'failed' | 'cancelled';
export type TrajectoryWaitKind = 'approval' | 'question' | 'child' | 'workflow';

export const TRAJECTORY_CALL_STATUS_LABEL: Record<TrajectoryCallStatus, string> = {
  get active() { return t('trajectory.callStatus.active'); },
  get completed() { return t('trajectory.callStatus.completed'); },
  get failed() { return t('trajectory.callStatus.failed'); },
  get cancelled() { return t('trajectory.callStatus.cancelled'); },
  get interrupted() { return t('trajectory.callStatus.interrupted'); },
  get legacy() { return t('trajectory.callStatus.legacy'); },
};

export const TRAJECTORY_USAGE_STATE_LABEL: Record<TrajectoryUsageState, string> = {
  get missing() { return t('trajectory.usageState.missing'); },
  get reported() { return t('trajectory.usageState.reported'); },
  get partial() { return t('trajectory.usageState.partial'); },
  get active() { return t('trajectory.usageState.active'); },
  get legacy() { return t('trajectory.usageState.legacy'); },
};

export const TRAJECTORY_ACTIVITY_STATE_LABEL: Record<TrajectoryActivityState, string> = {
  get queued() { return t('trajectory.activityState.queued'); },
  get active() { return t('trajectory.activityState.active'); },
  get waiting() { return t('trajectory.activityState.waiting'); },
  get completed() { return t('trajectory.activityState.completed'); },
  get failed() { return t('trajectory.activityState.failed'); },
  get cancelled() { return t('trajectory.activityState.cancelled'); },
};

export const TRAJECTORY_WAIT_KIND_LABEL: Record<TrajectoryWaitKind, string> = {
  get approval() { return t('trajectory.waitKind.approval'); },
  get question() { return t('trajectory.waitKind.question'); },
  get child() { return t('trajectory.waitKind.child'); },
  get workflow() { return t('trajectory.waitKind.workflow'); },
};

/** Call 级用量证据（D4 usage_evidence）。 */
export interface TrajectoryUsageEvidence {
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  reasoningTokens: number | null;
  cachedTokens: number | null;
  partial: boolean;
}

/** 请求级信息（详情面板 Summary/Usage/Timing 的来源）。 */
export interface TrajectoryRequest {
  number: number;
  turn: number | null;
  group: string;
  purpose?: 'compaction';
  /** legacy status mirror；活跃调用可能是 'active'/'cancelled'。 */
  status: 'complete' | 'error' | 'active' | 'cancelled';
  startedAt: number;
  completedAt: number;
  provider?: string;
  model?: string;
  requestConfig?: { provider: string; model: string };
  usage: Required<TrajectoryTokens>;
  error?: string;
  retry?: number;
  maxRetries?: number;
  retryDelayMs?: number;
  /** 请求消息条数与前置装配字节数（真实 RPC 携带）。 */
  messages?: number;
  preambleBytes?: number;
  // OBS-04 v2 字段：
  /** 稳定请求身份 `run_id:call_id`（legacy 行为 `run_id:request_start_seq`）。 */
  requestId?: string;
  runId?: string;
  callId?: string;
  callStatus?: TrajectoryCallStatus;
  finishedAt?: number | null;
  usageState?: TrajectoryUsageState;
  usageEvidence?: TrajectoryUsageEvidence | null;
}

/** Run 级活动状态（D4 run_activity）。 */
export interface TrajectoryRunActivity {
  runId: string;
  status: string;
  activityState: TrajectoryActivityState;
  waitKind?: TrajectoryWaitKind;
  parentRunId?: string;
  childRunIds: string[];
  workflowId?: string;
}

/** 时间轴投影模式：等宽序列 / 按真实时长（压缩空闲）。 */
export type TrajectoryTimelineMode = 'sequence' | 'duration';

/** 时间轴选区（闭区间，作用于当前投影的域）。 */
export interface TrajectoryTimeRange {
  start: number;
  end: number;
}
