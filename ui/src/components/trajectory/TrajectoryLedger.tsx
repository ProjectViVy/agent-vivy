/**
 * 回合感知的轨迹事件账本：两列布局（事件列 + 内容列），
 * 复刻 DeepSeek Harness `TrajectoryTable` 的可见结构（简化实现）。
 */

import { useMemo, type KeyboardEvent } from 'react';
import {
  Info,
  Minimize2,
  Settings,
  Sparkles,
  User,
  Wrench,
} from 'lucide-react';
import { useTranslation } from '@/i18n';
import {
  TRAJECTORY_KIND_LABEL,
  type TrajectoryRecord,
} from './trajectory-types';
import {
  turnLabel,
  turnStartIndexes,
  type DisplayRow,
  buildDisplayRows,
} from './trajectory-utils';

export interface TrajectoryLedgerProps {
  records: readonly TrajectoryRecord[];
  collapsedTurns: ReadonlySet<number>;
  collapsedAssistants: ReadonlySet<string>;
  /** 时间轴选区聚焦索引；非 null 时区间外的行淡化。 */
  focusIndexes: ReadonlySet<number> | null;
  selectedRecordId: string | null;
  /** 记录 id → 请求号（用于在 ASSISTANT 行渲染 `Request #N` 标记）。 */
  requestNumberByRecordId: ReadonlyMap<string, number>;
  onToggleTurn: (turn: number) => void;
  onToggleAssistant: (id: string) => void;
  onSelectRecord: (id: string) => void;
  onSelectRequest: (number: number) => void;
}

const KIND_ICON: Record<TrajectoryRecord['kind'], typeof Settings> = {
  system: Settings,
  user: User,
  context: Info,
  compacted: Minimize2,
  message: Sparkles,
  tool: Wrench,
  subtool: Wrench,
};

const KIND_TAG_CLASS: Record<TrajectoryRecord['kind'], string> = {
  system: 'bg-slate-500/10 text-slate-600 dark:text-slate-400',
  user: 'bg-blue-500/10 text-blue-600 dark:text-blue-400',
  context: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  compacted: 'bg-purple-500/10 text-purple-600 dark:text-purple-400',
  message: 'bg-violet-500/10 text-violet-600 dark:text-violet-400',
  tool: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
  subtool: 'bg-orange-500/10 text-orange-600 dark:text-orange-400',
};

/** 渲染一条记录行。 */
function RecordRow({
  record,
  isTurnStart,
  turnRail,
  requestNumber,
  focusDimmed,
  selected,
  onSelectRecord,
  onSelectRequest,
}: {
  record: TrajectoryRecord;
  isTurnStart: boolean;
  turnRail: boolean;
  requestNumber: number | undefined;
  focusDimmed: boolean;
  selected: boolean;
  onSelectRecord: (id: string) => void;
  onSelectRequest: (number: number) => void;
}) {
  const { t } = useTranslation();
  const Icon = KIND_ICON[record.kind];
  const handleKeyDown = (event: KeyboardEvent<HTMLTableRowElement>) => {
    if (event.key !== 'Enter' && event.key !== ' ') return;
    event.preventDefault();
    onSelectRecord(record.id);
  };
  return (
    <tr
      tabIndex={0}
      aria-selected={selected}
      data-kind={record.kind}
      data-trajectory-row-key={record.id}
      data-error={record.isError || undefined}
      onClick={() => onSelectRecord(record.id)}
      onKeyDown={handleKeyDown}
      className={[
        'cursor-pointer border-b border-border/50 text-left align-top outline-none',
        'hover:bg-muted/40 focus-visible:bg-muted/60',
        focusDimmed ? 'opacity-35' : '',
        selected ? 'bg-primary/10' : '',
      ].join(' ')}
    >
      <td className="w-[210px] px-2 py-1.5">
        <div className="flex items-center gap-1.5">
          {isTurnStart && (
            <span
              className="shrink-0 rounded border px-1 font-mono text-[10px] leading-4 text-muted-foreground"
              aria-label={record.turn === null ? t('trajectory.session') : `${t('trajectory.turn')} ${record.turn}`}
            >
              {turnLabel(record.turn)}
            </span>
          )}
          {requestNumber !== undefined && (
            <button
              type="button"
              className="shrink-0 rounded border border-primary/40 px-1 font-mono text-[10px] leading-4 text-primary hover:bg-primary/10"
              aria-label={`${t('trajectory.request')} ${requestNumber}`}
              onClick={(event) => {
                event.stopPropagation();
                onSelectRequest(requestNumber);
              }}
            >
              {t('trajectory.request')} {requestNumber}
            </button>
          )}
        </div>
        <div className={turnRail ? 'mt-1 border-l-2 border-primary/40 pl-1.5' : 'mt-1'}>
          <span className={`inline-flex items-center gap-1 rounded px-1 py-0.5 font-mono text-[10px] leading-4 ${KIND_TAG_CLASS[record.kind]}`}>
            <Icon className="h-3 w-3" aria-hidden="true" />
            {TRAJECTORY_KIND_LABEL[record.kind]}
          </span>
        </div>
      </td>
      <td className="min-w-0 px-2 py-1.5">
        <span className="text-xs leading-5 text-foreground">
          <span className={record.kind === 'tool' || record.kind === 'subtool' ? 'font-mono' : ''}>
            {record.text}
          </span>
          {record.result !== undefined && (
            <span className={record.isError ? 'text-red-600 dark:text-red-400' : 'text-muted-foreground'}>
              {' '}
              <span aria-hidden="true">→</span>
              {' '}
              <span className={record.isError ? 'font-medium text-red-600 dark:text-red-400' : ''}>{record.result}</span>
            </span>
          )}
        </span>
      </td>
    </tr>
  );
}

