import { describe, expect, it } from 'vitest';
import {
  DEMO_TRAJECTORY_RECORDS,
  DEMO_TRAJECTORY_REQUESTS,
  TRAJECTORY_KIND_LABEL,
  type TrajectoryCellKind,
} from './trajectory-demo-data';
import {
  buildDisplayRows,
  deriveTrajectoryTimeline,
  formatDurationMs,
  formatTimelineOffset,
  laneForKind,
  trajectoryTimelineFocusIndexes,
  turnLabel,
  turnStartIndexes,
} from './trajectory-utils';

const KINDS: TrajectoryCellKind[] = ['system', 'user', 'context', 'compacted', 'message', 'tool', 'subtool'];

describe('trajectory demo data', () => {
  it('keeps indexes contiguous starting at 1 with stable ids', () => {
    expect(DEMO_TRAJECTORY_RECORDS[0].index).toBe(1);
    DEMO_TRAJECTORY_RECORDS.forEach((record, position) => {
      expect(record.index).toBe(position + 1);
      expect(record.id).toBe(`rec-${position + 1}`);
    });
  });

  it('uses only closed-set kinds with labels and deterministic timing', () => {
    for (const record of DEMO_TRAJECTORY_RECORDS) {
      expect(KINDS).toContain(record.kind);
      expect(TRAJECTORY_KIND_LABEL[record.kind]).toBeTruthy();
      if (record.timeSeconds !== null) expect(record.timeSeconds).toBeGreaterThanOrEqual(0);
      if (record.startedAt !== null) expect(Number.isFinite(record.startedAt)).toBe(true);
      for (const value of Object.values(record.tokens ?? {})) {
        expect(value ?? 0).toBeGreaterThanOrEqual(0);
      }
    }
  });

  it('covers every interesting record kind including an error and a compaction', () => {
    const kinds = new Set(DEMO_TRAJECTORY_RECORDS.map((record) => record.kind));
    expect(kinds.size).toBe(KINDS.length);
    expect(DEMO_TRAJECTORY_RECORDS.some((record) => record.isError === true)).toBe(true);
    expect(DEMO_TRAJECTORY_RECORDS.some((record) => record.kind === 'compacted')).toBe(true);
    expect(DEMO_TRAJECTORY_RECORDS.some((record) => record.kind === 'subtool')).toBe(true);
  });

  it('orders turns non-decreasing and requests with unique sequential numbers', () => {
    const turns = DEMO_TRAJECTORY_RECORDS.map((record) => record.turn);
    for (let index = 1; index < turns.length; index += 1) {
      const previous = turns[index - 1];
      const current = turns[index];
      if (previous === null || current === null) continue;
      expect(current).toBeGreaterThanOrEqual(previous);
    }
    const numbers = DEMO_TRAJECTORY_REQUESTS.map((request) => request.number);
    expect(numbers).toEqual([...numbers].sort((left, right) => left - right));
    expect(new Set(numbers).size).toBe(numbers.length);
    for (const request of DEMO_TRAJECTORY_REQUESTS) {
      expect(request.completedAt).toBeGreaterThan(request.startedAt);
      expect(request.usage.input + request.usage.output).toBeGreaterThan(0);
    }
  });
});

describe('deriveTrajectoryTimeline', () => {
  it('projects one equal-width span per record in sequence mode with DSH lanes', () => {
    const model = deriveTrajectoryTimeline(DEMO_TRAJECTORY_RECORDS, 'sequence');
    expect(model).not.toBeNull();
    expect(model!.spans.length).toBe(DEMO_TRAJECTORY_RECORDS.length);
    expect(model!.start).toBe(0);
    expect(model!.end).toBe(DEMO_TRAJECTORY_RECORDS.length);
    model!.spans.forEach((span, position) => {
      expect(span.index).toBe(position + 1);
      expect(span.start).toBe(position);
      expect(span.end).toBe(position + 1);
      expect(span.lane).toBe(laneForKind(DEMO_TRAJECTORY_RECORDS[position].kind));
    });
    expect(model!.turnBoundaries.map((boundary) => boundary.turn)).toEqual([1, 2, 3]);
  });

  it('applies laneForKind to tool/subtool in the tools lane and message in the model lane', () => {
    expect(laneForKind('tool')).toBe(2);
    expect(laneForKind('subtool')).toBe(2);
    expect(laneForKind('message')).toBe(1);
    expect(laneForKind('compacted')).toBe(1);
    expect(laneForKind('system')).toBe(0);
    expect(laneForKind('user')).toBe(0);
  });

  it('drops untimed records in duration mode and compresses idle gaps', () => {
    const untimed = DEMO_TRAJECTORY_RECORDS.filter((record) => record.startedAt === null);
    const model = deriveTrajectoryTimeline(DEMO_TRAJECTORY_RECORDS, 'duration');
    expect(model).not.toBeNull();
    expect(model!.spans.length).toBe(DEMO_TRAJECTORY_RECORDS.length - untimed.length);
    let previousEnd = Number.NEGATIVE_INFINITY;
    for (const span of [...model!.spans].sort((left, right) => left.start - right.start)) {
      expect(span.start).toBeGreaterThanOrEqual(previousEnd - 1e-9);
      previousEnd = span.end;
    }
    // 回合边界布局到各自回合内最早条带（用户行无计时，不参与）。
    expect(model!.turnBoundaries.length).toBe(3);
  });

  it('returns null for empty input', () => {
    expect(deriveTrajectoryTimeline([], 'sequence')).toBeNull();
    expect(deriveTrajectoryTimeline([], 'duration')).toBeNull();
  });
});

