/**
 * 轨迹面板的展示层类型（面板 / 工具栏 / 时间轴 / 账本 / 详情共用）。
 * 字段语义沿用 DeepSeek Harness `packages/client/ui-trajectory` 的
 * `trajectory-record.ts` 子集；真实数据由 `trajectory-session.ts` 从
 * `trajectory/session` RPC 的 snake_case wire 形态映射而来。
 */

/** 轨迹记录类型的闭合集合（与 DSH TrajectoryCellKind 一致）。 */
export type TrajectoryCellKind =
  | 'system'
  | 'user'
  | 'context'
  | 'compacted'
  | 'message'
  | 'tool'
  | 'subtool';

/** 类型徽标文案（与 DSH KIND_LABEL 一致，不做本地化）。 */
export const TRAJECTORY_KIND_LABEL: Record<TrajectoryCellKind, string> = {
  system: 'SYSTEM',
  user: 'USER',
  context: 'CONTEXT',
  compacted: 'COMPACTED',
  message: 'ASSISTANT',
  tool: 'TOOL',
  subtool: 'SUBTOOL',
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

/** 请求级信息（详情面板 Summary/Usage/Timing 的来源）。 */
export interface TrajectoryRequest {
  number: number;
  turn: number | null;
  group: string;
  purpose?: 'compaction';
  status: 'complete' | 'error';
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
}

/** 时间轴投影模式：等宽序列 / 按真实时长（压缩空闲）。 */
export type TrajectoryTimelineMode = 'sequence' | 'duration';

/** 时间轴选区（闭区间，作用于当前投影的域）。 */
export interface TrajectoryTimeRange {
  start: number;
  end: number;
}
