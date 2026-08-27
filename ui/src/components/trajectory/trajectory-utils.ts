/**
 * 轨迹时间轴纯投影与格式化工具。
 * 复刻 DeepSeek Harness `packages/client/ui-trajectory/src/client/timeline.ts`
 * 与 `trajectory-record.ts` 的格式化/投影语义，无 React 依赖，便于单测。
 */

import type { TrajectoryRecord, TrajectoryTimelineMode, TrajectoryTimeRange } from './trajectory-demo-data';

/** 一条时间轴条带（记录在所选投影域内的位置）。 */
export interface TrajectoryTimelineSpan {
  /** 记录索引（全局唯一）。 */
  index: number;
  isError: boolean;
  kind: TrajectoryRecord['kind'];
  /** 条带所在泳道：0=Input、1=Model、2=Tools。 */
  lane: number;
  /** 域内起始位置。 */
  start: number;
  /** 域内结束位置。 */
  end: number;
}

/** 回合边界（在所选投影域内的位置）。 */
export interface TrajectoryTimelineTurnBoundary {
  turn: number;
  time: number;
}

/** 全域时间轴模型。 */
export interface TrajectoryTimelineModel {
  start: number;
  end: number;
  spans: readonly TrajectoryTimelineSpan[];
  turnBoundaries: readonly TrajectoryTimelineTurnBoundary[];
}

/** 记录 → 泳道映射（与 DSH laneFor 一致）。 */
export function laneForKind(kind: TrajectoryRecord['kind']): number {
  if (kind === 'tool' || kind === 'subtool') return 2;
  if (kind === 'message' || kind === 'compacted') return 1;
  return 0;
}

function finite(value: number | null | undefined): value is number {
  return value !== null && value !== undefined && Number.isFinite(value);
}

/** 记录在时长投影下的域区间；无 startedAt 时无法计时，返回 null。 */
function cellRange(record: TrajectoryRecord): TrajectoryTimeRange | null {
  if (!finite(record.startedAt)) return null;
  const durationMs = finite(record.timeSeconds) ? Math.max(0, record.timeSeconds * 1000) : 0;
  return { start: record.startedAt, end: record.startedAt + durationMs };
}

/**
 * 把全部记录投影为稳定的三泳道时间轴。
 * @param records - 未过滤的轨迹记录（按 index 升序）。
 * @param mode - `sequence` 等宽序列；`duration` 按真实时长并压缩空闲。
 * @returns 时间轴模型；无可见记录时返回 null。
 */
export function deriveTrajectoryTimeline(
  records: readonly TrajectoryRecord[],
  mode: TrajectoryTimelineMode = 'sequence',
): TrajectoryTimelineModel | null {
  if (records.length === 0) return null;
  const visible = records;

  if (mode === 'sequence') {
    const spans: TrajectoryTimelineSpan[] = visible.map((record, offset): TrajectoryTimelineSpan => ({
      index: record.index,
      isError: record.isError === true,
      kind: record.kind,
      lane: laneForKind(record.kind),
      start: offset,
      end: offset + 1,
    }));
    const turnBoundaries: TrajectoryTimelineTurnBoundary[] = [];
    for (const record of visible) {
      if (record.turn !== null && record.kind === 'user') {
        const at = visible.findIndex((candidate) => candidate.index === record.index);
        if (at >= 0) turnBoundaries.push({ turn: record.turn, time: at });
      }
    }
    return { start: 0, end: visible.length, spans, turnBoundaries };
  }

  // duration 模式：仅带时间信息且无 startedAt 缺口的记录入带；压缩跨记录空闲。
  const timed: Array<{ turn: number | null; span: TrajectoryTimelineSpan }> = [];
  for (const record of visible) {
    const range = cellRange(record);
    if (range === null) continue;
    timed.push({
      turn: record.turn,
      span: {
        index: record.index,
        isError: record.isError === true,
        kind: record.kind,
        lane: laneForKind(record.kind),
        ...range,
      },
    });
  }
  if (timed.length === 0) return null;

  const removedIdleByIndex = new Map<number, number>();
  let removedIdle = 0;
  let coveredUntil: number | null = null;
  for (const { span } of [...timed].sort((left, right) => left.span.start - right.span.start || left.span.end - right.span.end)) {
    if (coveredUntil !== null && span.start > coveredUntil) {
      removedIdle += span.start - coveredUntil;
    }
    removedIdleByIndex.set(span.index, removedIdle);
    coveredUntil = coveredUntil === null ? span.end : Math.max(coveredUntil, span.end);
  }

  const spans = timed.map(({ span }) => {
    const offset = removedIdleByIndex.get(span.index) ?? 0;
    return { ...span, start: span.start - offset, end: span.end - offset };
  });
  const start = Math.min(...spans.map((span) => span.start));
  const end = Math.max(...spans.map((span) => span.end));
  const boundaryByTurn = new Map<number, number>();
  for (const [index, entry] of timed.entries()) {
    if (entry.turn === null) continue;
    const at = spans[index].start;
    const current = boundaryByTurn.get(entry.turn);
    boundaryByTurn.set(entry.turn, current === undefined ? at : Math.min(current, at));
  }
  const turnBoundaries = [...boundaryByTurn.entries()]
    .map(([turn, time]) => ({ turn, time }))
    .sort((left, right) => left.time - right.time);
  return { start, end, spans, turnBoundaries };
}

