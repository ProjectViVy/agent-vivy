/**
 * trajectory/session wire → 展示层映射（UI-TRAJECTORY-DEMO）。
 * snake_case wire 形态（api.ts）映射为 DSH 风格的展示层类型
 * （trajectory-types.ts）；只做字段搬运与默认值填充，不做语义加工。
 */

import { fetchSessionTrajectory, type TrajectoryRequestWire, type TrajectorySessionWire, type TrajectoryTokensWire } from '@/lib/api';
import type { TrajectoryRecord, TrajectoryRequest, TrajectoryTokens } from './trajectory-types';

export interface SessionTrajectory {
  sessionId: string;
  turns: number;
  records: TrajectoryRecord[];
  requests: TrajectoryRequest[];
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

function mapRequest(wire: TrajectoryRequestWire): TrajectoryRequest {
  return {
    number: wire.number,
    turn: wire.turn,
    group: wire.group,
    purpose: wire.group === 'Compaction' ? 'compaction' : undefined,
    status: wire.status === 'error' ? 'error' : 'complete',
    startedAt: wire.started_at,
    completedAt: wire.completed_at,
    provider: wire.provider,
    model: wire.model,
    usage: mapUsage(wire.usage),
    error: wire.error,
    retry: wire.retry,
    messages: wire.messages,
    preambleBytes: wire.preamble_bytes,
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
  };
}
