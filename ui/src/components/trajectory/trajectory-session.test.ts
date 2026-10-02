/**
 * OBS-04: TrajectoryLiveTracker — listener-before-snapshot 合并判定。
 * duplicate/late → ignore；contiguous → advance+refresh；gap → refresh；
 * 未知 run_id → 新 run 发现 + refresh。
 */

import { describe, expect, it } from 'vitest';
import { TrajectoryLiveTracker } from './trajectory-session';

describe('TrajectoryLiveTracker', () => {
  it('ignores duplicate and late events below the watermark', () => {
    const tracker = new TrajectoryLiveTracker();
    tracker.setSnapshot({ 'run-1': 5 });
    expect(tracker.accept('run-1', 5)).toBe('ignore');
    expect(tracker.accept('run-1', 3)).toBe('ignore');
  });

  it('advances on contiguous sequences only', () => {
    const tracker = new TrajectoryLiveTracker();
    tracker.setSnapshot({ 'run-1': 5 });
    expect(tracker.accept('run-1', 6)).toBe('refresh');
    expect(tracker.accept('run-1', 7)).toBe('refresh');
    expect(tracker.accept('run-1', 7)).toBe('ignore');
  });

  it('flags gaps for snapshot reconcile without skipping ahead', () => {
    const tracker = new TrajectoryLiveTracker();
    tracker.setSnapshot({ 'run-1': 5 });
    expect(tracker.accept('run-1', 9)).toBe('refresh');
    // An out-of-order event filling the gap is still accepted as contiguous.
    expect(tracker.accept('run-1', 6)).toBe('refresh');
    expect(tracker.accept('run-1', 6)).toBe('ignore');
    // The stale gap seq is now late, not a second gap.
    expect(tracker.accept('run-1', 9)).toBe('refresh');
  });

  it('discovers new runs and keeps per-run watermarks independent', () => {
    const tracker = new TrajectoryLiveTracker();
    tracker.setSnapshot({ 'run-1': 5 });
    expect(tracker.accept('run-2', 1)).toBe('refresh');
    expect(tracker.accept('run-2', 1)).toBe('ignore');
    expect(tracker.accept('run-1', 6)).toBe('refresh');
  });

  it('resyncs cleanly when a fresh snapshot arrives', () => {
    const tracker = new TrajectoryLiveTracker();
    tracker.setSnapshot({ 'run-1': 5 });
    tracker.accept('run-1', 9);
    tracker.setSnapshot({ 'run-1': 12 });
    expect(tracker.accept('run-1', 9)).toBe('ignore');
    expect(tracker.accept('run-1', 13)).toBe('refresh');
  });
});
