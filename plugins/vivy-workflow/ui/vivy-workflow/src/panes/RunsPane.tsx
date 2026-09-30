// Runs tab: durable run list on the left, selected run's committed journal
// ledger + node states + protected outputs + cancel on the right. Live
// events arrive through the inofy.events subscription (journal authority);
// a missed stream page is refilled via the paged events RPC.

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { MasterDetail } from '@/components/layout/MasterDetail';
import { ScrollArea } from '@/components/ui/scroll-area';
import type { UITranslator } from '@vivy/ui-sdk';
import { Ban, Eye, RefreshCw } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import type { WorkflowClient } from '../client';
import type { RunDetail, RunEvent, RunSummary } from '../studio/schema';
import type { EventSubscription } from '../studio/transport';
import { TransportError } from '../studio/transport';

interface RunsPaneProps {
  client: WorkflowClient;
  t: UITranslator;
  focusRunId: string | null;
  onFocusHandled: () => void;
}

const TERMINAL = new Set(['succeeded', 'failed', 'cancelled']);

function isTerminalStatus(status: string | undefined): boolean {
  return status != null && TERMINAL.has(status);
}

function statusVariant(status: string | undefined): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (status) {
    case 'succeeded':
      return 'secondary';
    case 'failed':
    case 'recovery_required':
      return 'destructive';
    case 'cancelled':
      return 'outline';
    default:
      return 'default';
  }
}

function basename(path: string | undefined): string {
  if (!path) return '';
  const seg = path.split('/').filter(Boolean);
  return seg[seg.length - 1] ?? path;
}

interface NodeState {
  path: string;
  kind: string;
  attempt?: number;
}

function nodeStates(events: RunEvent[]): NodeState[] {
  const map = new Map<string, NodeState>();
  for (const e of events) {
    if (!e.path || !e.kind.startsWith('node_')) continue;
    const key = e.path;
    // node_attempt is an intermediate signal — keep the last terminal-ish kind.
    const prev = map.get(key);
    if (e.kind === 'node_attempt' && prev && prev.kind !== 'node_attempt') continue;
    map.set(key, { path: key, kind: e.kind, attempt: e.attempt });
  }
  return [...map.values()].sort((a, b) => a.path.localeCompare(b.path));
}

function kindVariant(kind: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  if (kind.endsWith('failed') || kind === 'node_degraded' || kind === 'run_recovery_required') return 'destructive';
  if (kind.endsWith('succeeded') || kind === 'node_completed') return 'secondary';
  if (kind === 'run_cancelled') return 'outline';
  return 'default';
}