describe('trajectoryTimelineFocusIndexes', () => {
  it('includes records overlapping the range inclusive of boundaries', () => {
    const range = { start: 0.5, end: 1.5 };
    const indexes = trajectoryTimelineFocusIndexes(DEMO_TRAJECTORY_RECORDS, range, 'sequence');
    expect([...indexes].sort((left, right) => left - right)).toEqual([1, 2]);
  });

  it('returns an empty set for an empty model', () => {
    expect(trajectoryTimelineFocusIndexes([], { start: 0, end: 1 }, 'sequence').size).toBe(0);
  });
});

describe('turnStartIndexes', () => {
  it('marks the session-start section and every turn boundary', () => {
    const indexes = turnStartIndexes(DEMO_TRAJECTORY_RECORDS);
    expect(indexes.has(1)).toBe(true); // 会话起始区段
    const users = DEMO_TRAJECTORY_RECORDS.filter((record) => record.kind === 'user');
    for (const user of users) expect(indexes.has(user.index)).toBe(true);
  });
});

describe('format helpers', () => {
  it('formats durations with ms below one second and seconds above', () => {
    expect(formatDurationMs(null)).toBe('—');
    expect(formatDurationMs(999)).toBe('999 ms');
    expect(formatDurationMs(1000)).toBe('1.00 s');
    expect(formatDurationMs(2760)).toBe('2.76 s');
    expect(formatDurationMs(20000)).toBe('20.0 s');
  });

  it('formats timeline offsets as thousand-separated milliseconds', () => {
    expect(formatTimelineOffset(812)).toBe('812 ms');
    expect(formatTimelineOffset(1234)).toBe('1,234 ms');
  });

  it('labels turns as #N and the leading section as Session', () => {
    expect(turnLabel(null)).toBe('Session');
    expect(turnLabel(2)).toBe('#2');
  });
});

describe('buildDisplayRows', () => {
  it('passes every record through when nothing is collapsed', () => {
    const rows = buildDisplayRows(DEMO_TRAJECTORY_RECORDS, new Set(), new Set());
    expect(rows.length).toBe(DEMO_TRAJECTORY_RECORDS.length);
    expect(rows.every((row) => row.type === 'record')).toBe(true);
  });

  it('folds a whole turn into one summary row', () => {
    const rows = buildDisplayRows(DEMO_TRAJECTORY_RECORDS, new Set([1]), new Set());
    const summary = rows.find((row) => row.type === 'turnSummary');
    const count = DEMO_TRAJECTORY_RECORDS.filter((record) => record.turn === 1).length;
    if (summary === undefined || summary.type !== 'turnSummary') {
      throw new Error('expected a turn summary row');
    }
    expect(summary.turn).toBe(1);
    expect(summary.count).toBe(count);
    expect(rows.some((row) => row.type === 'record' && row.record.turn === 1)).toBe(false);
  });

  it('folds an assistant message with its trailing tool chain', () => {
    const assistant = DEMO_TRAJECTORY_RECORDS.find((record) =>
      record.kind === 'message' && record.turn === 1 && record.group === 'Step 1');
    expect(assistant).toBeDefined();
    const rows = buildDisplayRows(DEMO_TRAJECTORY_RECORDS, new Set(), new Set([assistant!.id]));
    const summary = rows.find((row) => row.type === 'assistantSummary');
    expect(summary).toMatchObject({ type: 'assistantSummary', id: assistant!.id, toolCount: 1 });
    expect(rows.some((row) => row.type === 'record' && row.record.id === assistant!.id)).toBe(false);
  });

  it('keeps rows in original order after folding', () => {
    const rows = buildDisplayRows(DEMO_TRAJECTORY_RECORDS, new Set([2]), new Set());
    const indexes: number[] = [];
    for (const row of rows) {
      if (row.type === 'record') indexes.push(row.record.index);
      if (row.type === 'assistantSummary') indexes.push(-1);
    }
    expect(indexes).toEqual([...indexes].sort((left, right) => left - right));
  });
});