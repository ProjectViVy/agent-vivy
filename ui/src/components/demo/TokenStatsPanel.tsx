import { useEffect, useRef, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { getTokenUsage, type TokenUsagePeriod, type TokenUsageSnapshot } from '@/lib/api';
import { formatTokenCount } from '@/lib/format';
import { useTranslation } from '@/i18n';
import { cn } from '@/lib/utils';
import { DemoLoadError } from './DemoBanner';

const PERIODS: TokenUsagePeriod[] = ['1d', '3d', '1w', '1m', '6m', '1y'];

export function TokenStatsPanel() {
  const { t } = useTranslation();
  const [period, setPeriod] = useState<TokenUsagePeriod>('1d');
  const [reloadKey, setReloadKey] = useState(0);
  const [view, setView] = useState<'overview' | 'detail'>('overview');
  const [snapshot, setSnapshot] = useState<TokenUsageSnapshot | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // Track whether we have ever loaded successfully so we can show empty vs skeleton.
  const hasLoaded = useRef(false);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      try {
        const next = await getTokenUsage({
          period,
          tz_offset_minutes: new Date().getTimezoneOffset(),
        });
        if (!cancelled) {
          setSnapshot(next);
          setError(null);
          hasLoaded.current = true;
        }
      } catch (cause) {
        if (!cancelled) {
          setError(cause instanceof Error ? cause.message : String(cause));
          // Keep old snapshot on error per scope-and-state-integrity contract.
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    void load();
    return () => { cancelled = true; };
  }, [period, reloadKey]);

  const exportSnapshot = () => {
    if (!snapshot) return;
    const blob = new Blob([JSON.stringify(snapshot, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `token-stats-${snapshot.period}.json`;
    link.click();
    URL.revokeObjectURL(url);
  };

  // Error with no prior snapshot → show error state.
  if (error && !hasLoaded.current) {
    return <DemoLoadError message={error} onRetry={() => setReloadKey((v) => v + 1)} />;
  }

  // First load skeleton.
  if (!snapshot && !hasLoaded.current) {
    return (
      <div className="space-y-3">
        <div className="h-10 animate-pulse rounded-md bg-muted" />
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          {Array.from({ length: 4 }, (_, i) => <div key={i} className="h-20 animate-pulse rounded-lg bg-muted" />)}
        </div>
      </div>
    );
  }

  // Empty state: real data loaded but no usage events.
  const isEmpty = snapshot?.total.request_count === 0;

  const visibleSessions = view === 'overview' && snapshot ? snapshot.sessions.slice(0, 5) : snapshot?.sessions ?? [];

  return (
    <div className="space-y-6">
      {view === 'detail' ? (
        <Button type="button" variant="ghost" size="sm" onClick={() => setView('overview')}>{t('common.back')}</Button>
      ) : (
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap gap-2" role="group" aria-label={t('token.periodGroupLabel')}>
            {PERIODS.map((item) => (
              <Button
                key={item}
                type="button"
                size="sm"
                variant={period === item ? 'default' : 'outline'}
                aria-pressed={period === item}
                disabled={loading && period === item}
                onClick={() => setPeriod(item)}
              >
                {t(`token.periods.${item}`)}
              </Button>
            ))}
          </div>
          <Button type="button" variant="outline" size="sm" onClick={exportSnapshot} disabled={!snapshot || isEmpty}>{t('token.export')}</Button>
        </div>
      )}

      {/* Inline error banner when we have stale data to show underneath. */}
      {error && hasLoaded.current && (
        <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
          {error}
        </div>
      )}

      {isEmpty ? (
        <div className="rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground">
          {t('token.emptyState')}
        </div>
      ) : snapshot ? (
        view === 'overview' ? (
          <>
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              <Metric label={t('token.totalTokens')} value={formatTokenCount(snapshot.total.total_tokens)} />
              <Metric label={t('token.input')} value={formatTokenCount(snapshot.total.total_input)} />
              <Metric label={t('token.output')} value={formatTokenCount(snapshot.total.total_output)} />
              <Metric label={t('token.requestCount')} value={String(snapshot.total.request_count)} />
            </div>

            <section>
              <h3 className="mb-3 text-sm font-semibold">{t('token.modelDistribution')}</h3>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('token.model')}</TableHead>
                    <TableHead>{t('token.share')}</TableHead>
                    <TableHead className="text-right">{t('token.tokenColumn')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {snapshot.models.map((model) => (
                    <TableRow key={model.model}>
                      <TableCell className="font-medium">{model.model}</TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <Progress value={model.percentage} className="h-2 w-24" />
                          <span>{model.percentage.toFixed(1)}%</span>
                        </div>
                      </TableCell>
                      <TableCell className="text-right tabular-nums">{formatTokenCount(model.total_tokens)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </section>

            <UsageTrendChart timeline={snapshot.timeline} />

            <SessionTable sessions={visibleSessions} columns="overview" />
            <div>
              <Button type="button" variant="ghost" size="sm" onClick={() => setView('detail')}>{t('token.viewDetails')}</Button>
            </div>
          </>
        ) : (
          <>
            <section>
              <h3 className="mb-3 text-sm font-semibold">{t('token.reasoningTokens')}</h3>
              <Metric label={t('token.totalReasoning')} value={formatTokenCount(snapshot.total.total_reasoning)} />
            </section>
            <section>
              <h3 className="mb-3 text-sm font-semibold">{t('token.providers')}</h3>
              <ul className="space-y-3">
                {snapshot.providers.map((prov) => {
                  const share = snapshot.total.total_tokens === 0 ? 0 : (prov.total_tokens / snapshot.total.total_tokens) * 100;
                  return (
                    <li key={prov.key} className="grid grid-cols-[7rem_1fr_4.5rem] items-center gap-3 text-sm">
                      <span className="font-medium">{prov.key}</span>
                      <Progress value={share} />
                      <span className="text-right tabular-nums">{formatTokenCount(prov.total_tokens)}</span>
                    </li>
                  );
                })}
              </ul>
            </section>
            <SessionTable sessions={visibleSessions} columns="detail" />
          </>
        )
      ) : null}
    </div>
  );
}

function UsageTrendChart({ timeline }: { timeline: TokenUsageSnapshot['timeline'] }) {
  const { t } = useTranslation();
  const maxTimeline = Math.max(...timeline.map((p) => p.total_tokens), 1);
  const fewBars = timeline.length <= 4;
  return (
    <section>
      <h3 className="mb-3 text-sm font-semibold">{t('token.usageTrend')}</h3>
      <div className="rounded-lg bg-muted/50 px-3 pb-3 pt-4">
        <div className={cn('flex h-40 items-end gap-2 border-b border-border', fewBars && 'justify-center')}>
          {timeline.map((point) => {
            const height = Math.max(6, (point.total_tokens / maxTimeline) * 100);
            const outputShare = point.total_tokens === 0 ? 0 : (point.total_output / point.total_tokens) * 100;
            const inputShare = point.total_tokens === 0 ? 0 : (point.total_input / point.total_tokens) * 100;
            return (
              <div key={point.time_bucket} className={cn('flex h-full min-w-1.5 flex-1 items-end', fewBars && 'max-w-20')}>
                <div
                  className="flex w-full flex-col justify-end overflow-hidden rounded-t-lg transition-opacity hover:opacity-80"
                  style={{ height: `${height}%` }}
                  title={t('token.trendTooltip', { label: point.label, input: formatTokenCount(point.total_input), output: formatTokenCount(point.total_output) })}
                  aria-label={t('token.trendAria', { label: point.label, total: formatTokenCount(point.total_tokens) })}
                >
                  <div className="w-full bg-primary/40" style={{ height: `${outputShare}%` }} />
                  <div className="w-full bg-primary" style={{ height: `${inputShare}%` }} />
                </div>
              </div>
            );
          })}
        </div>
        <div className="mt-2 flex justify-between text-xs text-muted-foreground">
          <span>{timeline[0]?.label}</span>
          <span>{timeline[timeline.length - 1]?.label}</span>
        </div>
      </div>
      <div className="mt-3 flex gap-4 text-xs text-muted-foreground">
        <span className="inline-flex items-center gap-1.5">
          <span className="h-2.5 w-2.5 rounded-sm bg-primary" aria-hidden="true" />{t('token.input')}
        </span>
        <span className="inline-flex items-center gap-1.5">
          <span className="h-2.5 w-2.5 rounded-sm bg-primary/40" aria-hidden="true" />{t('token.output')}
        </span>
      </div>
    </section>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border p-4">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 text-lg font-semibold tabular-nums">{value}</p>
    </div>
  );
}

function SessionTable({
  sessions,
  columns,
}: {
  sessions: TokenUsageSnapshot['sessions'];
  columns: 'overview' | 'detail';
}) {
  const { t } = useTranslation();
  return (
    <section>
      <h3 className="mb-3 text-sm font-semibold">{t('token.sessionDetails')}</h3>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('token.session')}</TableHead>
            <TableHead>{t('token.model')}</TableHead>
            {columns === 'overview' ? (
              <>
                <TableHead className="text-right">{t('token.requests')}</TableHead>
                <TableHead className="text-right">{t('token.tokenColumn')}</TableHead>
              </>
            ) : (
              <>
                <TableHead className="text-right">{t('token.input')}</TableHead>
                <TableHead className="text-right">{t('token.output')}</TableHead>
              </>
            )}
          </TableRow>
        </TableHeader>
        <TableBody>
          {sessions.map((session) => (
            <TableRow key={session.id}>
              <TableCell className="font-medium">{session.title || session.id}</TableCell>
              <TableCell>{session.model}</TableCell>
              {columns === 'overview' ? (
                <>
                  <TableCell className="text-right tabular-nums">{session.request_count}</TableCell>
                  <TableCell className="text-right tabular-nums">{formatTokenCount(session.total_tokens)}</TableCell>
                </>
              ) : (
                <>
                  <TableCell className="text-right tabular-nums">{formatTokenCount(session.total_input)}</TableCell>
                  <TableCell className="text-right tabular-nums">{formatTokenCount(session.total_output)}</TableCell>
                </>
              )}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </section>
  );
}
