/**
 * Chrome-Network 式轨迹总览时间轴：三泳道（Input/Model/Tools）条带、
 * 实际/等宽投影切换、拖拽选区间、悬停 tooltip。
 * 复刻 DeepSeek Harness `TrajectoryTimeline` 的可见行为（简化实现）。
 */

import { useMemo, useRef, useState, type PointerEvent as ReactPointerEvent } from 'react';
import { useTranslation } from '@/i18n';
import type {
  TrajectoryCellKind,
  TrajectoryRecord,
  TrajectoryTimelineMode,
  TrajectoryTimeRange,
} from './trajectory-types';
import {
  deriveTrajectoryTimeline,
  formatDurationMs,
  formatTimelineOffset,
  type TrajectoryTimelineSpan,
} from './trajectory-utils';

const MINIMUM_DRAG_PX = 3;
const LANE_HEIGHT_PX = 14;
const SPAN_HEIGHT_PX = 8;

const SPAN_KIND_CLASS: Record<TrajectoryCellKind, string> = {
  system: 'bg-slate-400',
  user: 'bg-blue-500',
  context: 'bg-emerald-500/80',
  compacted: 'bg-slate-500',
  message: '',
  tool: 'bg-amber-500',
  subtool: 'bg-amber-400',
};

/** 助理条带解码段基色（近似 DSH 紫罗兰）。 */
const ASSISTANT_COLOR = 'rgba(139, 92, 246, 0.92)';
/** 助理条带 TTFT 段基色（近似 DSH 亮色混合）。 */
const ASSISTANT_TTFT_COLOR = 'rgba(139, 92, 246, 0.42)';

function spanBackground(span: TrajectoryTimelineSpan, records: readonly TrajectoryRecord[]): string {
  if (span.kind !== 'message') return '';
  const record = records.find((candidate) => candidate.index === span.index);
  const durationMs = record?.timeSeconds == null
    ? 0
    : Math.max(0, record.timeSeconds * 1000);
  const ttftMs = record?.ttftMs;
  if (durationMs > 0 && ttftMs != null && ttftMs >= 0 && ttftMs <= durationMs) {
    const fraction = (ttftMs / durationMs) * 100;
    return `linear-gradient(to right, ${ASSISTANT_TTFT_COLOR} 0, ${ASSISTANT_TTFT_COLOR} ${fraction}%, ${ASSISTANT_COLOR} ${fraction}%, ${ASSISTANT_COLOR} 100%)`;
  }
  return ASSISTANT_COLOR;
}

function formatRecordedTime(timestamp: number): string {
  const date = new Date(timestamp);
  const two = (value: number) => String(value).padStart(2, '0');
  const three = (value: number) => String(value).padStart(3, '0');
  return `${two(date.getHours())}:${two(date.getMinutes())}:${two(date.getSeconds())}.${three(date.getMilliseconds())}`;
}

export interface TrajectoryTimelineProps {
  records: readonly TrajectoryRecord[];
  mode: TrajectoryTimelineMode;
  /** 已生效的时区选区（投影域内的闭区间）。 */
  range: TrajectoryTimeRange | null;
  /** 与 range 对应的聚焦记录索引（时间轴外记录淡化）。 */
  focusIndexes: ReadonlySet<number> | null;
  selectedIndex: number | null;
  /** 命中搜索的记录索引；null 表示无搜索。 */
  searchMatchIndexes: ReadonlySet<number> | null;
  onRangeChange: (range: TrajectoryTimeRange | null) => void;
  onRecordSelect: (index: number) => void;
}

interface DragState {
  pointerId: number;
  startClientX: number;
  startFraction: number;
  moved: boolean;
}

interface HoverState {
  index: number | null;
  fraction: number;
}