/**
 * 找出与给定闭区间重叠的记录索引（用于时间轴选区 → 账本聚焦）。
 * @param records - 全部记录。
 * @param range - 当前投影域内的闭区间。
 * @param mode - 与选中区间相同的投影模式。
 * @returns 区间内记录索引集合；恒含区间边界上的记录。
 */
export function trajectoryTimelineFocusIndexes(
  records: readonly TrajectoryRecord[],
  range: TrajectoryTimeRange,
  mode: TrajectoryTimelineMode = 'sequence',
): ReadonlySet<number> {
  const model = deriveTrajectoryTimeline(records, mode);
  if (model === null) return new Set();
  return new Set(
    model.spans
      .filter((span) => span.start <= range.end && span.end >= range.start)
      .map((span) => span.index),
  );
}

/**
 * 格式化时长标签：<1s 显示整数毫秒，否则显示秒。
 * @param milliseconds - 时长（毫秒），未知传 null。
 * @returns `—` 或形如 `2,760 ms` / `3.14 s` 的文案。
 */
export function formatDurationMs(milliseconds: number | null): string {
  if (milliseconds === null || !Number.isFinite(milliseconds)) return '—';
  const ms = Math.max(0, milliseconds);
  if (ms < 1000) return `${Math.round(ms)} ms`;
  return `${(ms / 1000).toFixed(ms < 10000 ? 2 : 1)} s`;
}

/**
 * 时间轴偏移标签：与 DSH formatTimelineOffset 一致（整数毫秒，千分位）。
 * @param milliseconds - 偏移量（毫秒）。
 * @returns 形如 `1,234 ms` 的文案。
 */
export function formatTimelineOffset(milliseconds: number): string {
  const integer = String(Math.round(milliseconds));
  return `${integer.replace(/\B(?=(\d{3})+(?!\d))/g, ',')} ms`;
}

/** 回合标签：`#N`；null 区段显示会话起始。 */
export function turnLabel(turn: number | null): string {
  return turn === null ? 'Session' : `#${turn}`;
}

/** 回合起始记录的索引（账本展示回合标签用；首个区段记录也算起始）。 */
export function turnStartIndexes(records: readonly TrajectoryRecord[]): ReadonlySet<number> {
  const indexes = new Set<number>();
  let previous: number | null | undefined;
  for (const record of records) {
    if (record.turn !== previous) indexes.add(record.index);
    previous = record.turn;
  }
  return indexes;
}

/** 请求起始记录索引（账本渲染 `Request #N` 标记用）。 */
export function requestStartIndexes(records: readonly TrajectoryRecord[]): ReadonlySet<number> {
  const indexes = new Set<number>();
  let previousGroup: string | null = null;
  let previousTurn: number | null = null;
  for (const record of records) {
    const isNewGroup = previousGroup === null
      || record.group !== previousGroup
      || record.turn !== previousTurn;
    if (isNewGroup && record.kind === 'message') indexes.add(record.index);
    previousGroup = record.group;
    previousTurn = record.turn;
  }
  return indexes;
}

/** 账本展示行：记录行或折叠摘要行。 */
export type DisplayRow =
  | { type: 'record'; record: TrajectoryRecord }
  | { type: 'turnSummary'; turn: number; count: number }
  | { type: 'assistantSummary'; id: string; text: string; toolCount: number };

/**
 * 按折叠状态把记录投影为账本展示行：
 * - `collapsedTurns` 内的回合 → 单条回合折叠摘要行；
 * - `collapsedAssistants` 内的 ASSISTANT 行（其后为同组工具行）→ 单条调用链摘要行。
 * 顺序与输入记录一致；折叠互斥（搜索时由调用方传空集合）。
 */
export function buildDisplayRows(
  records: readonly TrajectoryRecord[],
  collapsedTurns: ReadonlySet<number>,
  collapsedAssistants: ReadonlySet<string>,
): DisplayRow[] {
  const rows: DisplayRow[] = [];
  let index = 0;
  while (index < records.length) {
    const record = records[index];
    if (record.turn !== null && collapsedTurns.has(record.turn)) {
      let cursor = index;
      while (cursor < records.length && records[cursor].turn === record.turn) cursor += 1;
      rows.push({ type: 'turnSummary', turn: record.turn, count: cursor - index });
      index = cursor;
      continue;
    }
    if (record.kind === 'message' && collapsedAssistants.has(record.id)) {
      let toolCount = 0;
      let cursor = index + 1;
      while (
        cursor < records.length
        && records[cursor].turn === record.turn
        && records[cursor].group === record.group
        && (records[cursor].kind === 'tool' || records[cursor].kind === 'subtool')
      ) {
        toolCount += 1;
        cursor += 1;
      }
      if (toolCount > 0) {
        rows.push({ type: 'assistantSummary', id: record.id, text: record.text, toolCount });
        index = cursor;
        continue;
      }
    }
    rows.push({ type: 'record', record });
    index += 1;
  }
  return rows;
}