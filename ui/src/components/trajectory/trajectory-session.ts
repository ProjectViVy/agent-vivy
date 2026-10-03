/**
 * trajectory/session wire → 展示层映射（UI-TRAJECTORY-DEMO / OBS-04 v2）。
 * snake_case wire 形态（api.ts）映射为 DSH 风格的展示层类型
 * （trajectory-types.ts）；只做字段搬运与默认值填充，不做语义加工。
 */

import {
  fetchSessionTrajectory,
  type TrajectoryRequestWire,
  type TrajectorySessionWire,
  type TrajectoryTokensWire,
  type TrajectoryUsageEvidenceWire,
} from '@/lib/api';
import type {
  TrajectoryRecord,
  TrajectoryRequest,
  TrajectoryRunActivity,
  TrajectoryTokens,
  TrajectoryUsageEvidence,
} from './trajectory-types';

export interface SessionTrajectory {
  sessionId: string;
  turns: number;
  records: TrajectoryRecord[];
  requests: TrajectoryRequest[];
  /** D4 投影版本；v1 应答缺省为 1。 */
  projectionVersion: number;
  /** Run 级活动/等待状态（独立于请求行）。 */
  runActivity: TrajectoryRunActivity[];
  /** 每个参与折叠的 run 的事件水位（快照读取前缀的最大 seq）。 */
  watermarks: Record<string, number>;
  /** 窗口之外还有更早的 run（默认 20 / 上限 50）。 */
  hasOlderRuns: boolean;
}

function mapTokens(wire: TrajectoryTokensWire | undefined): TrajectoryTokens | undefined {
  if (wire === undefined) return undefined;
  return { input: wire.input, output: wire.output, think: wire.think, cacheRead: wire.cache_read, cacheWrite: wire.cache_write };
}

function mapRecord(wire: TrajectorySessionWire['records'][number]): TrajectoryRecord {
  return {
    index: wire.index,
    id: wire.id,
    turn: wire.turn,
    group: wire.group,
    kind: wire.kind as TrajectoryRecord['kind'],
    text: wire.text,
    timeSeconds: wire.time_seconds ?? null,
    startedAt: wire.started_at ?? null,
    tokens: mapTokens(wire.tokens),
    result: wire.result,
    isError: wire.is_error,
    inputDetail: wire.input_detail,
    outputDetail: wire.output_detail,
    callId: wire.call_id,
    provider: wire.provider,
    model: wire.model,
    opensTurn: wire.opens_turn,
  };
}

function mapUsage(wire: TrajectoryTokensWire): Required<TrajectoryTokens> {
  return {
    input: wire.input ?? 0,
    output: wire.output ?? 0,
    think: wire.think ?? 0,
    cacheRead: wire.cache_read ?? 0,
    cacheWrite: wire.cache_write ?? 0,
  };
}

function mapEvidence(wire: TrajectoryUsageEvidenceWire | null | undefined): TrajectoryUsageEvidence | null {
  if (wire === undefined || wire === null) return null;
  return {
    promptTokens: wire.prompt_tokens,
    completionTokens: wire.completion_tokens,
    totalTokens: wire.total_tokens,
    reasoningTokens: wire.reasoning_tokens ?? null,
    cachedTokens: wire.cached_tokens ?? null,
    partial: wire.partial ?? false,
  };
}

function mapRequest(wire: TrajectoryRequestWire): TrajectoryRequest {
  return {
    number: wire.number,
    turn: wire.turn,
    group: wire.group,
    purpose: wire.group === 'Compaction' ? 'compaction' : undefined,
    status: wire.status === 'error' ? 'error'
      : wire.status === 'active' ? 'active'
        : wire.status === 'cancelled' ? 'cancelled' : 'complete',
    startedAt: wire.started_at,
    completedAt: wire.completed_at,
    provider: wire.provider,
    model: wire.model,
    usage: mapUsage(wire.usage),
    error: wire.error,
    retry: wire.retry,
    messages: wire.messages,
    preambleBytes: wire.preamble_bytes,
    requestId: wire.request_id,
    runId: wire.run_id,
    callId: wire.call_id,
    callStatus: wire.call_status,
    finishedAt: wire.finished_at,
    usageState: wire.usage_state,
    usageEvidence: mapEvidence(wire.usage_evidence),
  };
}

function mapRunActivity(wire: NonNullable<TrajectorySessionWire['run_activity']>[number]): TrajectoryRunActivity {
  return {
    runId: wire.run_id,
    status: wire.status,
    activityState: wire.activity_state,
    waitKind: wire.wait_kind,
    parentRunId: wire.parent_run_id,
    childRunIds: wire.child_run_ids ?? [],
    workflowId: wire.workflow_id,
  };
}

/** 拉取并映射一个会话的轨迹投影；网络/RPC 错误原样上抛。 */
export async function loadSessionTrajectory(sessionId: string, limit?: number): Promise<SessionTrajectory> {
  const wire = await fetchSessionTrajectory(sessionId, limit);
  return {
    sessionId: wire.session_id,
    turns: wire.turns,
    records: (wire.records ?? []).map(mapRecord),
    requests: (wire.requests ?? []).map(mapRequest),
    projectionVersion: wire.projection_version ?? 1,
    runActivity: (wire.run_activity ?? []).map(mapRunActivity),
    watermarks: wire.watermarks ?? {},
    hasOlderRuns: wire.has_older_runs ?? false,
  };
}

/**
 * OBS-04 监听端判定器：复用 store.runEvents（单一订阅 owner）的
 * (run_id, seq) 去重水位，决定一次 store 事件是否需要一次快照重拉。
 * - duplicate / late（seq ≤ 水位）→ ignore
 * - contiguous（seq == 水位 + 1）→ advance + refresh
 * - gap（seq > 水位 + 1）→ refresh（不重置水位，乱序回填仍被视作重复；快照重拉即 reconcile）
 * - 未知 run_id → 新 run 发现，advance + refresh
 */
export class TrajectoryLiveTracker {
  private watermarks = new Map<string, number>();

  /** 快照到达后同步水位。 */
  setSnapshot(watermarks: Record<string, number>): void {
    this.watermarks = new Map(Object.entries(watermarks));
  }

  accept(runId: string, seq: number): 'ignore' | 'refresh' {
    const watermark = this.watermarks.get(runId);
    if (watermark === undefined) {
      this.watermarks.set(runId, seq);
      return 'refresh';
    }
    if (seq <= watermark) return 'ignore';
    if (seq === watermark + 1) this.watermarks.set(runId, seq);
    return 'refresh';
  }
}
