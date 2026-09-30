/**
 * Workflow Module page — a VIVY-native INOFY definition editor. Rendered
 * inline in the host DOM like every other Module page: host UI kit
 * components + Tailwind tokens provide the styling, the module catalog the
 * copy, and FaceBridge the session-bound `inofy.*` host actions.
 *
 * Three tabs: Editor (graph canvas + draft lifecycle), Workflows (published
 * revisions + host capabilities), Runs (durable runs with live journal
 * ledger, node status, protected outputs, cancel).
 */
import {
  type FaceClientStore,
  type FaceStoreState,
  type FullUIHost,
  type UITranslator,
} from '@vivy/ui-sdk';
import { usePluginHost, usePluginTranslation } from '@vivy/ui-sdk';
import { useCallback, useEffect, useMemo, useState, useSyncExternalStore } from 'react';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { WorkflowClient } from './client';
import { FaceBridge } from './face-bridge';
import { EditorPane, type EditTarget } from './editor/EditorPane';
import { RunsPane } from './panes/RunsPane';
import { WorkflowsPane } from './panes/WorkflowsPane';
import type { NodeDescriptor } from './studio/schema';

const EMPTY_FACE_STATE = { activeSessionId: null, currentRun: null, connection: 'idle' } as unknown as FaceStoreState;

function useFaceState(host?: FullUIHost): FaceStoreState {
  const subscribe = useCallback((listener: () => void) => {
    if (!host) return () => undefined;
    return host.store.subscribe(() => listener());
  }, [host]);
  const getSnapshot = useCallback(() => host?.store.getState() ?? EMPTY_FACE_STATE, [host]);
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

function Page({ host, client, t }: { host: FullUIHost; client: WorkflowClient; t: UITranslator }) {
  const faceState = useFaceState(host);
  const [tab, setTab] = useState<'editor' | 'workflows' | 'runs'>('editor');
  const [catalog, setCatalog] = useState<NodeDescriptor[]>([]);
  const [catalogError, setCatalogError] = useState<string | null>(null);
  const [editTarget, setEditTarget] = useState<EditTarget | null>(null);
  const [focusRun, setFocusRun] = useState<string | null>(null);
  const canRun = faceState.currentRun != null;

  useEffect(() => {
    let alive = true;
    client
      .nodeTypes()
      .then((types) => {
        if (alive) setCatalog(types);
      })
      .catch((e: unknown) => {
        if (alive) setCatalogError(e instanceof Error ? e.message : String(e));
      });
    return () => {
      alive = false;
    };
  }, [client]);

  const openDraft = useCallback((workflow: string) => {
    setEditTarget({ workflow, key: Date.now() });
    setTab('editor');
  }, []);
  const openRevision = useCallback((workflow: string, revision: number) => {
    setEditTarget({ workflow, revision, key: Date.now() });
    setTab('editor');
  }, []);
  const openRun = useCallback((runId: string) => {
    setFocusRun(runId);
    setTab('runs');
  }, []);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <Tabs value={tab} onValueChange={(v) => setTab(v as typeof tab)} className="flex h-full min-h-0 flex-col">
        <div className="flex items-center gap-2 border-b px-3 py-2">
          <TabsList className="h-8">
            <TabsTrigger value="editor" className="text-xs">{t('plugin.vivy/workflow-ui.tab.editor')}</TabsTrigger>
            <TabsTrigger value="workflows" className="text-xs">{t('plugin.vivy/workflow-ui.tab.workflows')}</TabsTrigger>
            <TabsTrigger value="runs" className="text-xs">{t('plugin.vivy/workflow-ui.tab.runs')}</TabsTrigger>
          </TabsList>
          {catalogError ? (
            <span className="ml-auto text-[11px] text-destructive">{catalogError}</span>
          ) : null}
        </div>
        <TabsContent value="editor" className="mt-0 min-h-0 flex-1">
          <EditorPane
            client={client}
            catalog={catalog}
            t={t}
            canRun={canRun}
            target={editTarget}
            onRunStarted={openRun}
          />
        </TabsContent>
        <TabsContent value="workflows" className="mt-0 min-h-0 flex-1">
          <WorkflowsPane
            client={client}
            t={t}
            canRun={canRun}
            onOpenDraft={openDraft}
            onOpenRevision={openRevision}
            onRunStarted={openRun}
          />
        </TabsContent>
        <TabsContent value="runs" className="mt-0 min-h-0 flex-1">
          <RunsPane client={client} t={t} focusRunId={focusRun} onFocusHandled={() => setFocusRun(null)} />
        </TabsContent>
      </Tabs>
    </div>
  );
}

export function WorkflowPage() {
  const host = usePluginHost();
  const { t } = usePluginTranslation();
  const client = useMemo(
    () => (host ? new WorkflowClient(new FaceBridge(host.rpc, host.store as FaceClientStore<FaceStoreState>)) : null),
    [host],
  );
  if (!host || !client) return null;
  return <Page host={host} client={client} t={t} />;
}
