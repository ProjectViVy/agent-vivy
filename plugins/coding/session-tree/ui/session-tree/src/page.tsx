/**
 * Session tree page - assembled by the vivy/session-tree Module.
 *
 * The page renders the kernel's session/tree read model and drives the
 * session/clone, session/import, session/export, and exports/read verbs
 * through the face API. Export downloads are digest-bound: the browser
 * verifies the received bytes against the sha256 the export result
 * reported before a blob URL is ever created.
 */
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Download, GitFork, RefreshCw, Upload } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { usePluginHost, usePluginTranslation } from '@vivy/ui-sdk';
import { decodeBase64, flattenTree, sha256Hex, type TreeRow } from './tree';

const CHAT_ROUTE = '/';

type Phase = 'loading' | 'ready' | 'error';

export function SessionTreePage() {
  const host = usePluginHost();
  const { t } = usePluginTranslation();
  const [rows, setRows] = useState<TreeRow[]>([]);
  const [phase, setPhase] = useState<Phase>('loading');
  const [error, setError] = useState('');
  const [selectedId, setSelectedId] = useState('');
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState('');
  const fileInput = useRef<HTMLInputElement | null>(null);
  const [activeId, setActiveId] = useState(host?.store.getState().activeSessionId ?? '');

  useEffect(() => {
    if (!host) return undefined;
    return host.store.subscribe((state) => setActiveId(state.activeSessionId ?? ''));
  }, [host]);

  const load = useCallback(async () => {
    if (!host) return;
    setPhase('loading');
    setError('');
    try {
      const tree = await host.api.sessionTree();
      setRows(flattenTree(tree));
      setPhase('ready');
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setPhase('error');
    }
  }, [host]);

  useEffect(() => { void load(); }, [load]);

  const openSession = useCallback(async (id: string) => {
    if (!host || !id) return;
    await host.store.getState().selectSession(id);
    await host.router.navigate({ to: CHAT_ROUTE });
  }, [host]);

  const cloneSelected = useCallback(async () => {
    if (!host || !selectedId) return;
    setBusy(true);
    setNotice('');
    try {
      const result = await host.api.cloneSession(selectedId);
      await openSession(result.session_id);
    } catch (err) {
      setNotice(t('plugin.vivy/session-tree.actionFailed'));
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }, [host, selectedId, openSession, t]);

  const exportSelected = useCallback(async () => {
    if (!host || !selectedId) return;
    setBusy(true);
    setNotice('');
    setError('');
    try {
      const result = await host.api.exportSession(selectedId);
      const read = await host.api.readExport(result.name, result.sha256);
      const bytes = decodeBase64(read.data_base64);
      const actual = await sha256Hex(bytes);
      if (actual !== result.sha256 || actual !== read.digest) {
        setNotice(t('plugin.vivy/session-tree.verifyFailed'));
        return;
      }
      const blob = new Blob([bytes.slice().buffer], { type: 'text/html' });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = result.name;
      anchor.click();
      URL.revokeObjectURL(url);
      setNotice(t('plugin.vivy/session-tree.exported', { name: result.name }));
    } catch (err) {
      setNotice(t('plugin.vivy/session-tree.actionFailed'));
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }, [host, selectedId, t]);

  const importFile = useCallback(async (file: File) => {
    if (!host) return;
    setBusy(true);
    setNotice('');
    setError('');
    try {
      const data = await file.text();
      const result = await host.api.importSession(data);
      setNotice(t('plugin.vivy/session-tree.imported', { imported: result.imported, skipped: result.skipped }));
      await openSession(result.session_id);
    } catch (err) {
      setNotice(t('plugin.vivy/session-tree.actionFailed'));
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }, [host, openSession, t]);

  const rowsView = useMemo(() => rows.map(({ node, depth }) => {
    const isActive = node.session_id === activeId;
    const isSelected = node.session_id === selectedId;
    return (
      <button
        key={node.session_id}
        type="button"
        className={`flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm transition-colors ${isSelected ? 'bg-accent' : 'hover:bg-accent/50'}`}
        style={{ paddingLeft: `${depth * 1.25 + 0.5}rem` }}
        onClick={() => setSelectedId(node.session_id)}
        onDoubleClick={() => void openSession(node.session_id)}
      >
        {depth > 0 ? <GitFork className="h-3.5 w-3.5 shrink-0 text-muted-foreground" /> : null}
        <span className="min-w-0 flex-1 truncate">{node.title || node.session_id}</span>
        {isActive ? <Badge variant="secondary" className="shrink-0">{t('plugin.vivy/session-tree.current')}</Badge> : null}
      </button>
    );
  }), [rows, activeId, selectedId, openSession, t]);

  return (
    <div className="flex h-full flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <Button size="sm" variant="outline" disabled={busy} onClick={() => void load()}>
          <RefreshCw className="mr-1.5 h-3.5 w-3.5" />{t('plugin.vivy/session-tree.refresh')}
        </Button>
        <Button size="sm" variant="outline" disabled={busy || !selectedId} onClick={() => void cloneSelected()}>
          <GitFork className="mr-1.5 h-3.5 w-3.5" />{t('plugin.vivy/session-tree.clone')}
        </Button>
        <Button size="sm" variant="outline" disabled={busy || !selectedId} onClick={() => void exportSelected()}>
          <Download className="mr-1.5 h-3.5 w-3.5" />{t('plugin.vivy/session-tree.export')}
        </Button>
        <Button size="sm" variant="outline" disabled={busy} onClick={() => fileInput.current?.click()}>
          <Upload className="mr-1.5 h-3.5 w-3.5" />{t('plugin.vivy/session-tree.import')}
        </Button>
        <input
          ref={fileInput}
          type="file"
          accept=".jsonl,.json,.log"
          className="hidden"
          onChange={(event) => {
            const file = event.target.files?.[0];
            event.target.value = '';
            if (file) void importFile(file);
          }}
        />
      </div>
      {notice ? <p className="text-sm text-muted-foreground">{notice}</p> : null}
      {phase === 'loading' ? <p className="text-sm text-muted-foreground">{t('plugin.vivy/session-tree.loading')}</p> : null}
      {phase === 'error' ? <p className="text-sm text-destructive">{t('plugin.vivy/session-tree.loadFailed')}: {error}</p> : null}
      {phase === 'ready' && rows.length === 0 ? <p className="text-sm text-muted-foreground">{t('plugin.vivy/session-tree.empty')}</p> : null}
      {phase === 'ready' && rows.length > 0 ? (
        <div className="min-h-0 flex-1 overflow-auto rounded-md border p-2">
          <p className="px-2 pb-2 text-xs text-muted-foreground">{t('plugin.vivy/session-tree.selectHint')}</p>
          {rowsView}
        </div>
      ) : null}
    </div>
  );
}
