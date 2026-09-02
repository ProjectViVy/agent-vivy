/**
 * 轨迹详情面板：请求级（摘要/用量/时序）与记录级（输入/输出/思考）两种视图。
 * 复刻 DeepSeek Harness `TrajectoryTable` 右侧事件详情 inspector 的结构。
 */

import { useState } from 'react';
import { X } from 'lucide-react';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useTranslation } from '@/i18n';
import {
  TRAJECTORY_KIND_LABEL,
  type TrajectoryRecord,
  type TrajectoryRequest,
} from './trajectory-types';
import { formatDurationMs, turnLabel } from './trajectory-utils';

export interface TrajectoryDetailPanelProps {
  /** 当前选中的请求（优先于记录）。 */
  request: TrajectoryRequest | null;
  /** 当前选中的记录。 */
  record: TrajectoryRecord | null;
  /** 该请求步内包含的工具/子工具行（Summary 计数用）。 */
  requestTools: readonly TrajectoryRecord[];
  /** 该请求对应的 ASSISTANT 记录（TTFT/解码用时来源）。 */
  requestAssistant: TrajectoryRecord | null;
  onClose: () => void;
}

type RequestTab = 'summary' | 'usage' | 'timing';
type RecordTab = 'input' | 'output' | 'thinking';

/** 渲染请求级详情（摘要 / 用量 / 时序）。 */
function RequestDetails({
  request,
  tools,
  assistant,
  t,
}: {
  request: TrajectoryRequest;
  tools: readonly TrajectoryRecord[];
  assistant: TrajectoryRecord | null;
  t: (key: string, params?: Record<string, string | number>) => string;
}) {
  const [tab, setTab] = useState<RequestTab>('summary');
  const toolCount = tools.filter((record) => record.kind === 'tool').length;
  const subtoolCount = tools.filter((record) => record.kind === 'subtool').length;
  const totalMs = request.completedAt - request.startedAt;
  const ttftMs = assistant?.ttftMs ?? undefined;
  const decodingMs = ttftMs === undefined ? null : Math.max(0, totalMs - ttftMs);
  const tabs: Array<{ id: RequestTab; label: string }> = [
    { id: 'summary', label: t('trajectory.summary') },
    { id: 'usage', label: t('trajectory.usage') },
    { id: 'timing', label: t('trajectory.timing') },
  ];

  return (
    <>
      <div className="flex gap-1 border-b px-3 py-1.5" role="tablist" aria-label={t('trajectory.detailsAria')}>
        {tabs.map((item) => (
          <button
            key={item.id}
            type="button"
            role="tab"
            aria-selected={tab === item.id}
            onClick={() => setTab(item.id)}
            className={[
              'rounded px-2 py-1 text-xs',
              tab === item.id
                ? 'bg-primary/10 text-primary'
                : 'text-muted-foreground hover:bg-muted/60',
            ].join(' ')}
          >
            {item.label}
          </button>
        ))}
      </div>
      <ScrollArea className="min-h-0 flex-1">
        {tab === 'summary' && (
          <dl className="space-y-2.5 px-3 py-3 text-xs">
            <div className="flex justify-between gap-3">
              <dt className="text-muted-foreground">{t('trajectory.status')}</dt>
              <dd className={request.status === 'error' ? 'text-red-600' : undefined}>
                {request.status === 'error' ? t('trajectory.statusError') : t('trajectory.statusComplete')}
              </dd>
            </div>
            {request.purpose === 'compaction' && (
              <div className="flex justify-between gap-3">
                <dt className="text-muted-foreground">{t('trajectory.purpose')}</dt>
                <dd>{t('trajectory.compaction')}</dd>
              </div>
            )}
            <div className="flex justify-between gap-3">
              <dt className="text-muted-foreground">{t('trajectory.provider')}</dt>
              <dd className="font-mono">{request.provider ?? request.requestConfig?.provider ?? '—'}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-muted-foreground">{t('trajectory.model')}</dt>
              <dd className="font-mono">{request.model ?? request.requestConfig?.model ?? '—'}</dd>
            </div>
            {request.purpose !== 'compaction' && (
              <div className="flex justify-between gap-3">
                <dt className="text-muted-foreground">{t('trajectory.toolCalls')}</dt>
                <dd>{toolCount}</dd>
              </div>
            )}
            {subtoolCount > 0 && (
              <div className="flex justify-between gap-3">
                <dt className="text-muted-foreground">{t('trajectory.subtoolCalls')}</dt>
                <dd>{subtoolCount}</dd>
              </div>
            )}
            {request.error !== undefined && (
              <div className="flex justify-between gap-3">
                <dt className="text-muted-foreground">{t('trajectory.error')}</dt>
                <dd className="text-red-600">{request.error}</dd>
              </div>
            )}
            {request.retry !== undefined && (
              <div className="flex justify-between gap-3">
                <dt className="text-muted-foreground">{t('trajectory.retry')}</dt>
                <dd>
                  {t('trajectory.retryOf', {
                    done: request.retry,
                    max: request.maxRetries ?? 1,
                  })}
                </dd>
              </div>
            )}
            {request.retryDelayMs !== undefined && (
              <div className="flex justify-between gap-3">
                <dt className="text-muted-foreground">{t('trajectory.retryDelay')}</dt>
                <dd className="font-mono">{formatDurationMs(request.retryDelayMs)}</dd>
              </div>
            )}
            {assistant !== null && (
              <div className="flex justify-between gap-3">
                <dt className="text-muted-foreground">{t('trajectory.result')}</dt>
                <dd className="max-w-52 truncate text-right">{assistant.text}</dd>
              </div>
            )}
          </dl>
        )}
        {tab === 'usage' && (
          <dl className="space-y-2.5 px-3 py-3 text-xs">
            {([
              ['input', request.usage.input],
              ['cacheRead', request.usage.cacheRead],
              ['cacheWrite', request.usage.cacheWrite],
              ['output', request.usage.output],
              ['think', request.usage.think],
            ] as const).map(([key, value]) => (
              <div key={key} className="flex justify-between gap-3">
                <dt className="text-muted-foreground">{t(`trajectory.token.${key}`)}</dt>
                <dd className="font-mono tabular-nums">{value.toLocaleString()}</dd>
              </div>
            ))}
          </dl>
        )}
        {tab === 'timing' && (
          <dl className="space-y-2.5 px-3 py-3 text-xs">
            <div className="flex justify-between gap-3">
              <dt className="text-muted-foreground">{t('trajectory.ttft')}</dt>
              <dd className="font-mono">{ttftMs === undefined ? '—' : formatDurationMs(ttftMs)}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-muted-foreground">{t('trajectory.decoding')}</dt>
              <dd className="font-mono">{decodingMs === null ? '—' : formatDurationMs(decodingMs)}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-muted-foreground">{t('trajectory.total')}</dt>
              <dd className="font-mono">{formatDurationMs(totalMs)}</dd>
            </div>
          </dl>
        )}
      </ScrollArea>
    </>
  );
}