/** 总览时间轴：条带 + 拖拽选区间 + 悬停提示。 */
export function TrajectoryTimeline({
  records,
  mode,
  range,
  focusIndexes,
  selectedIndex,
  searchMatchIndexes,
  onRangeChange,
  onRecordSelect,
}: TrajectoryTimelineProps) {
  const { t } = useTranslation();
  const model = useMemo(() => deriveTrajectoryTimeline(records, mode), [records, mode]);
  const dragRef = useRef<DragState | null>(null);
  const [dragRange, setDragRange] = useState<{ start: number; end: number } | null>(null);
  const [hover, setHover] = useState<HoverState>({ index: null, fraction: 0 });
  const recordByIndex = useMemo(
    () => new Map(records.map((record) => [record.index, record] as const)),
    [records],
  );

  if (model === null) {
    return (
      <div className="flex h-[50px] items-center justify-center border-b text-xs text-muted-foreground">
        {t('trajectory.empty')}
      </div>
    );
  }

  const domainLength = Math.max(1, model.end - model.start);
  const fractionToValue = (fraction: number) => model.start + fraction * domainLength;

  const spanStyle = (span: TrajectoryTimelineSpan) => {
    const left = (span.start - model.start) / domainLength * 100;
    const width = (span.end - span.start) / domainLength * 100;
    return {
      left: `${left}%`,
      width: `calc(${width}% - 1px)`,
      top: `${7 + span.lane * LANE_HEIGHT_PX}px`,
      height: `${SPAN_HEIGHT_PX}px`,
      minWidth: '2px',
    } as const;
  };

  const commitRange = (drag: { start: number; end: number }) => {
    const ordered = drag.start <= drag.end ? drag : { start: drag.end, end: drag.start };
    onRangeChange({
      start: fractionToValue(ordered.start),
      end: fractionToValue(ordered.end),
    });
  };

  const handlePointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    const rect = event.currentTarget.getBoundingClientRect();
    const fraction = (event.clientX - rect.left) / Math.max(1, rect.width);
    dragRef.current = {
      pointerId: event.pointerId,
      startClientX: event.clientX,
      startFraction: fraction,
      moved: false,
    };
    event.currentTarget.setPointerCapture(event.pointerId);
    setDragRange({ start: fraction, end: fraction });
  };

  const handlePointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (dragRef.current !== null && dragRef.current.pointerId === event.pointerId) {
      const rect = event.currentTarget.getBoundingClientRect();
      const fraction = Math.min(1, Math.max(0, (event.clientX - rect.left) / Math.max(1, rect.width)));
      const drag = dragRef.current;
      if (!drag.moved && Math.abs(event.clientX - drag.startClientX) >= MINIMUM_DRAG_PX) {
        drag.moved = true;
      }
      if (drag.moved) setDragRange({ start: drag.startFraction, end: fraction });
      return;
    }
    const rect = event.currentTarget.getBoundingClientRect();
    const fraction = Math.min(1, Math.max(0, (event.clientX - rect.left) / Math.max(1, rect.width)));
    const value = fractionToValue(fraction);
    const span = model.spans.find((candidate) => value >= candidate.start && value < candidate.end)
      ?? model.spans.find((candidate) => value >= candidate.start && value <= candidate.end);
    setHover({ index: span?.index ?? null, fraction });
  };

  const handlePointerUp = (event: ReactPointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    dragRef.current = null;
    if (drag === null || drag.pointerId !== event.pointerId) return;
    event.currentTarget.releasePointerCapture(event.pointerId);
    if (drag.moved) {
      if (dragRange !== null) commitRange(dragRange);
      setDragRange(null);
      return;
    }
    setDragRange(null);
    const rect = event.currentTarget.getBoundingClientRect();
    const fraction = Math.min(1, Math.max(0, (event.clientX - rect.left) / Math.max(1, rect.width)));
    const value = fractionToValue(fraction);
    const span = model.spans.find((candidate) => value >= candidate.start && value < candidate.end);
    if (span !== undefined) onRecordSelect(span.index);
    else onRangeChange(null);
  };

  const clearSelection = () => {
    if (dragRef.current !== null) return;
    onRangeChange(null);
  };

  const hoveredSpan = hover.index === null
    ? null
    : model.spans.find((span) => span.index === hover.index);
  const hoveredRecord = hover.index === null
    ? null
    : recordByIndex.get(hover.index) ?? null;

  const tooltipLines: string[] = [];
  if (hoveredRecord !== null) {
    tooltipLines.push(hoveredRecord.kind.toUpperCase());
    const durationMs = hoveredRecord.timeSeconds == null
      ? null
      : hoveredRecord.timeSeconds * 1000;
    if (hoveredRecord.startedAt !== null) {
      const endedAt = durationMs === null
        ? null
        : hoveredRecord.startedAt + durationMs;
      tooltipLines.push(endedAt === null
        ? `Started ${formatRecordedTime(hoveredRecord.startedAt)}`
        : `${formatRecordedTime(hoveredRecord.startedAt)} → ${formatRecordedTime(endedAt)}`);
    }
    const durationLabel = durationMs === null ? null : `Total ${formatDurationMs(durationMs)}`;
    const ttftLabel = hoveredRecord.ttftMs == null || durationMs === null
      ? null
      : `TTFT ${formatTimelineOffset(hoveredRecord.ttftMs)} · Decoding ${formatTimelineOffset(durationMs - hoveredRecord.ttftMs)}`;
    const timing = [durationLabel, ttftLabel].filter((value) => value !== null).join(' · ');
    if (timing !== '') tooltipLines.push(timing);
    const preview = hoveredRecord.text.length > 56
      ? `${hoveredRecord.text.slice(0, 56)}…`
      : hoveredRecord.text;
    if (preview !== '' && !tooltipLines.includes(preview)) tooltipLines.push(preview);
  }

  return (
    <div
      className="grid grid-cols-[44px_minmax(0,1fr)] h-[50px] flex-none border-b select-none"
      role="img"
      aria-label={t('trajectory.timelineAria')}
      data-trajectory-timeline=""
    >
      <div className="relative border-r bg-muted/40" aria-hidden="true">
        {(['Input', 'Model', 'Tools'] as const).map((label, lane) => (
          <span
            key={label}
            className="absolute right-1 text-[10px] leading-none text-muted-foreground"
            style={{ top: `${7 + lane * LANE_HEIGHT_PX}px` }}
          >
            {label}
          </span>
        ))}
      </div>
      <div
        className="relative cursor-crosshair touch-none overflow-hidden outline-none focus-visible:ring-1 focus-visible:ring-ring"
        tabIndex={0}
        data-trajectory-track=""
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerCancel={() => { dragRef.current = null; setDragRange(null); }}
        onPointerLeave={() => setHover({ index: null, fraction: 0 })}
        onKeyDown={(event) => { if (event.key === 'Escape') clearSelection(); }}
      >
        {/* 回合边界竖线 */}
        {model.turnBoundaries.map((boundary) => (
          <span
            key={`turn-${boundary.turn}`}
            className="absolute top-0 bottom-0 w-px bg-border"
            style={{ left: `${(boundary.time - model.start) / domainLength * 100}%` }}
            aria-hidden="true"
          />
        ))}
        {/* 条带 */}
        {model.spans.map((span) => {
          const dimmedBySearch = searchMatchIndexes !== null && !searchMatchIndexes.has(span.index);
          const dimmedByFocus = focusIndexes !== null && !focusIndexes.has(span.index);
          const isSelected = selectedIndex === span.index;
          return (
            <span
              key={span.index}
              className={[
                'absolute rounded-[1px] transition-opacity',
                SPAN_KIND_CLASS[span.kind],
                dimmedBySearch ? 'opacity-15' : dimmedByFocus ? 'opacity-25' : 'opacity-80',
                isSelected ? 'opacity-100 ring-1 ring-ring' : '',
              ].join(' ')}
              style={{
                ...spanStyle(span),
                background: spanBackground(span, records),
              }}
              data-timeline-span={span.kind}
              data-error={span.isError || undefined}
              data-lane={span.lane}
              aria-hidden="true"
            />
          );
        })}
        {/* 拖拽中的选区 */}
        {dragRange !== null && (
          <span
            className="absolute top-0 bottom-0 bg-primary/15"
            style={{
              left: `${Math.min(dragRange.start, dragRange.end) * 100}%`,
              width: `${Math.abs(dragRange.end - dragRange.start) * 100}%`,
            }}
            aria-hidden="true"
          />
        )}
        {/* 已生效选区 */}
        {range !== null && dragRange === null && (
          <span
            className="absolute top-0 bottom-0 border-x border-primary bg-primary/10"
            style={{
              left: `${(range.start - model.start) / domainLength * 100}%`,
              width: `${(range.end - range.start) / domainLength * 100}%`,
            }}
            aria-hidden="true"
          />
        )}
        {/* 悬停竖线 */}
        {hover.index !== null && dragRange === null && (
          <span
            className="pointer-events-none absolute top-0 bottom-0 w-0.5 bg-primary"
            style={{ left: `calc(${hover.fraction * 100}% - 1px)` }}
            aria-hidden="true"
          />
        )}
        {/* 悬停 tooltip */}
        {hoveredSpan !== null && hoveredRecord !== null && tooltipLines.length > 0 && (
          <div
            role="tooltip"
            className="pointer-events-none absolute top-1 z-20 max-w-72 whitespace-pre-line rounded-md border bg-popover px-2 py-1.5 font-mono text-[10px] leading-4 text-popover-foreground shadow-md"
            style={{ left: `${Math.min(hover.fraction * 100, 60)}%` }}
          >
            {tooltipLines.join('\n')}
          </div>
        )}
      </div>
    </div>
  );
}