import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { formatTokenCost, formatTokenCount, getDemoTokenUsage } from '@/lib/demo-api';
import { cn } from '@/lib/utils';
import type { DemoTokenPeriod, DemoTokenUsageSnapshot } from '@/lib/types';
import { DemoLoadError } from './DemoBanner';

const PERIODS: Array<{ id: DemoTokenPeriod; label: string }> = [
  { id: '1d', label: '1天' },
  { id: '3d', label: '3天' },
  { id: '1w', label: '1周' },
  { id: '1m', label: '1月' },
  { id: '6m', label: '6月' },
  { id: '1y', label: '1年' },
];

export function TokenStatsPanel() {
  const [period, setPeriod] = useState<DemoTokenPeriod>('1d');
  const [reloadKey, setReloadKey] = useState(0);
  const [view, setView] = useState<'overview' | 'detail'>('overview');
  const [snapshot, setSnapshot] = useState<DemoTokenUsageSnapshot | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      setError(null);
      try {
        const next = await getDemoTokenUsage(period);
        if (!cancelled) setSnapshot(next);
      } catch (cause) {
        if (!cancelled) setError(cause instanceof Error ? cause.message : String(cause));
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

  if (error) return <DemoLoadError message={error} onRetry={() => setReloadKey((value) => value + 1)} />;
  if (!snapshot) {
    return (
      <div className="space-y-3">
        <div className="h-10 animate-pulse rounded-md bg-muted" />
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          {Array.from({ length: 4 }, (_, index) => <div key={index} className="h-20 animate-pulse rounded-lg bg-muted" />)}
        </div>
      </div>
    );
  }

  const visibleSessions = view === 'overview' ? snapshot.sessions.slice(0, 5) : snapshot.sessions;

  return (
    <div className="space-y-6">
      {view === 'detail' ? (
        <Button type="button" variant="ghost" size="sm" onClick={() => setView('overview')}>返回</Button>
      ) : (
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap gap-2" role="group" aria-label="统计周期">
            {PERIODS.map((item) => (
              <Button
                key={item.id}
                type="button"
                size="sm"
                variant={period === item.id ? 'default' : 'outline'}
                aria-pressed={period === item.id}
                disabled={loading && period === item.id}
                onClick={() => setPeriod(item.id)}
              >
                {item.label}
              </Button>
            ))}
          </div>
          <Button type="button" variant="outline" size="sm" onClick={exportSnapshot} disabled={!snapshot}>导出</Button>
        </div>
      )}

      {view === 'overview' ? (
        <>
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <Metric label="总 Token" value={formatTokenCount(snapshot.total.total_tokens)} />
            <Metric label="输入" value={formatTokenCount(snapshot.total.total_input)} />
            <Metric label="输出" value={formatTokenCount(snapshot.total.total_output)} />
            <Metric label="预估费用" value={formatTokenCost(snapshot.total.total_cost)} />
          </div>

          <section>
            <h3 className="mb-3 text-sm font-semibold">模型分布</h3>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>模型</TableHead>
                  <TableHead>占比</TableHead>
                  <TableHead className="text-right">Token</TableHead>
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
            <Button type="button" variant="ghost" size="sm" onClick={() => setView('detail')}>查看详细统计</Button>
          </div>
        </>
      ) : (
        <>
          <section>
            <h3 className="mb-3 text-sm font-semibold">缓存 Token</h3>
            <div className="grid gap-3 sm:grid-cols-2">
              <Metric label="缓存创建" value={formatTokenCount(snapshot.total.total_cache_creation)} />
              <Metric label="缓存读取" value={formatTokenCount(snapshot.total.total_cache_read)} />
            </div>
          </section>
          <section>
            <h3 className="mb-3 text-sm font-semibold">端点</h3>
            <ul className="space-y-3">
              {snapshot.endpoints.map((endpoint) => {
                const share = snapshot.total.total_tokens === 0 ? 0 : (endpoint.total_tokens / snapshot.total.total_tokens) * 100;
                return (
                  <li key={endpoint.key} className="grid grid-cols-[7rem_1fr_4.5rem] items-center gap-3 text-sm">
                    <span className="font-medium">{endpoint.key}</span>
                    <Progress value={share} />
                    <span className="text-right tabular-nums">{formatTokenCount(endpoint.total_tokens)}</span>
                  </li>
                );
              })}
            </ul>
          </section>
          <SessionTable sessions={visibleSessions} columns="detail" />
        </>
      )}
    </div>
  );
}

function UsageTrendChart({ timeline }: { timeline: DemoTokenUsageSnapshot['timeline'] }) {
  const maxTimeline = Math.max(...timeline.map((point) => point.total_tokens), 1);
  const fewBars = timeline.length <= 4;
  return (
    <section>
      <h3 className="mb-3 text-sm font-semibold">使用趋势</h3>
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
                  title={`${point.label}\n输入 ${formatTokenCount(point.total_input)}\n输出 ${formatTokenCount(point.total_output)}`}
                  aria-label={`${point.label} 共 ${formatTokenCount(point.total_tokens)} Token`}
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
          <span className="h-2.5 w-2.5 rounded-sm bg-primary" aria-hidden="true" />输入
        </span>
        <span className="inline-flex items-center gap-1.5">
          <span className="h-2.5 w-2.5 rounded-sm bg-primary/40" aria-hidden="true" />输出
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
  sessions: DemoTokenUsageSnapshot['sessions'];
  columns: 'overview' | 'detail';
}) {
  return (
    <section>
      <h3 className="mb-3 text-sm font-semibold">会话明细</h3>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>会话</TableHead>
            <TableHead>模型</TableHead>
            {columns === 'overview' ? (
              <>
                <TableHead className="text-right">请求</TableHead>
                <TableHead className="text-right">Token</TableHead>
                <TableHead className="text-right">费用</TableHead>
              </>
            ) : (
              <>
                <TableHead className="text-right">输入</TableHead>
                <TableHead className="text-right">输出</TableHead>
                <TableHead className="text-right">费用</TableHead>
              </>
            )}
          </TableRow>
        </TableHeader>
        <TableBody>
          {sessions.map((session) => (
            <TableRow key={session.id}>
              <TableCell className="font-medium">{session.title}</TableCell>
              <TableCell>{session.model}</TableCell>
              {columns === 'overview' ? (
                <>
                  <TableCell className="text-right tabular-nums">{session.request_count}</TableCell>
                  <TableCell className="text-right tabular-nums">{formatTokenCount(session.total_tokens)}</TableCell>
                  <TableCell className="text-right tabular-nums">{formatTokenCost(session.total_cost)}</TableCell>
                </>
              ) : (
                <>
                  <TableCell className="text-right tabular-nums">{formatTokenCount(session.total_input)}</TableCell>
                  <TableCell className="text-right tabular-nums">{formatTokenCount(session.total_output)}</TableCell>
                  <TableCell className="text-right tabular-nums">{formatTokenCost(session.total_cost)}</TableCell>
                </>
              )}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </section>
  );
}