/** 渲染记录级详情（输入 / 输出 / 思考）。 */
function RecordDetails({
  record,
  t,
}: {
  record: TrajectoryRecord;
  t: (key: string, params?: Record<string, string | number>) => string;
}) {
  const [tab, setTab] = useState<RecordTab>('input');
  const tabs: Array<{ id: RecordTab; label: string }> = [
    { id: 'input', label: t('trajectory.input') },
    { id: 'output', label: t('trajectory.output') },
    ...(record.thinkingDetail !== undefined
      ? [{ id: 'thinking', label: t('trajectory.thinking') } as const]
      : []),
  ];
  const content: Record<RecordTab, string | undefined> = {
    input: record.inputDetail,
    output: record.outputDetail ?? record.result,
    thinking: record.thinkingDetail,
  };

  return (
    <>
      <div className="flex gap-1 border-b px-3 py-1.5" role="tablist" aria-label={t('trajectory.detailsAria')}>
        {tabs.map((item) => (
          <button
            key={item.id}
            type="button"
            role="tab"
            aria-selected={tab === item.id}
            onClick={() => setTab(item.id)}
            className={[
              'rounded px-2 py-1 text-xs',
              tab === item.id
                ? 'bg-primary/10 text-primary'
                : 'text-muted-foreground hover:bg-muted/60',
            ].join(' ')}
          >
            {item.label}
          </button>
        ))}
      </div>
      <ScrollArea className="min-h-0 flex-1">
        <pre className="whitespace-pre-wrap px-3 py-3 font-mono text-[11px] leading-5 text-foreground/90">
          {content[tab] ?? t('trajectory.detailsEmpty')}
        </pre>
      </ScrollArea>
    </>
  );
}

/** 轨迹详情侧栏（仅在有选中项时由父组件渲染）。 */
export function TrajectoryDetailPanel({
  request,
  record,
  requestTools,
  requestAssistant,
  onClose,
}: TrajectoryDetailPanelProps) {
  const { t } = useTranslation();
  const showRequest = request !== null;
  const showRecord = record !== null;
  if (!showRequest && !showRecord) return null;

  const location = showRequest
    ? `${turnLabel(request!.turn)} · ${request!.group}`
    : `${turnLabel(record!.turn)} · ${record!.group}`;

  return (
    <aside
      className="flex w-[320px] flex-none flex-col border-l bg-background"
      aria-label={t('trajectory.detailsAria')}
      data-trajectory-details=""
    >
      <div className="flex items-center justify-between gap-2 border-b px-3 py-2">
        <div className="flex min-w-0 items-center gap-2">
          {showRequest ? (
            <>
              <span className="h-2 w-2 shrink-0 rounded-full bg-primary" aria-hidden="true" />
              <span className="shrink-0 font-mono text-xs">
                {t('trajectory.request')} {request!.number}
              </span>
              {request!.purpose === 'compaction' && (
                <span className="shrink-0 text-[10px] text-muted-foreground">{t('trajectory.compaction')}</span>
              )}
            </>
          ) : (
            <span
              className={[
                'shrink-0 rounded px-1 py-0.5 font-mono text-[10px]',
                record!.kind === 'message'
                  ? 'bg-violet-500/10 text-violet-600 dark:text-violet-400'
                  : record!.kind === 'tool' || record!.kind === 'subtool'
                    ? 'bg-amber-500/10 text-amber-600 dark:text-amber-400'
                    : 'bg-slate-500/10 text-slate-600 dark:text-slate-400',
              ].join(' ')}
            >
              {TRAJECTORY_KIND_LABEL[record!.kind]}
            </span>
          )}
          <span className="truncate text-[10px] text-muted-foreground">{location}</span>
        </div>
        <button
          type="button"
          className="rounded p-1 text-muted-foreground hover:bg-muted/60 hover:text-foreground"
          aria-label={t('trajectory.detailsClose')}
          onClick={onClose}
        >
          <X className="h-3.5 w-3.5" aria-hidden="true" />
        </button>
      </div>
      {showRequest
        ? (
          <RequestDetails
            request={request!}
            tools={requestTools}
            assistant={requestAssistant}
            t={t}
          />
        )
        : <RecordDetails record={record!} t={t} />}
    </aside>
  );
}