/** 两条折叠摘要行（回合 / 助理调用链）。 */
function SummaryRow({
  label,
  detail,
  onClick,
}: {
  label: string;
  detail: string;
  onClick: () => void;
}) {
  return (
    <tr
      tabIndex={0}
      className="cursor-pointer border-b border-border/50 text-left align-top hover:bg-muted/40 focus-visible:bg-muted/60"
      onClick={onClick}
      onKeyDown={(event) => {
        if (event.key !== 'Enter' && event.key !== ' ') return;
        event.preventDefault();
        onClick();
      }}
    >
      <td className="w-[210px] px-2 py-1.5" />
      <td className="px-2 py-1.5">
        <span className="inline-flex items-center gap-2 text-xs text-muted-foreground">
          <span aria-hidden="true">…</span>
          <span>
            {label}
            {detail}
          </span>
        </span>
      </td>
    </tr>
  );
}

/** 渲染轨迹账本表。 */
export function TrajectoryLedger({
  records,
  collapsedTurns,
  collapsedAssistants,
  focusIndexes,
  selectedRecordId,
  requestNumberByRecordId,
  onToggleTurn,
  onToggleAssistant,
  onSelectRecord,
  onSelectRequest,
}: TrajectoryLedgerProps) {
  const { t } = useTranslation();
  const displayRows = useMemo(
    () => buildDisplayRows(records, collapsedTurns, collapsedAssistants),
    [records, collapsedTurns, collapsedAssistants],
  );
  const turnStarts = useMemo(() => turnStartIndexes(records), [records]);

  if (displayRows.length === 0) {
    return (
      <div className="flex h-full items-center justify-center px-4 text-xs text-muted-foreground">
        {t('trajectory.empty')}
      </div>
    );
  }

  let previousTurn: number | null = null;
  return (
    <table className="w-full border-collapse">
      <tbody>
        {displayRows.map((row: DisplayRow) => {
          if (row.type === 'turnSummary') {
            return (
              <SummaryRow
                key={`turn-summary-${row.turn}`}
                label={`${turnLabel(row.turn)} · `}
                detail={t('trajectory.collapsedTurn', { count: row.count })}
                onClick={() => onToggleTurn(row.turn)}
              />
            );
          }
          if (row.type === 'assistantSummary') {
            return (
              <SummaryRow
                key={`assistant-summary-${row.id}`}
                label={row.text ? `${row.text} · ` : ''}
                detail={t('trajectory.collapsedCalls', { count: row.toolCount })}
                onClick={() => onToggleAssistant(row.id)}
              />
            );
          }
          const isTurnStart = turnStarts.has(row.record.index);
          const turnRail = !isTurnStart && previousTurn === row.record.turn;
          previousTurn = row.record.turn;
          return (
            <RecordRow
              key={row.record.id}
              record={row.record}
              isTurnStart={isTurnStart}
              turnRail={turnRail}
              requestNumber={requestNumberByRecordId.get(row.record.id)}
              focusDimmed={focusIndexes !== null && !focusIndexes.has(row.record.index)}
              selected={selectedRecordId === row.record.id}
              onSelectRecord={onSelectRecord}
              onSelectRequest={onSelectRequest}
            />
          );
        })}
      </tbody>
    </table>
  );
}