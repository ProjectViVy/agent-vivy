// Draft editor: React Flow canvas projected from the artifact plus the
// draft lifecycle actions (open/create, save with CAS, validate, publish,
// run). Every mutation flows artifact -> toCanvas, so the definition stays
// the single source of truth and layout writes back only on real moves.

import {
  applyEdgeChanges,
  applyNodeChanges,
  Background,
  Controls,
  ReactFlow,
  type Connection,
  type EdgeChange,
  type NodeChange,
  type Node as RFNode,
  type Viewport,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import type { UITranslator } from '@vivy/ui-sdk';
import { Play, Plus, RefreshCw, Save, ShieldCheck, Upload } from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { WorkflowClient } from '../client';
import { blankArtifact, validateWorkflowID } from '../seed';
import {
  addNodeAt,
  exitOutputName,
  removeNode,
} from '../studio/edit';
import { fromCanvas, toCanvas, type Canvas, type CanvasNodeData } from '../studio/graph';
import type { ApiError, Artifact, NodeDescriptor } from '../studio/schema';
import { TransportError, type WorkflowStartRequest } from '../studio/transport';
import { NodePanel } from './NodePanel';
import { WorkflowNode } from './WorkflowNode';

const nodeTypes = { inofyNode: WorkflowNode };

interface DraftState {
  workflow: string;
  etag: string | null;
  artifact: Artifact;
  canvas: Canvas;
  dirty: boolean;
}

export interface EditTarget {
  workflow: string;
  revision?: number;
  key: number;
}

interface EditorPaneProps {
  client: WorkflowClient;
  catalog: NodeDescriptor[];
  t: UITranslator;
  canRun: boolean;
  target: EditTarget | null;
  onRunStarted: (runId: string) => void;
}

interface Diagnostics {
  items: Array<{ check?: string; path?: string; message?: string }>;
}

function diagnosticItems(raw: ApiError['diagnostics']): Diagnostics['items'] {
  if (!Array.isArray(raw)) return [];
  return raw.filter((x): x is { check?: string; path?: string; message?: string } => x != null && typeof x === 'object');
}

function shortDigest(digest: string | undefined): string {
  if (!digest) return '';
  return digest.length > 12 ? `${digest.slice(0, 12)}…` : digest;
}

export function EditorPane({ client, catalog, t, canRun, target, onRunStarted }: EditorPaneProps) {
  const [idInput, setIdInput] = useState('');
  const [draft, setDraft] = useState<DraftState | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [error, setError] = useState<TransportError | null>(null);
  const [diagnostics, setDiagnostics] = useState<Diagnostics['items'] | null>(null);
  const [busy, setBusy] = useState<'save' | 'validate' | 'publish' | 'run' | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const [pendingIntent, setPendingIntent] = useState<WorkflowStartRequest | null>(null);
  const viewportRef = useRef<Viewport | undefined>(undefined);
  const idError = idInput !== '' ? validateWorkflowID(idInput) : null;
  const catalogIds = useMemo(() => catalog.map((d) => d.type_id), [catalog]);
  const catalogById = useMemo(() => new Map(catalog.map((d) => [d.type_id, d])), [catalog]);

  // Re-projection rebuilds RF node objects without selection state, so the
  // app-level selection flag is written back into the canvas here — in state,
  // never at render time, so RF's own select change pipeline stays in charge.
  const reproject = useCallback(
    (artifact: Artifact, sel: string | null): Canvas => {
      const canvas = toCanvas(artifact, catalogIds);
      if (sel != null) {
        canvas.nodes = canvas.nodes.map((n) => (n.id === sel ? { ...n, selected: true } : n));
      }
      return canvas;
    },
    [catalogIds],
  );

  const applyArtifact = useCallback(
    (workflow: string, etag: string | null, artifact: Artifact, dirty: boolean, preserveSelection = false) => {
      const canvas = preserveSelection ? reproject(artifact, selected) : toCanvas(artifact, catalogIds);
      setDraft({ workflow, etag, artifact, canvas, dirty });
      if (!preserveSelection) setSelected(null);
      setDiagnostics(null);
      viewportRef.current = undefined;
    },
    [catalogIds, reproject, selected],
  );

  const invalidateStartIntent = useCallback(() => {
    setPendingIntent(null);
    setError(null);
  }, []);

  const openDraft = useCallback(
    async (id: string) => {
      invalidateStartIntent();
      setError(null);
      setNote(null);
      try {
        const d = await client.loadDraft(id);
        applyArtifact(id, d.etag, d.artifact, false);
      } catch (e) {
        if (e instanceof TransportError && e.status === 412) {
          applyArtifact(id, null, blankArtifact(id), false);
          setNote(t('plugin.vivy/workflow-ui.editor.newDraftNote'));
          return;
        }
        setError(e instanceof TransportError ? e : new TransportError(0, { code: 'rpc_error', message: String(e) }));
      }
    },
    [applyArtifact, client, invalidateStartIntent, t],
  );

  const openRevision = useCallback(
    async (id: string, revision: number) => {
      invalidateStartIntent();
      setError(null);
      setNote(null);
      try {
        const [r, currentETag] = await Promise.all([
          client.getRevision(id, revision),
          client.loadDraft(id)
            .then((draft) => draft.etag)
            .catch((e: unknown) => {
              if (e instanceof TransportError && e.status === 412) return null;
              throw e;
            }),
        ]);
        applyArtifact(id, currentETag, r.artifact, true);
        setNote(t('plugin.vivy/workflow-ui.editor.forkNote', { revision }));
      } catch (e) {
        setError(e instanceof TransportError ? e : new TransportError(0, { code: 'rpc_error', message: String(e) }));
      }
    },
    [applyArtifact, client, invalidateStartIntent, t],
  );

  useEffect(() => {
    if (!target) return;
    setIdInput(target.workflow);
    if (target.revision != null) void openRevision(target.workflow, target.revision);
    else void openDraft(target.workflow);
  }, [target, openDraft, openRevision]);

  const materialize = useCallback((): Artifact => {
    if (!draft) return blankArtifact('');
    const viewport = viewportRef.current ?? draft.canvas.viewport;
    return fromCanvas({ ...draft.canvas, viewport }, draft.artifact);
  }, [draft]);

  const commitSemantic = useCallback(
    (artifact: Artifact, dirty: boolean) => {
      invalidateStartIntent();
      setDraft((d) => (d ? { ...d, artifact, canvas: reproject(artifact, selected), dirty: d.dirty || dirty } : d));
    },
    [invalidateStartIntent, reproject, selected],
  );

  const onNodesChange = useCallback((changes: NodeChange<RFNode<CanvasNodeData>>[]) => {
    const structural = changes.some((c) => c.type === 'remove' || c.type === 'add');
    const moved = changes.some((c) => c.type === 'position' && c.position !== undefined);
    const removedSelected = changes.some((c) => c.type === 'remove' && c.id === selected);
    if (removedSelected) setSelected(null);
    if (structural || moved) invalidateStartIntent();
    setDraft((d) => {
      if (!d) return d;
      const removes = changes.filter((c): c is NodeChange<RFNode<CanvasNodeData>> & { type: 'remove' } => c.type === 'remove');
      if (removes.length > 0) {
        let artifact = fromCanvas(d.canvas, d.artifact);
        for (const r of removes) artifact = removeNode(artifact, r.id);
        const sel = removedSelected ? null : selected;
        return { ...d, artifact, canvas: reproject(artifact, sel), dirty: true };
      }
      const canvas = { ...d.canvas, nodes: applyNodeChanges(changes, d.canvas.nodes) };
      return { ...d, canvas, dirty: d.dirty || moved || structural };
    });
  }, [invalidateStartIntent, reproject, selected]);

  const onEdgesChange = useCallback((changes: EdgeChange[]) => {
    const structural = changes.some((c) => c.type === 'remove' || c.type === 'add');
    if (structural) invalidateStartIntent();
    setDraft((d) => (d ? { ...d, canvas: { ...d.canvas, edges: applyEdgeChanges(changes, d.canvas.edges) }, dirty: d.dirty || structural } : d));
  }, [invalidateStartIntent]);

  const onConnect = useCallback((conn: Connection) => {
    if (!conn.source || !conn.target) return;
    invalidateStartIntent();
    setDraft((d) => {
      if (!d) return d;
      const edge = {
        id: `e-${conn.source}-${conn.sourceHandle ?? ''}-${conn.target}`,
        source: conn.source,
        target: conn.target,
        ...(conn.sourceHandle ? { sourceHandle: conn.sourceHandle } : {}),
        data: {
          semantic: {
            from: conn.source,
            to: conn.target,
            ...(conn.sourceHandle ? { port: conn.sourceHandle } : {}),
          },
        },
      };
      return { ...d, canvas: { ...d.canvas, edges: [...d.canvas.edges, edge] }, dirty: true };
    });
  }, [invalidateStartIntent]);

  const addNode = useCallback((typeID: string) => {
    invalidateStartIntent();
    setDraft((d) => {
      if (!d) return d;
      const artifact = fromCanvas(d.canvas, d.artifact);
      const count = artifact.definition.graph.nodes.length;
      const pos = { x: 80 + (count % 4) * 220, y: 60 + Math.floor(count / 4) * 120 };
      const { artifact: next } = addNodeAt(artifact, typeID, pos);
      return { ...d, artifact: next, canvas: reproject(next, selected), dirty: true };
    });
  }, [invalidateStartIntent, reproject, selected]);

  const patchSelected = useCallback(
    (artifact: Artifact) => {
      commitSemantic(artifact, true);
    },
    [commitSemantic],
  );

  const deleteNode = useCallback(
    (id: string) => {
      if (!draft) return;
      invalidateStartIntent();
      const artifact = removeNode(fromCanvas(draft.canvas, draft.artifact), id);
      if (selected === id) setSelected(null);
      setDraft((d) => (d ? { ...d, artifact, canvas: reproject(artifact, selected === id ? null : selected), dirty: true } : d));
    },
    [draft, invalidateStartIntent, reproject, selected],
  );

  const save = useCallback(async () => {
    if (!draft || !draft.workflow) return;
    setBusy('save');
    setError(null);
    try {
      const artifact = materialize();
      const saved = await client.saveDraft(draft.workflow, artifact, draft.etag);
      applyArtifact(draft.workflow, saved.etag, saved.artifact, false, true);
      setNote(t('plugin.vivy/workflow-ui.editor.saved'));
    } catch (e) {
      setError(e instanceof TransportError ? e : new TransportError(0, { code: 'rpc_error', message: String(e) }));
    } finally {
      setBusy(null);
    }
  }, [applyArtifact, client, draft, materialize, t]);

  const validate = useCallback(async () => {
    if (!draft) return;
    setBusy('validate');
    setError(null);
    try {
      const artifact = materialize();
      const saved = await client.saveDraft(draft.workflow, artifact, draft.etag);
      applyArtifact(draft.workflow, saved.etag, saved.artifact, false, true);
      const res = await client.validate(draft.workflow, saved.etag);
      setDiagnostics(diagnosticItems(res.diagnostics));
      setNote(res.valid === false ? null : t('plugin.vivy/workflow-ui.editor.valid'));
    } catch (e) {
      const err = e instanceof TransportError ? e : new TransportError(0, { code: 'rpc_error', message: String(e) });
      setError(err);
      setDiagnostics(diagnosticItems(err.diagnostics));
    } finally {
      setBusy(null);
    }
  }, [applyArtifact, client, draft, materialize, t]);

  const publish = useCallback(async () => {
    if (!draft) return;
    setBusy('publish');
    setError(null);
    try {
      const artifact = materialize();
      const saved = await client.saveDraft(draft.workflow, artifact, draft.etag);
      applyArtifact(draft.workflow, saved.etag, saved.artifact, false, true);
      const res = await client.publish(draft.workflow, saved.etag);
      setNote(t('plugin.vivy/workflow-ui.editor.published', { revision: res.revision, digest: shortDigest(res.definition_digest) }));
    } catch (e) {
      const err = e instanceof TransportError ? e : new TransportError(0, { code: 'rpc_error', message: String(e) });
      setError(err);
      setDiagnostics(diagnosticItems(err.diagnostics));
    } finally {
      setBusy(null);
    }
  }, [applyArtifact, client, draft, materialize, t]);

  const run = useCallback(async () => {
    if (!draft) return;
    setBusy('run');
    setError(null);
    let request = pendingIntent;
    try {
      if (!request) {
        const artifact = materialize();
        const saved = await client.saveDraft(draft.workflow, artifact, draft.etag);
        applyArtifact(draft.workflow, saved.etag, saved.artifact, false, true);
        request = client.prepareStartRun({ workflow: draft.workflow, draft_etag: saved.etag });
        setPendingIntent(request);
      }
      const res = await client.startRun(request);
      setPendingIntent(null);
      onRunStarted(res.run_id);
    } catch (e) {
      const err = e instanceof TransportError ? e : new TransportError(0, { code: 'rpc_error', message: String(e) });
      setError(err);
      setDiagnostics(diagnosticItems(err.diagnostics));
    } finally {
      setBusy(null);
    }
  }, [applyArtifact, client, draft, materialize, onRunStarted, pendingIntent]);

  const conflict = error?.status === 412;
  const selectedNode = useMemo(() => {
    if (!draft || !selected) return undefined;
    return draft.canvas.nodes.find((n) => n.id === selected)?.data.node;
  }, [draft, selected]);
  const selectedDescriptor = selectedNode ? catalogById.get(selectedNode.type ?? '') : undefined;

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex items-center gap-2 border-b px-3 py-2">
        <Input
          value={idInput}
          onChange={(e) => setIdInput(e.target.value)}
          placeholder={t('plugin.vivy/workflow-ui.editor.idPlaceholder')}
          className="h-8 w-64 text-xs font-mono"
          spellCheck={false}
        />
        <Button size="sm" variant="outline" disabled={!idInput || !!idError || busy !== null} onClick={() => void openDraft(idInput)}>
          {t('common.open')}
        </Button>
        <div className="ml-auto flex items-center gap-1.5">
          {draft?.dirty ? <Badge variant="outline" className="text-[10px]">{t('plugin.vivy/workflow-ui.editor.dirty')}</Badge> : null}
          {catalog.length > 0 ? (
            <Button size="sm" variant="outline" disabled={!draft} onClick={() => addNode(catalog[0]!.type_id)}>
              <Plus className="mr-1 h-3.5 w-3.5" />
              {catalog[0]!.display?.title ?? catalog[0]!.type_id}
            </Button>
          ) : null}
          <Button size="sm" variant="outline" disabled={!draft || busy !== null} onClick={() => void save()}>
            <Save className="mr-1 h-3.5 w-3.5" />
            {t('plugin.vivy/workflow-ui.editor.save')}
          </Button>
          <Button size="sm" variant="outline" disabled={!draft || busy !== null} onClick={() => void validate()}>
            <ShieldCheck className="mr-1 h-3.5 w-3.5" />
            {t('plugin.vivy/workflow-ui.editor.validate')}
          </Button>
          <Button size="sm" variant="outline" disabled={!draft || busy !== null} onClick={() => void publish()}>
            <Upload className="mr-1 h-3.5 w-3.5" />
            {t('plugin.vivy/workflow-ui.editor.publish')}
          </Button>
          <Button
            size="sm"
            disabled={!draft || busy !== null || !canRun}
            title={canRun ? undefined : t('plugin.vivy/workflow-ui.editor.runDisabled')}
            onClick={() => void run()}
          >
            <Play className="mr-1 h-3.5 w-3.5" />
            {t('plugin.vivy/workflow-ui.editor.run')}
          </Button>
        </div>
      </div>
      {idError ? <p className="border-b px-3 py-1 text-[11px] text-destructive">{t(`plugin.vivy/workflow-ui.editor.idError.${idError}`)}</p> : null}
      {error ? (
        <div className="flex items-center gap-2 border-b bg-destructive/10 px-3 py-1.5 text-xs text-destructive" role="alert">
          <span className="truncate">
            {pendingIntent && error.code === 'revision_conflict'
              ? t('plugin.vivy/workflow-ui.editor.startSourceConflict')
              : `${error.code}: ${error.message}`}
          </span>
          {pendingIntent ? (
            <Button size="sm" variant="outline" className="ml-auto h-6 px-2 text-[11px]" disabled={busy !== null} onClick={() => void run()}>
              <RefreshCw className="mr-1 h-3 w-3" />
              {t('plugin.vivy/workflow-ui.editor.retryStart')}
            </Button>
          ) : conflict ? (
            <Button size="sm" variant="outline" className="ml-auto h-6 px-2 text-[11px]" onClick={() => void openDraft(idInput)}>
              <RefreshCw className="mr-1 h-3 w-3" />
              {t('plugin.vivy/workflow-ui.editor.reload')}
            </Button>
          ) : null}
        </div>
      ) : null}
      {note ? <p className="border-b bg-muted/50 px-3 py-1.5 text-[11px] text-muted-foreground">{note}</p> : null}
      {diagnostics && diagnostics.length > 0 ? (
        <div className="max-h-32 overflow-auto border-b bg-amber-500/10 px-3 py-1.5">
          <ul className="space-y-0.5 text-[11px]">
            {diagnostics.map((d, i) => (
              <li key={i} className="font-mono">
                {d.check ? <span className="text-muted-foreground">[{d.check}] </span> : null}
                {d.path ? <span className="text-muted-foreground">{d.path}: </span> : null}
                {d.message ?? JSON.stringify(d)}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
      <div className="flex min-h-0 flex-1">
        <div className="min-w-0 flex-1">
          {draft ? (
            <ReactFlow
              nodes={draft.canvas.nodes}
              edges={draft.canvas.edges}
              nodeTypes={nodeTypes}
              defaultViewport={draft.canvas.viewport}
              onNodesChange={onNodesChange}
              onEdgesChange={onEdgesChange}
              onConnect={onConnect}
              onSelectionChange={({ nodes }) => setSelected(nodes[0]?.id ?? null)}
              onMoveEnd={(_, vp) => {
                viewportRef.current = vp;
                invalidateStartIntent();
                setDraft((d) => (d ? { ...d, canvas: { ...d.canvas, viewport: vp }, dirty: true } : d));
              }}
              deleteKeyCode={['Backspace', 'Delete']}
              fitView={draft.canvas.viewport == null}
              proOptions={{ hideAttribution: true }}
            >
              <Background gap={16} size={1} />
              <Controls showInteractive={false} />
            </ReactFlow>
          ) : (
            <div className="flex h-full items-center justify-center">
              <p className="text-sm text-muted-foreground">{t('plugin.vivy/workflow-ui.editor.empty')}</p>
            </div>
          )}
        </div>
        {selectedNode && draft ? (
          <div className="w-80 shrink-0 border-l">
            <NodePanel
              artifact={fromCanvas(draft.canvas, draft.artifact)}
              node={selectedNode}
              descriptor={selectedDescriptor}
              t={t}
              onChange={patchSelected}
              onDelete={deleteNode}
            />
          </div>
        ) : draft ? (
          <div className="w-80 shrink-0 border-l p-3">
            <p className="text-[11px] text-muted-foreground">{t('plugin.vivy/workflow-ui.editor.selectNode')}</p>
          </div>
        ) : null}
      </div>
    </div>
  );
}