function RunDetailView({ client, t, runId, refreshKey }: { client: WorkflowClient; t: UITranslator; runId: string; refreshKey: number }) {
  const [detail, setDetail] = useState<RunDetail | null>(null);
  const [events, setEvents] = useState<RunEvent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [output, setOutput] = useState<{ node: string; json: string } | null>(null);
  const [cancelling, setCancelling] = useState(false);
  const subRef = useRef<EventSubscription | null>(null);

  useEffect(() => {
    let alive = true;
    setEvents([]);
    setOutput(null);
    setError(null);
    const load = async () => {
      try {
        const d = await client.getRun(runId);
        if (!alive) return;
        setDetail(d);
        let page = await client.events(runId);
        if (!alive) return;
        let all = page.events;
        let cursor = all.length > 0 ? all[all.length - 1]!.seq : 0;
        for (let guard = 0; page.next_cursor != null && alive && guard < 50; guard += 1) {
          page = await client.events(runId, Number(page.next_cursor));
          if (!alive) return;
          all = [...all, ...page.events];
          if (page.events.length > 0) cursor = page.events[page.events.length - 1]!.seq;
          if (page.events.length === 0) break;
        }
        setEvents(all);
        subRef.current?.close();
        subRef.current = client.subscribeEvents(
          runId,
          cursor,
          (data) => {
            const e = data as RunEvent;
            if (typeof e.seq !== 'number') return;
            setEvents((prev) => (prev.some((x) => x.seq === e.seq) ? prev : [...prev, e]));
            if (e.kind === 'run_succeeded' || e.kind === 'run_failed' || e.kind === 'run_cancelled') {
              void client.getRun(runId).then((d2) => alive && setDetail(d2)).catch(() => undefined);
            }
          },
          () => undefined,
        );
      } catch (e) {
        if (!alive) return;
        setError(e instanceof TransportError ? `${e.code}: ${e.message}` : String(e));
      }
    };
    void load();
    return () => {
      alive = false;
      subRef.current?.close();
      subRef.current = null;
    };
  }, [client, runId, refreshKey]);

  const showOutput = useCallback(
    async (nodePath: string) => {
      try {
        const res = await client.nodeOutput(runId, nodePath);
        setOutput({ node: nodePath, json: JSON.stringify(res.output ?? null, null, 2) });
      } catch (e) {
        setOutput({ node: nodePath, json: e instanceof Error ? e.message : String(e) });
      }
    },
    [client, runId],
  );

  const cancel = useCallback(async () => {
    setCancelling(true);
    setError(null);
    try {
      await client.cancelRun(runId);
      const d = await client.getRun(runId);
      setDetail(d);
    } catch (e) {
      setError(e instanceof TransportError ? `${e.code}: ${e.message}` : String(e));
    } finally {
      setCancelling(false);
    }
  }, [client, runId]);

  const nodes = nodeStates(events);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex items-center gap-2 border-b px-3 py-2">
        <span className="truncate font-mono text-xs">{runId}</span>
        {detail ? (
          <>
            <Badge variant={statusVariant(detail.status)} className="text-[10px]">{detail.status}</Badge>
            {detail.workflow_id ? (
              <span className="truncate text-[11px] text-muted-foreground">
                {detail.workflow_id}{detail.revision != null ? `@r${detail.revision}` : ''}
              </span>
            ) : null}
          </>
        ) : null}
        <div className="ml-auto">
          <Button
            size="sm"
            variant="destructive"
            disabled={cancelling || isTerminalStatus(detail?.status)}
            onClick={() => void cancel()}
          >
            <Ban className="mr-1 h-3.5 w-3.5" />
            {t('plugin.vivy/workflow-ui.runs.cancel')}
          </Button>
        </div>
      </div>
      {error ? <div className="border-b bg-destructive/10 px-3 py-1.5 text-xs text-destructive" role="alert">{error}</div> : null}
      {detail?.waits && detail.waits.length > 0 ? (
        <div className="border-b bg-amber-500/10 px-3 py-1.5">
          <p className="text-[11px] font-medium">{t('plugin.vivy/workflow-ui.runs.waits', { count: detail.waits.length })}</p>
          {detail.waits.map((w) => (
            <p key={w.request_id} className="mt-0.5 font-mono text-[10px] text-muted-foreground">
              {w.request_id}{w.prompt ? ` — ${w.prompt}` : ''}
            </p>
          ))}
        </div>
      ) : null}
      <div className="grid min-h-0 flex-1 grid-cols-1 md:grid-cols-2">
        <div className="flex min-h-0 flex-col border-b md:border-b-0 md:border-r">
          <h4 className="border-b px-3 py-1.5 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
            {t('plugin.vivy/workflow-ui.runs.nodes')}
          </h4>
          <ScrollArea className="min-h-0 flex-1">
            {nodes.length === 0 ? (
              <p className="p-3 text-[11px] text-muted-foreground">{t('plugin.vivy/workflow-ui.runs.noNodes')}</p>
            ) : (
              <ul className="divide-y">
                {nodes.map((n) => (
                  <li key={n.path} className="flex items-center gap-2 px-3 py-1.5">
                    <span className="min-w-0 flex-1 truncate font-mono text-[11px]">{basename(n.path)}</span>
                    {n.attempt != null && n.attempt > 1 ? (
                      <span className="text-[10px] text-muted-foreground">×{n.attempt}</span>
                    ) : null}
                    <Badge variant={kindVariant(n.kind)} className="text-[10px]">{n.kind.replace(/^node_/, '')}</Badge>
                    {n.kind === 'node_completed' ? (
                      <Button size="icon" variant="ghost" className="h-5 w-5" title={t('plugin.vivy/workflow-ui.runs.output')} onClick={() => void showOutput(n.path)}>
                        <Eye className="h-3 w-3" />
                      </Button>
                    ) : null}
                  </li>
                ))}
              </ul>
            )}
          </ScrollArea>
          {output ? (
            <div className="border-t">
              <div className="flex items-center gap-2 px-3 py-1.5">
                <span className="text-[11px] font-semibold">{t('plugin.vivy/workflow-ui.runs.outputTitle')}</span>
                <span className="truncate font-mono text-[10px] text-muted-foreground">{basename(output.node)}</span>
              </div>
              <pre className="max-h-40 overflow-auto bg-muted/50 px-3 py-2 font-mono text-[10px] leading-relaxed">{output.json}</pre>
            </div>
          ) : null}
        </div>
        <div className="flex min-h-0 flex-col">
          <h4 className="border-b px-3 py-1.5 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
            {t('plugin.vivy/workflow-ui.runs.events')}
          </h4>
          <ScrollArea className="min-h-0 flex-1">
            {events.length === 0 ? (
              <p className="p-3 text-[11px] text-muted-foreground">{t('plugin.vivy/workflow-ui.runs.noEvents')}</p>
            ) : (
              <ul className="divide-y font-mono text-[10px]">
                {events.map((e) => (
                  <li key={e.seq} className="flex items-center gap-2 px-3 py-1">
                    <span className="w-8 shrink-0 text-right text-muted-foreground">#{e.seq}</span>
                    <Badge variant={kindVariant(e.kind)} className="shrink-0 text-[9px]">{e.kind}</Badge>
                    <span className="truncate text-muted-foreground">{basename(e.path)}</span>
                  </li>
                ))}
              </ul>
            )}
          </ScrollArea>
        </div>
      </div>
    </div>
  );
}

