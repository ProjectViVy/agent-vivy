/**
 * Entry editor: plain-Markdown draft + rendered preview backed by the sealed
 * notebook actions. Editor state is (entryId, loadedVersion, baseRevisionId,
 * localDraft, pendingOperation): a failed save keeps both the key and the
 * request so a same-request retry reconciles the stored receipt instead of
 * minting a second revision; a changed request starts a new operation.
 */
import { useEffect, useRef, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { usePluginTranslation } from '@vivy/ui-sdk';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { cn } from '@/lib/utils';
import { NotebookError, newOperationKey, type NotebookClient } from './api';
import { NOTEBOOK_MAX_BODY_BYTES, type EntryView, type Revision } from './types';

export interface NotebookEditorProps {
  readonly client: NotebookClient;
  readonly entryId: string;
  /** When set, the panel shows this revision read-only instead of the head. */
  readonly viewRevisionId?: string | null;
  readonly onChanged?: () => void;
  readonly onRemoved?: () => void;
  readonly onDirty?: (dirty: boolean) => void;
}

type SaveState = 'clean' | 'dirty' | 'saving' | 'saved' | 'conflict' | 'unknown' | 'error';

interface PendingOperation {
  readonly key: string;
  readonly request: {
    readonly entry_id: string;
    readonly expected_version: number;
    readonly base_revision_id: string;
    readonly title: string;
    readonly markdown: string;
  };
}

const utf8Length = (value: string) => new TextEncoder().encode(value).byteLength;

export function NotebookEditor({ client, entryId, viewRevisionId, onChanged, onDirty }: NotebookEditorProps) {
  const { t } = usePluginTranslation();
  const [view, setView] = useState<EntryView | null>(null);
  const [viewed, setViewed] = useState<Revision | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [draftTitle, setDraftTitle] = useState('');
  const [draftMarkdown, setDraftMarkdown] = useState('');
  const [saveState, setSaveState] = useState<SaveState>('clean');
  const [saveError, setSaveError] = useState<{ readonly code: string; readonly message: string; readonly currentVersion?: number; readonly currentRevisionId?: string } | null>(null);
  const [pending, setPending] = useState<PendingOperation | null>(null);
  const [currentHead, setCurrentHead] = useState<Revision | null>(null);
  const [showCurrent, setShowCurrent] = useState(false);
  const [mode, setMode] = useState<'edit' | 'preview'>('edit');
  const epoch = useRef(0);

  const deleted = (view?.entry.deleted_at ?? 0) > 0;
  const readonly = deleted || viewRevisionId != null;
  const activeRevision = viewed ?? view?.revision ?? null;

  const load = async (revisionId?: string | null) => {
    const my = ++epoch.current;
    setLoading(true);
    setLoadError(null);
    try {
      const next = await client.getEntry({ id: entryId, ...(revisionId ? { revision_id: revisionId } : {}) });
      if (my !== epoch.current) return;
      setView(next);
      setViewed(revisionId ? next.revision : null);
      if (!revisionId) {
        setDraftTitle(next.entry.title);
        setDraftMarkdown(next.revision.markdown);
        setSaveState('clean');
        setSaveError(null);
        setPending(null);
        setCurrentHead(null);
        setShowCurrent(false);
      }
    } catch (cause) {
      if (my !== epoch.current) return;
      setLoadError(describe(cause));
    } finally {
      if (my === epoch.current) setLoading(false);
    }
  };

  useEffect(() => { void load(viewRevisionId ?? null); }, [client, entryId, viewRevisionId]);

  const dirty = !readonly && view !== null && (draftTitle !== view.entry.title || draftMarkdown !== view.revision.markdown);
  useEffect(() => {
    setSaveState((state) => (state === 'clean' || state === 'saved' || state === 'dirty') && dirty ? 'dirty' : !dirty && state === 'dirty' ? 'clean' : state);
    onDirty?.(dirty);
  }, [dirty, onDirty]);

  const markdownBytes = utf8Length(draftMarkdown);
  const overLimit = markdownBytes > NOTEBOOK_MAX_BODY_BYTES;

  const save = async () => {
    if (readonly || !view || !dirty || saveState === 'saving') return;
    const request = {
      entry_id: view.entry.id,
      expected_version: view.entry.version,
      base_revision_id: view.revision.id,
      title: draftTitle,
      markdown: draftMarkdown,
    };
    const key = pending && JSON.stringify(pending.request) === JSON.stringify(request) ? pending.key : newOperationKey();
    setPending({ key, request });
    setSaveState('saving');
    setSaveError(null);
    try {
      await client.saveEntry({ operationKey: key, ...request });
      setPending(null);
      await load(null);
      setSaveState('saved');
      onChanged?.();
    } catch (cause) {
      if (cause instanceof NotebookError && cause.code === 'revision_conflict') {
        setSaveState('conflict');
        setSaveError({ code: cause.code, message: cause.message, currentVersion: cause.currentVersion, currentRevisionId: cause.currentRevisionId });
        return;
      }
      setSaveState('unknown');
      setSaveError({ code: errorCode(cause), message: describe(cause) });
    }
  };

  const rebase = () => {
    if (!view || !saveError) return;
    setView({
      entry: { ...view.entry, version: saveError.currentVersion ?? view.entry.version },
      revision: { ...view.revision, id: saveError.currentRevisionId ?? view.revision.id },
    });
    setSaveState('dirty');
    setSaveError(null);
    setPending(null);
    setShowCurrent(false);
  };

  const showCurrentVersion = async () => {
    try {
      const head = await client.getEntry({ id: entryId });
      setCurrentHead(head.revision);
      setShowCurrent(true);
    } catch (cause) {
      setSaveError({ code: errorCode(cause), message: describe(cause) });
    }
  };

  const adoptViewed = async () => {
    if (!view || !viewed) return;
    try {
      await client.adoptRevision({
        operationKey: newOperationKey(), entry_id: view.entry.id,
        revision_id: viewed.id, expected_version: view.entry.version,
      });
      onChanged?.();
      void load(null);
    } catch (cause) {
      setSaveError({ code: errorCode(cause), message: describe(cause) });
    }
  };

  if (loading) {
    return (
      <div className="flex h-full min-h-0 flex-col gap-3 p-4" data-testid="notebook-editor-loading">
        <div className="h-8 w-2/3 animate-pulse rounded-lg bg-muted" />
        <div className="h-40 animate-pulse rounded-lg bg-muted" />
      </div>
    );
  }
  if (loadError || !view) {
    return (
      <div className="p-6 text-sm text-destructive" data-testid="notebook-editor-error">
        <p>{loadError ?? t('plugin.vivy/notebook.loadFailed')}</p>
        <Button className="mt-2" size="sm" variant="outline" onClick={() => void load(viewRevisionId ?? null)}>{t('common.retry')}</Button>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col" data-testid="notebook-editor">
      <div className="flex flex-wrap items-center gap-2 border-b px-4 py-2">
        <Badge variant="outline">{t(`plugin.vivy/notebook.kind.${view.entry.kind}`)}</Badge>
        {activeRevision ? <Badge variant="secondary">{t(`plugin.vivy/notebook.origin.${activeRevision.origin}`)}</Badge> : null}
        {deleted ? <Badge variant="destructive">{t('plugin.vivy/notebook.deleted')}</Badge> : null}
        {viewed ? <Badge variant="outline">{t('plugin.vivy/notebook.viewingRevision', { sequence: String(viewed.sequence) })}</Badge> : null}
        <span className="text-xs text-muted-foreground" data-testid="notebook-save-state">{t(`plugin.vivy/notebook.saveState.${saveState}`)}</span>
        <span className={cn('ml-auto text-xs', overLimit ? 'text-destructive' : 'text-muted-foreground')}>
          {markdownBytes}/{NOTEBOOK_MAX_BODY_BYTES}
        </span>
        {viewed ? (
          <>
            <Button size="sm" variant="outline" onClick={() => void load(null)}>{t('plugin.vivy/notebook.backToHead')}</Button>
            <Button size="sm" onClick={() => void adoptViewed()}>{t('plugin.vivy/notebook.adopt')}</Button>
          </>
        ) : null}
      </div>

      {saveState === 'conflict' ? (
        <div className="border-b bg-destructive/10 px-4 py-2 text-sm" data-testid="notebook-conflict">
          <p>{t('plugin.vivy/notebook.conflictBody')}</p>
          {showCurrent && currentHead ? (
            <article className="mt-2 rounded-lg border bg-card p-3" data-testid="notebook-current-preview">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{currentHead.markdown}</ReactMarkdown>
            </article>
          ) : null}
          <div className="mt-2 flex flex-wrap gap-2">
            <Button size="sm" variant="outline" onClick={rebase}>{t('plugin.vivy/notebook.keepEditing')}</Button>
            <Button size="sm" variant="outline" onClick={() => void showCurrentVersion()} data-testid="notebook-view-current">{t('plugin.vivy/notebook.viewCurrent')}</Button>
            <Button size="sm" variant="outline" onClick={() => void load(null)} data-testid="notebook-reload">{t('plugin.vivy/notebook.reload')}</Button>
          </div>
        </div>
      ) : null}
      {saveState === 'unknown' || saveState === 'error' ? (
        <div className="border-b bg-amber-500/10 px-4 py-2 text-sm" data-testid="notebook-unknown">
          <p>{t('plugin.vivy/notebook.unknownBody')}</p>
          <p className="mt-1 text-xs text-muted-foreground">{saveError?.message}</p>
          <div className="mt-2 flex gap-2">
            <Button size="sm" variant="outline" onClick={() => void save()}>{t('plugin.vivy/notebook.retrySave')}</Button>
            <Button size="sm" variant="outline" onClick={() => void load(null)}>{t('plugin.vivy/notebook.checkState')}</Button>
          </div>
        </div>
      ) : null}

      {readonly ? (
        <article className="min-h-0 flex-1 overflow-auto p-4 sm:p-6" data-testid="notebook-readonly">
          <h2 className="mb-3 text-lg font-semibold">{activeRevision?.title}</h2>
          <div className="prose prose-sm max-w-none dark:prose-invert">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{activeRevision?.markdown ?? ''}</ReactMarkdown>
          </div>
        </article>
      ) : (
        <Tabs value={mode} onValueChange={(value) => setMode(value as 'edit' | 'preview')} className="flex min-h-0 flex-1 flex-col">
          <div className="flex items-center gap-2 border-b px-4 py-2">
            <Input value={draftTitle} onChange={(event) => setDraftTitle(event.target.value)} className="max-w-sm" aria-label={t('plugin.vivy/notebook.titleLabel')} data-testid="notebook-title" />
            <TabsList className="ml-auto">
              <TabsTrigger value="edit">{t('plugin.vivy/notebook.edit')}</TabsTrigger>
              <TabsTrigger value="preview">{t('plugin.vivy/notebook.preview')}</TabsTrigger>
            </TabsList>
            <Button size="sm" disabled={!dirty || overLimit || saveState === 'saving'} onClick={() => void save()} data-testid="notebook-save">
              {t('plugin.vivy/notebook.save')}
            </Button>
          </div>
          <TabsContent value="edit" className="m-0 min-h-0 flex-1">
            <Textarea
              value={draftMarkdown}
              onChange={(event) => setDraftMarkdown(event.target.value)}
              className="h-full min-h-40 resize-none rounded-none border-0 font-mono text-sm focus-visible:ring-0"
              aria-label={t('plugin.vivy/notebook.markdownLabel')}
              data-testid="notebook-draft"
            />
          </TabsContent>
          <TabsContent value="preview" className="m-0 min-h-0 flex-1 overflow-auto p-4">
            <article className="prose prose-sm max-w-none dark:prose-invert">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{draftMarkdown}</ReactMarkdown>
            </article>
          </TabsContent>
        </Tabs>
      )}
    </div>
  );

}

function describe(cause: unknown): string {
  return cause instanceof Error ? cause.message : String(cause);
}

function errorCode(cause: unknown): string {
  return cause instanceof NotebookError ? cause.code : 'outcome_unknown';
}
