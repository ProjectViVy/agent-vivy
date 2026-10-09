/**
 * Notebook page: durable section/document editing over the sealed module
 * actions. Master = section picker + entry list; detail = editor plus
 * revisions/comments panels and entry lifecycle actions. Content authority is
 * the backend; an unsaved buffer lives only in page state and blocks
 * navigation until explicitly confirmed.
 */
import { useEffect, useMemo, useRef, useState } from 'react';
import { BookOpen, FilePlus2, FolderPlus, Pencil, RotateCcw, Trash2 } from 'lucide-react';
import { usePluginHost, usePluginTranslation } from '@vivy/ui-sdk';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { MasterDetail } from '@/components/layout/MasterDetail';
import { dateTimeLocale } from '@/i18n';
import { cn } from '@/lib/utils';
import { NotebookClient, NotebookError, newOperationKey } from './api';
import { NotebookEditor } from './editor';
import { NotebookRevisions } from './revisions';
import { NotebookComments } from './comments';
import {
  NOTEBOOK_ERROR_CODES, NOTEBOOK_MAX_PAGE_ROWS,
  type Entry, type Section,
} from './types';

type CollectionState = 'loading' | 'ready' | 'error';
type DetailTab = 'editor' | 'revisions' | 'comments';

export function NotebookView() {
  const host = usePluginHost();
  const { t } = usePluginTranslation();
  const client = useMemo(() => (host?.rpc ? NotebookClient.fromRPC(host.rpc) : null), [host?.rpc]);

  const [sections, setSections] = useState<readonly Section[]>([]);
  const [sectionsState, setSectionsState] = useState<CollectionState>('loading');
  const [unavailable, setUnavailable] = useState<string | null>(null);
  const [sectionId, setSectionId] = useState<string | null>(null);
  const [sectionForm, setSectionForm] = useState<'closed' | 'create' | 'rename'>('closed');
  const [sectionTitle, setSectionTitle] = useState('');
  const [sectionBusy, setSectionBusy] = useState(false);
  const [sectionError, setSectionError] = useState<string | null>(null);

  const [entries, setEntries] = useState<readonly Entry[]>([]);
  const [entriesState, setEntriesState] = useState<CollectionState>('loading');
  const [includeDeleted, setIncludeDeleted] = useState(false);
  const [entryId, setEntryId] = useState<string | null>(null);
  const [dirty, setDirty] = useState(false);
  const [pendingNav, setPendingNav] = useState<(() => void) | null>(null);
  const [viewRevisionId, setViewRevisionId] = useState<string | null>(null);
  const [detailTab, setDetailTab] = useState<DetailTab>('editor');
  const [moveTarget, setMoveTarget] = useState('');
  const [refreshKey, setRefreshKey] = useState(0);
  const [detailError, setDetailError] = useState<string | null>(null);
  const sectionsEpoch = useRef(0);
  const entriesEpoch = useRef(0);

  const selectedSection = sections.find((section) => section.id === sectionId) ?? null;
  const selectedEntry = entries.find((entry) => entry.id === entryId) ?? null;

  const loadSections = async () => {
    if (!client) {
      setUnavailable('no-rpc');
      return;
    }
    const my = ++sectionsEpoch.current;
    setSectionsState('loading');
    setUnavailable(null);
    try {
      const page = await client.listSections({ limit: NOTEBOOK_MAX_PAGE_ROWS });
      if (my !== sectionsEpoch.current) return;
      setSections(page.sections);
      setSectionsState('ready');
      setSectionId((current) => (current && page.sections.some((s) => s.id === current)
        ? current
        : page.sections[0]?.id ?? null));
    } catch (cause) {
      if (my !== sectionsEpoch.current) return;
      if (cause instanceof NotebookError && cause.code === NOTEBOOK_ERROR_CODES.capabilityUnavailable) {
        setUnavailable(cause.message);
        return;
      }
      setSectionsState('error');
    }
  };

  const loadEntries = async (section: string | null) => {
    if (!client || section == null) {
      setEntries([]);
      setEntriesState('ready');
      return;
    }
    const my = ++entriesEpoch.current;
    setEntriesState('loading');
    try {
      const page = await client.listEntries({ section_id: section, limit: NOTEBOOK_MAX_PAGE_ROWS, include_deleted: includeDeleted });
      if (my !== entriesEpoch.current) return;
      setEntries(page.entries);
      setEntriesState('ready');
      setEntryId((current) => (current && page.entries.some((e) => e.id === current) ? current : null));
    } catch {
      if (my === entriesEpoch.current) setEntriesState('error');
    }
  };

  useEffect(() => { void loadSections(); }, [client]);
  useEffect(() => { void loadEntries(sectionId); }, [client, sectionId, includeDeleted]);

  /** Navigation that would drop a dirty buffer goes through one confirmation. */
  const navigate = (next: () => void) => {
    if (dirty) {
      setPendingNav(() => next);
      return;
    }
    next();
  };

  const selectSection = (id: string) => navigate(() => { setSectionId(id); setEntryId(null); setViewRevisionId(null); });
  const selectEntry = (id: string) => navigate(() => { setEntryId(id); setViewRevisionId(null); setDetailTab('editor'); });

  const submitSection = async () => {
    if (!client) return;
    const title = sectionTitle.trim();
    if (!title) return;
    setSectionBusy(true);
    setSectionError(null);
    try {
      if (sectionForm === 'create') {
        await client.createSection({ operationKey: newOperationKey(), title });
      } else if (sectionForm === 'rename' && selectedSection) {
        await client.updateSection({ operationKey: newOperationKey(), id: selectedSection.id, title, expected_version: selectedSection.version });
      }
      setSectionForm('closed');
      setSectionTitle('');
      await loadSections();
    } catch (cause) {
      setSectionError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setSectionBusy(false);
    }
  };

  const createEntry = async () => {
    if (!client || !sectionId) return;
    setDetailError(null);
    try {
      const receipt = await client.createEntry({
        operationKey: newOperationKey(), section_id: sectionId,
        title: t('plugin.vivy/notebook.untitled'), markdown: '',
      });
      await loadEntries(sectionId);
      setEntryId(receipt.resource_id);
      setViewRevisionId(null);
      setDetailTab('editor');
    } catch (cause) {
      setDetailError(cause instanceof Error ? cause.message : String(cause));
    }
  };

  const mutateEntry = async (action: 'delete' | 'restore') => {
    if (!client || !selectedEntry) return;
    setDetailError(null);
    try {
      if (action === 'delete') {
        await client.deleteEntry({ operationKey: newOperationKey(), entry_id: selectedEntry.id, expected_version: selectedEntry.version });
      } else {
        await client.restoreEntry({ operationKey: newOperationKey(), entry_id: selectedEntry.id, expected_version: selectedEntry.version });
      }
      await loadEntries(sectionId);
      setRefreshKey((value) => value + 1);
    } catch (cause) {
      setDetailError(cause instanceof Error ? cause.message : String(cause));
    }
  };

  const moveEntry = async (targetSectionId: string) => {
    if (!client || !selectedEntry || !targetSectionId || targetSectionId === selectedEntry.section_id) return;
    setDetailError(null);
    try {
      await client.moveEntry({
        operationKey: newOperationKey(), entry_id: selectedEntry.id,
        section_id: targetSectionId, expected_version: selectedEntry.version,
      });
      await loadEntries(sectionId);
      setRefreshKey((value) => value + 1);
    } catch (cause) {
      setDetailError(cause instanceof Error ? cause.message : String(cause));
    }
  };

  const exportRevision = async () => {
    if (!client || !selectedEntry) return;
    setDetailError(null);
    try {
      const bundle = await client.exportEntry({ entry_id: selectedEntry.id, revision_id: viewRevisionId ?? selectedEntry.head_revision_id });
      const slug = `${bundle.entry.title.replace(/[^\p{L}\p{N}_-]+/gu, '_') || 'entry'}-r${bundle.revision.sequence}`;
      download(`${slug}.md`, bundle.revision.markdown, 'text/markdown');
      download(`${slug}.sidecar.json`, JSON.stringify({
        entry: bundle.entry,
        revision: { ...bundle.revision, markdown: undefined },
        comments: bundle.comments,
      }, null, 2), 'application/json');
    } catch (cause) {
      setDetailError(cause instanceof Error ? cause.message : String(cause));
    }
  };

  if (unavailable) {
    return (
      <div className="flex h-full items-center justify-center p-6" data-testid="notebook-unavailable">
        <div className="text-center">
          <BookOpen className="mx-auto mb-2 h-12 w-12 text-muted-foreground opacity-40" />
          <p className="text-sm text-muted-foreground">{t('plugin.vivy/notebook.unavailable')}</p>
        </div>
      </div>
    );
  }

  const master = (
    <aside className="flex h-full min-h-0 flex-col border-b bg-sidebar md:border-b-0 md:border-r">
      <div className="space-y-1 border-b p-3">
        <div className="flex items-center gap-2">
          <select
            className="h-9 min-w-0 flex-1 rounded-md border border-input bg-transparent px-2 text-sm"
            value={sectionId ?? ''}
            onChange={(event) => selectSection(event.target.value)}
            aria-label={t('plugin.vivy/notebook.sectionsLabel')}
            data-testid="notebook-section-select"
          >
            {sections.map((section) => (
              <option key={section.id} value={section.id}>
                {section.system_role ? t(`plugin.vivy/notebook.roles.${section.system_role}`) : section.title}
              </option>
            ))}
          </select>
          <Button size="icon" variant="outline" title={t('plugin.vivy/notebook.newSection')} onClick={() => { setSectionForm('create'); setSectionTitle(''); setSectionError(null); }} data-testid="notebook-section-create">
            <FolderPlus className="h-4 w-4" />
          </Button>
          {selectedSection && !selectedSection.system_role ? (
            <Button size="icon" variant="outline" title={t('plugin.vivy/notebook.renameSection')} onClick={() => { setSectionForm('rename'); setSectionTitle(selectedSection.title); setSectionError(null); }} data-testid="notebook-section-rename">
              <Pencil className="h-4 w-4" />
            </Button>
          ) : null}
        </div>
        {sectionForm !== 'closed' ? (
          <div className="flex gap-2">
            <Input value={sectionTitle} onChange={(event) => setSectionTitle(event.target.value)} placeholder={t('plugin.vivy/notebook.sectionTitlePlaceholder')} data-testid="notebook-section-title" onKeyDown={(event) => { if (event.key === 'Enter') void submitSection(); }} />
            <Button size="sm" disabled={sectionBusy || !sectionTitle.trim()} onClick={() => void submitSection()} data-testid="notebook-section-create-submit">{t('common.save')}</Button>
            <Button size="sm" variant="ghost" onClick={() => setSectionForm('closed')}>{t('common.cancel')}</Button>
          </div>
        ) : null}
        {sectionError ? <p className="text-xs text-destructive">{sectionError}</p> : null}
      </div>

      <div className="flex items-center justify-between border-b px-3 py-2">
        <span className="text-xs font-medium uppercase text-muted-foreground">{t('plugin.vivy/notebook.documents')}</span>
        <div className="flex items-center gap-2">
          <label className="flex items-center gap-1 text-xs text-muted-foreground">
            <input type="checkbox" checked={includeDeleted} onChange={(event) => setIncludeDeleted(event.target.checked)} />
            {t('plugin.vivy/notebook.showDeleted')}
          </label>
          <Button size="icon" variant="ghost" title={t('plugin.vivy/notebook.newNote')} disabled={!sectionId} onClick={() => void createEntry()} data-testid="notebook-entry-create">
            <FilePlus2 className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {pendingNav ? (
        <div className="border-b bg-amber-500/10 px-3 py-2 text-xs" data-testid="notebook-discard-confirm">
          <p>{t('plugin.vivy/notebook.discardPrompt')}</p>
          <div className="mt-1 flex gap-2">
            <Button size="sm" variant="outline" data-testid="notebook-discard-confirm-leave" onClick={() => { const next = pendingNav; setPendingNav(null); next(); }}>{t('plugin.vivy/notebook.discardLeave')}</Button>
            <Button size="sm" variant="ghost" data-testid="notebook-discard-confirm-stay" onClick={() => setPendingNav(null)}>{t('plugin.vivy/notebook.discardStay')}</Button>
          </div>
        </div>
      ) : null}

      <ScrollArea className="min-h-0 flex-1 p-2">
        {sectionsState === 'error' ? (
          <div className="p-3 text-sm text-destructive">
            <p>{t('plugin.vivy/notebook.sectionsFailed')}</p>
            <Button className="mt-2" size="sm" variant="outline" onClick={() => void loadSections()}>{t('common.retry')}</Button>
          </div>
        ) : entriesState === 'error' ? (
          <div className="p-3 text-sm text-destructive">
            <p>{t('plugin.vivy/notebook.entriesFailed')}</p>
            <Button className="mt-2" size="sm" variant="outline" onClick={() => void loadEntries(sectionId)}>{t('common.retry')}</Button>
          </div>
        ) : sectionsState === 'loading' || entriesState === 'loading' ? (
          <div className="space-y-2 p-1">
            <div className="h-14 animate-pulse rounded-lg bg-muted" />
            <div className="h-14 animate-pulse rounded-lg bg-muted" />
          </div>
        ) : entries.length === 0 ? (
          <p className="py-10 text-center text-sm text-muted-foreground" data-testid="notebook-entries-empty">{t('plugin.vivy/notebook.emptySection')}</p>
        ) : entries.map((entry) => (
          <button
            key={entry.id}
            type="button"
            onClick={() => selectEntry(entry.id)}
            data-testid="notebook-entry"
            className={cn('mb-1 w-full rounded-lg p-3 text-left', entryId === entry.id ? 'bg-sidebar-accent' : 'hover:bg-sidebar-accent/50')}
          >
            <div className="flex items-center gap-2">
              <span className="min-w-0 flex-1 truncate text-sm font-medium">{entry.title}</span>
              {entry.kind === 'report' ? <Badge variant="secondary">{t('plugin.vivy/notebook.kind.report')}</Badge> : null}
              {(entry.deleted_at ?? 0) > 0 ? <Badge variant="destructive">{t('plugin.vivy/notebook.deleted')}</Badge> : null}
            </div>
            <p className="mt-1 text-xs text-muted-foreground">{new Date(entry.updated_at * 1000).toLocaleString(dateTimeLocale())}</p>
          </button>
        ))}
      </ScrollArea>
    </aside>
  );

  const detail = selectedEntry && client ? (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex flex-wrap items-center gap-2 border-b px-4 py-2">
        <span className="min-w-0 flex-1 truncate text-sm font-medium">{selectedEntry.title}</span>
        {(selectedEntry.deleted_at ?? 0) > 0 ? (
          <Button size="sm" variant="outline" onClick={() => void mutateEntry('restore')} data-testid="notebook-entry-restore">
            <RotateCcw className="mr-1 h-3.5 w-3.5" />{t('plugin.vivy/notebook.restore')}
          </Button>
        ) : (
          <>
            <select
              className="h-8 rounded-md border border-input bg-transparent px-2 text-xs"
              value={moveTarget || selectedEntry.section_id}
              onChange={(event) => { setMoveTarget(event.target.value); void moveEntry(event.target.value); }}
              aria-label={t('plugin.vivy/notebook.moveTo')}
              data-testid="notebook-entry-move"
            >
              {sections.map((section) => (
                <option key={section.id} value={section.id}>
                  {section.system_role ? t(`plugin.vivy/notebook.roles.${section.system_role}`) : section.title}
                </option>
              ))}
            </select>
            <Button size="sm" variant="outline" onClick={() => void exportRevision()} data-testid="notebook-export">{t('plugin.vivy/notebook.export')}</Button>
            <Button size="sm" variant="outline" onClick={() => void mutateEntry('delete')} data-testid="notebook-entry-delete">
              <Trash2 className="mr-1 h-3.5 w-3.5" />{t('common.delete')}
            </Button>
          </>
        )}
      </div>
      {detailError ? <p className="border-b bg-destructive/10 px-4 py-1.5 text-xs text-destructive" data-testid="notebook-detail-error">{detailError}</p> : null}
      <Tabs value={detailTab} onValueChange={(value) => setDetailTab(value as DetailTab)} className="flex min-h-0 flex-1 flex-col">
        <div className="border-b px-4 py-1.5">
          <TabsList>
            <TabsTrigger value="editor">{t('plugin.vivy/notebook.tabs.editor')}</TabsTrigger>
            <TabsTrigger value="revisions">{t('plugin.vivy/notebook.tabs.revisions')}</TabsTrigger>
            <TabsTrigger value="comments">{t('plugin.vivy/notebook.tabs.comments')}</TabsTrigger>
          </TabsList>
        </div>
        <TabsContent value="editor" className="m-0 flex min-h-0 flex-1 flex-col">
          <NotebookEditor
            key={`${selectedEntry.id}:${selectedEntry.version}`}
            client={client}
            entryId={selectedEntry.id}
            viewRevisionId={viewRevisionId}
            onChanged={() => { void loadEntries(sectionId); setRefreshKey((value) => value + 1); }}
            onDirty={setDirty}
          />
        </TabsContent>
        <TabsContent value="revisions" className="m-0 flex min-h-0 flex-1 flex-col">
          <NotebookRevisions
            client={client}
            entryId={selectedEntry.id}
            headRevisionId={selectedEntry.head_revision_id}
            entryVersion={selectedEntry.version}
            refreshKey={refreshKey}
            viewingRevisionId={viewRevisionId}
            onView={(id) => { setViewRevisionId(id); if (id) setDetailTab('editor'); }}
            onAdopted={() => { void loadEntries(sectionId); setRefreshKey((value) => value + 1); }}
          />
        </TabsContent>
        <TabsContent value="comments" className="m-0 flex min-h-0 flex-1 flex-col">
          <NotebookComments
            client={client}
            entryId={selectedEntry.id}
            anchorRevisionId={viewRevisionId ?? selectedEntry.head_revision_id}
            refreshKey={refreshKey}
          />
        </TabsContent>
      </Tabs>
    </div>
  ) : (
    <div className="flex h-full items-center justify-center p-6 text-muted-foreground">
      <div className="text-center">
        <BookOpen className="mx-auto mb-2 h-12 w-12 opacity-40" />
        <p>{t('plugin.vivy/notebook.selectEntry')}</p>
      </div>
    </div>
  );

  return (
    <div className="h-full" data-testid="notebook-page">
      <MasterDetail selected={selectedEntry !== null} onBack={() => setEntryId(null)} master={master} detail={detail} />
    </div>
  );
}

function download(name: string, content: string, type: string) {
  const url = URL.createObjectURL(new Blob([content], { type }));
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = name;
  anchor.click();
  URL.revokeObjectURL(url);
}