export function RunsPane({ client, t, focusRunId, onFocusHandled }: RunsPaneProps) {
  const [items, setItems] = useState<RunSummary[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  const reload = useCallback(async () => {
    setError(null);
    try {
      const page = await client.listRuns();
      setItems(page.items);
    } catch (e) {
      setError(e instanceof TransportError ? `${e.code}: ${e.message}` : String(e));
      setItems([]);
    }
  }, [client]);

  useEffect(() => {
    void reload();
  }, [reload, refreshKey]);

  useEffect(() => {
    if (!focusRunId) return;
    setSelected(focusRunId);
    setRefreshKey((k) => k + 1);
    onFocusHandled();
  }, [focusRunId, onFocusHandled]);

  const master = (
    <div className="flex h-full min-h-0 flex-col bg-sidebar">
      <div className="flex items-center gap-2 border-b px-3 py-2">
        <h3 className="text-xs font-semibold">{t('plugin.vivy/workflow-ui.runs.title')}</h3>
        <Button size="icon" variant="ghost" className="ml-auto h-6 w-6" onClick={() => setRefreshKey((k) => k + 1)} title={t('common.refresh')}>
          <RefreshCw className="h-3 w-3" />
        </Button>
      </div>
      {error ? <div className="border-b bg-destructive/10 px-3 py-1.5 text-xs text-destructive" role="alert">{error}</div> : null}
      <ScrollArea className="min-h-0 flex-1">
        {items === null ? (
          <p className="p-3 text-[11px] text-muted-foreground">{t('common.loading')}</p>
        ) : items.length === 0 ? (
          <p className="p-3 text-[11px] text-muted-foreground">{t('plugin.vivy/workflow-ui.runs.empty')}</p>
        ) : (
          <ul className="divide-y">
            {items.map((r) => (
              <li key={r.run_id}>
                <button
                  type="button"
                  className={`flex w-full items-center gap-2 px-3 py-2 text-left hover:bg-accent/50 ${selected === r.run_id ? 'bg-accent' : ''}`}
                  onClick={() => setSelected(r.run_id)}
                >
                  <div className="min-w-0 flex-1">
                    <p className="truncate font-mono text-[11px]">{r.run_id}</p>
                    <p className="truncate text-[10px] text-muted-foreground">
                      {r.workflow_id ?? ''}{r.revision != null ? `@r${r.revision}` : ''}
                    </p>
                  </div>
                  <Badge variant={statusVariant(r.status)} className="shrink-0 text-[9px]">{r.status}</Badge>
                </button>
              </li>
            ))}
          </ul>
        )}
      </ScrollArea>
    </div>
  );

  return (
    <MasterDetail
      selected={selected != null}
      onBack={() => setSelected(null)}
      master={master}
      detail={
        selected ? (
          <RunDetailView client={client} t={t} runId={selected} refreshKey={refreshKey} />
        ) : (
          <div className="flex h-full items-center justify-center p-6">
            <p className="text-sm text-muted-foreground">{t('plugin.vivy/workflow-ui.runs.selectRun')}</p>
          </div>
        )
      }
    />
  );
}
