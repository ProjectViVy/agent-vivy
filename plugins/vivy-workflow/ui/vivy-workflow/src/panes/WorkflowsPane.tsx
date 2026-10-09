// Published workflow list: each row is one revision with its definition
// digest, openable as a fresh draft for the next revision or runnable as
// published. The footer surfaces host capabilities and provider connections
// read-only — what the backend declares, nothing more.

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import type { UITranslator } from '@vivy/ui-sdk';
import { FilePlus, Play, RefreshCw } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import type { WorkflowClient } from '../client';
import type { ConnectionView, WorkflowSummary } from '../studio/schema';
import type { WorkflowStartRequest } from '../studio/transport';
import { TransportError } from '../studio/transport';

interface WorkflowsPaneProps {
  client: WorkflowClient;
  t: UITranslator;
  canRun: boolean;
  onOpenDraft: (workflow: string) => void;
  onOpenRevision: (workflow: string, revision: number) => void;
  onRunStarted: (runId: string) => void;
}

export function WorkflowsPane({ client, t, canRun, onOpenDraft, onOpenRevision, onRunStarted }: WorkflowsPaneProps) {
  const [items, setItems] = useState<WorkflowSummary[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [capabilities, setCapabilities] = useState<Record<string, unknown> | null>(null);
  const [connections, setConnections] = useState<ConnectionView[] | null>(null);
  const [newId, setNewId] = useState('');
  const [runningKey, setRunningKey] = useState<string | null>(null);
  const [pendingStarts, setPendingStarts] = useState<Record<string, WorkflowStartRequest>>({});

  const reload = useCallback(async () => {
    setError(null);
    try {
      const page = await client.listWorkflows();
      setItems(page.items);
    } catch (e) {
      setError(e instanceof TransportError ? `${e.code}: ${e.message}` : String(e));
      setItems([]);
    }
    try {
      setCapabilities(await client.capabilities());
    } catch {
      setCapabilities(null);
    }
    try {
      setConnections(await client.listConnections());
    } catch {
      setConnections(null);
    }
  }, [client]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const run = useCallback(
    async (w: WorkflowSummary) => {
      const key = `${w.workflow_id}@${w.revision ?? '?'}`;
      setRunningKey(key);
      setError(null);
      let request = pendingStarts[key];
      try {
        if (!request) {
          if (w.revision == null) throw new TypeError('published revision is missing');
          request = client.prepareStartRun({ workflow: w.workflow_id, revision: w.revision });
          setPendingStarts((current) => ({ ...current, [key]: request! }));
        }
        const res = await client.startRun(request);
        setPendingStarts((current) => {
          const next = { ...current };
          delete next[key];
          return next;
        });
        onRunStarted(res.run_id);
      } catch (e) {
        setError(e instanceof TransportError && request && e.code === 'revision_conflict'
          ? t('plugin.vivy/workflow-ui.editor.startSourceConflict')
          : e instanceof TransportError ? `${e.code}: ${e.message}` : String(e));
      } finally {
        setRunningKey(null);
      }
    },
    [client, onRunStarted, pendingStarts, t],
  );

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex items-center gap-2 border-b px-3 py-2">
        <Input
          value={newId}
          onChange={(e) => setNewId(e.target.value)}
          placeholder={t('plugin.vivy/workflow-ui.workflows.newPlaceholder')}
          className="h-8 w-64 text-xs font-mono"
          spellCheck={false}
        />
        <Button size="sm" variant="outline" disabled={newId.trim() === ''} onClick={() => onOpenDraft(newId.trim())}>
          <FilePlus className="mr-1 h-3.5 w-3.5" />
          {t('plugin.vivy/workflow-ui.workflows.new')}
        </Button>
        <Button size="icon" variant="ghost" className="ml-auto h-7 w-7" onClick={() => void reload()} title={t('common.refresh')}>
          <RefreshCw className="h-3.5 w-3.5" />
        </Button>
      </div>
      {error ? (
        <div className="border-b bg-destructive/10 px-3 py-1.5 text-xs text-destructive" role="alert">{error}</div>
      ) : null}
      <ScrollArea className="min-h-0 flex-1">
        {items === null ? (
          <p className="p-4 text-sm text-muted-foreground">{t('common.loading')}</p>
        ) : items.length === 0 ? (
          <p className="p-4 text-sm text-muted-foreground">{t('plugin.vivy/workflow-ui.workflows.empty')}</p>
        ) : (
          <ul className="divide-y">
            {items.map((w) => (
              <li key={`${w.workflow_id}@${w.revision ?? '?'}`} className="flex items-center gap-3 px-4 py-2.5">
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="truncate font-mono text-xs font-medium">{w.workflow_id}</span>
                    <Badge variant="secondary" className="text-[10px]">r{w.revision}</Badge>
                  </div>
                </div>
                <Button size="sm" variant="outline" onClick={() => w.revision != null && onOpenRevision(w.workflow_id, w.revision)}>
                  {t('plugin.vivy/workflow-ui.workflows.openDraft')}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={!canRun || runningKey === `${w.workflow_id}@${w.revision ?? '?'}`}
                  title={canRun ? undefined : t('plugin.vivy/workflow-ui.editor.runDisabled')}
                  onClick={() => void run(w)}
                >
                  {pendingStarts[`${w.workflow_id}@${w.revision ?? '?'}`]
                    ? <RefreshCw className="mr-1 h-3.5 w-3.5" />
                    : <Play className="mr-1 h-3.5 w-3.5" />}
                  {t(pendingStarts[`${w.workflow_id}@${w.revision ?? '?'}`]
                    ? 'plugin.vivy/workflow-ui.editor.retryStart'
                    : 'plugin.vivy/workflow-ui.editor.run')}
                </Button>
              </li>
            ))}
          </ul>
        )}
        {capabilities ? (
          <>
            <Separator />
            <div className="px-4 py-3">
              <h4 className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
                {t('plugin.vivy/workflow-ui.workflows.capabilities')}
              </h4>
              <pre className="mt-1 overflow-auto rounded-md bg-muted/50 p-2 font-mono text-[10px] leading-relaxed">
                {JSON.stringify(capabilities, null, 2)}
              </pre>
            </div>
          </>
        ) : null}
        {connections && connections.length > 0 ? (
          <div className="px-4 pb-3">
            <h4 className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
              {t('plugin.vivy/workflow-ui.workflows.connections')}
            </h4>
            <ul className="mt-1 space-y-1">
              {connections.map((c) => (
                <li key={c.id} className="flex items-center gap-2 text-[11px]">
                  <span className="font-mono">{c.id}</span>
                  <Badge variant="outline" className="text-[10px]">{c.kind}</Badge>
                  <span className="text-muted-foreground">{c.model}</span>
                </li>
              ))}
            </ul>
          </div>
        ) : null}
      </ScrollArea>
    </div>
  );
}
