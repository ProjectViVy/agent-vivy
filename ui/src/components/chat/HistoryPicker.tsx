import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { History, Loader2, Search, X } from 'lucide-react';
import * as api from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Skeleton } from '@/components/ui/skeleton';
import { useIsMobile } from '@/hooks/use-mobile';
import { dateTimeLocale, useTranslation } from '@/i18n';

export interface HistoryPickerProps {
  destinationSessionId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onAttach: (preview: api.ReferencePreview, selection: api.ReferenceSelection, allowFurtherReading: boolean) => void;
}

type KindFilter = 'message' | 'tool' | 'file';
const KIND_MAP: Record<KindFilter, string[]> = {
  message: ['message'],
  tool: ['tool_call', 'tool_result'],
  file: ['file', 'diff'],
};

const refKey = (ref: api.SourceRef) => [ref.session_id, ref.run_id ?? '', ref.message_id ?? '', ref.event_seq ?? 0, ref.kind].join('/');
const itemKey = (item: api.HistoryItem) => refKey(item.ref);
const itemBytes = (item: api.HistoryItem) => new TextEncoder().encode(item.text).length;

/**
 * Explicit history picker (SC-D4 §13.1–13.2): session list → bounded source
 * records → reference/preview → composer chip. Selections are handles, not
 * authorization; search only runs on submit and responses from a stale
 * session/query generation are discarded.
 */
export function HistoryPicker({ destinationSessionId, open, onOpenChange, onAttach }: HistoryPickerProps) {
  const { t } = useTranslation();
  const mobile = useIsMobile();
  const [sessionQuery, setSessionQuery] = useState('');
  const [sessions, setSessions] = useState<api.HistorySession[]>([]);
  const [sessionsCursor, setSessionsCursor] = useState('');
  const [sessionsPhase, setSessionsPhase] = useState<'idle' | 'loading' | 'ready' | 'error'>('idle');
  const [sourceId, setSourceId] = useState<string | null>(null);
  const [recordQuery, setRecordQuery] = useState('');
  const [kindFilter, setKindFilter] = useState<KindFilter>('message');
  const [items, setItems] = useState<api.HistoryItem[]>([]);
  const [itemsCursor, setItemsCursor] = useState('');
  const [itemsPhase, setItemsPhase] = useState<'idle' | 'loading' | 'ready' | 'error'>('idle');
  const [itemsStatus, setItemsStatus] = useState('');
  const [selected, setSelected] = useState<Map<string, api.HistoryItem>>(new Map());
  const [allowFurther, setAllowFurther] = useState(false);
  const [preview, setPreview] = useState<api.ReferencePreview | null>(null);
  const [previewPhase, setPreviewPhase] = useState<'idle' | 'loading' | 'ready' | 'error'>('idle');
  // Stale-response guard: every new search/preview bumps the generation.
  const generation = useRef(0);

  const loadSessions = useCallback(async (query: string, cursor: string) => {
    const gen = ++generation.current;
    setSessionsPhase('loading');
    try {
      const page = await api.historySessions({ query, cursor, limit: 20 });
      if (gen !== generation.current) return;
      setSessions((current) => cursor ? [...current, ...page.sessions] : page.sessions);
      setSessionsCursor(page.next_cursor ?? '');
      setSessionsPhase('ready');
    } catch {
      if (gen !== generation.current) return;
      setSessionsPhase('error');
    }
  }, []);

  const loadItems = useCallback(async (source: string, query: string, kind: KindFilter, cursor: string) => {
    const gen = ++generation.current;
    setItemsPhase('loading');
    try {
      const page = await api.historySearch(destinationSessionId, {
        session_ids: [source], query, kinds: KIND_MAP[kind], cursor, limit: 50,
      });
      if (gen !== generation.current) return;
      setItems((current) => cursor ? [...current, ...page.items] : page.items);
      setItemsCursor(page.next_cursor ?? '');
      setItemsStatus(page.status);
      setItemsPhase('ready');
    } catch {
      if (gen !== generation.current) return;
      setItemsPhase('error');
    }
  }, [destinationSessionId]);

  // Opening shows recent sessions only — metadata, never transcripts.
  useEffect(() => {
    if (!open) return;
    setSessionQuery(''); setSourceId(null); setRecordQuery(''); setItems([]);
    setItemsCursor(''); setItemsPhase('idle'); setSelected(new Map());
    setAllowFurther(false); setPreview(null); setPreviewPhase('idle');
    void loadSessions('', '');
  }, [open, loadSessions]);

  useEffect(() => {
    if (!sourceId) return;
    setItems([]); setItemsCursor(''); setPreview(null); setPreviewPhase('idle');
    void loadItems(sourceId, recordQuery, kindFilter, '');
    // recordQuery is applied via explicit search submit, not on each keystroke.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sourceId, kindFilter, loadItems]);

  const toggleItem = (item: api.HistoryItem, checked: boolean) => {
    setPreview(null); setPreviewPhase('idle');
    setSelected((current) => {
      const next = new Map(current);
      if (checked) next.set(itemKey(item), item); else next.delete(itemKey(item));
      return next;
    });
  };

  const selectedBytes = useMemo(
    () => [...selected.values()].reduce((sum, item) => sum + itemBytes(item), 0),
    [selected],
  );

  const selection = useMemo<api.HistorySelection | null>(() => {
    if (!sourceId || selected.size === 0) return null;
    return { source_session_id: sourceId, refs: [...selected.values()].map((item) => item.ref) };
  }, [sourceId, selected]);

  const runPreview = async () => {
    if (!selection) return;
    const gen = ++generation.current;
    setPreviewPhase('loading');
    try {
      const result = await api.previewReference(destinationSessionId, selection);
      if (gen !== generation.current) return;
      setPreview(result);
      setPreviewPhase('ready');
    } catch {
      if (gen !== generation.current) return;
      setPreviewPhase('error');
    }
  };

  const attach = () => {
    if (!preview || preview.source_status !== 'ok') return;
    onAttach(preview, { selection: preview.selection, expected_digest: preview.digest }, allowFurther);
    onOpenChange(false);
  };

  const attachable = preview !== null && previewPhase === 'ready' && preview.source_status === 'ok';

  const sessionList = (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex gap-2 p-2">
        <Input
          value={sessionQuery}
          onChange={(event) => setSessionQuery(event.target.value)}
          onKeyDown={(event) => { if (event.key === 'Enter') void loadSessions(sessionQuery, ''); }}
          placeholder={t('historyPicker.searchSessions')}
          aria-label={t('historyPicker.searchSessions')}
          className="h-8"
        />
        <Button type="button" variant="outline" size="icon" className="h-8 w-8 shrink-0" aria-label={t('historyPicker.search')} disabled={sessionsPhase === 'loading'} onClick={() => void loadSessions(sessionQuery, '')}>
          <Search className="h-4 w-4" />
        </Button>
      </div>
      <ScrollArea className="min-h-0 flex-1">
        {sessionsPhase === 'loading' && sessions.length === 0 ? <div className="space-y-2 p-2"><Skeleton className="h-10" /><Skeleton className="h-10" /></div> : null}
        {sessionsPhase === 'ready' && sessions.length === 0 ? <p className="p-3 text-sm text-muted-foreground">{t('historyPicker.noSessions')}</p> : null}
        {sessions.map((session) => (
          <button
            key={session.id}
            type="button"
            onClick={() => setSourceId(session.id)}
            className={`flex w-full flex-col gap-0.5 border-b px-3 py-2 text-left hover:bg-accent ${sourceId === session.id ? 'bg-accent' : ''}`}
          >
            <span className="truncate text-sm" title={session.title}>{session.title || session.id}</span>
            <span className="truncate text-xs text-muted-foreground">{session.workspace_label || new Date(session.updated_at).toLocaleString(dateTimeLocale())}</span>
          </button>
        ))}
        {sessionsCursor ? (
          <Button type="button" variant="ghost" size="sm" className="m-2" disabled={sessionsPhase === 'loading'} onClick={() => void loadSessions(sessionQuery, sessionsCursor)}>
            {sessionsPhase === 'loading' ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}{t('historyPicker.loadMore')}
          </Button>
        ) : null}
      </ScrollArea>
    </div>
  );

  const recordList = (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex flex-wrap items-center gap-2 border-b p-2">
        <Button type="button" variant="ghost" size="icon" className="h-8 w-8 shrink-0 md:hidden" aria-label={t('historyPicker.back')} onClick={() => { setSourceId(null); setItems([]); setItemsPhase('idle'); setItemsCursor(''); }}>
          <X className="h-4 w-4" />
        </Button>
        <Input
          value={recordQuery}
          onChange={(event) => setRecordQuery(event.target.value)}
          onKeyDown={(event) => { if (event.key === 'Enter' && sourceId) void loadItems(sourceId, recordQuery, kindFilter, ''); }}
          placeholder={t('historyPicker.searchRecords')}
          aria-label={t('historyPicker.searchRecords')}
          className="h-8 min-w-0 flex-1"
        />
        <Button type="button" variant="outline" size="icon" className="h-8 w-8 shrink-0" aria-label={t('historyPicker.search')} disabled={!sourceId || itemsPhase === 'loading'} onClick={() => sourceId && void loadItems(sourceId, recordQuery, kindFilter, '')}>
          <Search className="h-4 w-4" />
        </Button>
        <div className="flex items-center gap-1 text-xs" role="group" aria-label={t('historyPicker.filters')}>
          {(['message', 'tool', 'file'] as const).map((kind) => (
            <Button key={kind} type="button" variant={kindFilter === kind ? 'secondary' : 'ghost'} size="sm" className="h-7 px-2" onClick={() => setKindFilter(kind)}>
              {t(`historyPicker.kind.${kind}`)}
            </Button>
          ))}
        </div>
      </div>
      <ScrollArea className="min-h-0 flex-1">
        {!sourceId ? <p className="p-4 text-sm text-muted-foreground">{t('historyPicker.chooseSession')}</p> : null}
        {itemsPhase === 'loading' && items.length === 0 ? <div className="space-y-2 p-3"><Skeleton className="h-8" /><Skeleton className="h-8" /></div> : null}
        {itemsPhase === 'error' ? <p className="p-4 text-sm text-destructive">{t('historyPicker.loadFailed')}</p> : null}
        {itemsPhase === 'ready' && items.length === 0 ? <p className="p-4 text-sm text-muted-foreground">{t('historyPicker.noRecords')}</p> : null}
        {items.map((item) => (
          <label key={itemKey(item)} className="flex items-start gap-2 border-b px-3 py-2 hover:bg-accent">
            <Checkbox
              checked={selected.has(itemKey(item))}
              onCheckedChange={(checked) => toggleItem(item, checked === true)}
              aria-label={t('historyPicker.selectRecord', { author: item.author, kind: item.ref.kind })}
              className="mt-0.5"
            />
            <span className="min-w-0 flex-1">
              <span className="text-xs text-muted-foreground">{item.author} · {item.ref.kind}{item.truncated ? ` · ${t('historyPicker.truncated')}` : ''}</span>
              <span className="block break-all text-sm">{item.text}</span>
            </span>
          </label>
        ))}
        {itemsCursor ? (
          <Button type="button" variant="ghost" size="sm" className="m-2" disabled={itemsPhase === 'loading'} onClick={() => sourceId && void loadItems(sourceId, recordQuery, kindFilter, itemsCursor)}>
            {itemsPhase === 'loading' ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}{t('historyPicker.loadMore')}
          </Button>
        ) : null}
        {itemsStatus === 'partial' ? <p className="p-2 text-xs text-muted-foreground">{t('historyPicker.moreRemain')}</p> : null}
      </ScrollArea>
    </div>
  );

  const footer = (
    <div className="flex flex-wrap items-center gap-3 border-t p-3">
      <span className="text-xs text-muted-foreground" aria-live="polite">
        {t('historyPicker.selectedCount', { count: selected.size, bytes: selectedBytes })}
      </span>
      <label className="flex items-center gap-2 text-xs">
        <Checkbox checked={allowFurther} onCheckedChange={(checked) => setAllowFurther(checked === true)} aria-label={t('historyPicker.allowFurther')} />
        {t('historyPicker.allowFurther')}
      </label>
      <div className="ml-auto flex items-center gap-2">
        <Button type="button" variant="outline" size="sm" disabled={!selection || previewPhase === 'loading'} onClick={() => void runPreview()}>
          {previewPhase === 'loading' ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}{t('historyPicker.preview')}
        </Button>
        <Button type="button" size="sm" disabled={!attachable} onClick={attach}>{t('historyPicker.attach')}</Button>
        <Button type="button" variant="ghost" size="sm" onClick={() => onOpenChange(false)}>{t('historyPicker.cancel')}</Button>
      </div>
      {previewPhase === 'ready' && preview ? (
        <div className="w-full space-y-1" role="status">
          <p className="text-xs text-muted-foreground">
            {t('historyPicker.previewMeta', { count: preview.items.length, bytes: preview.byte_count, status: preview.source_status })}
          </p>
          {preview.source_status !== 'ok' ? <p className="text-xs text-destructive">{t('historyPicker.narrowing')}</p> : null}
          <ScrollArea className="max-h-32 rounded border p-2">
            {preview.items.map((item) => (
              <p key={`preview-${itemKey(item)}`} className="break-all text-xs"><span className="text-muted-foreground">{item.author}:</span> {item.text}</p>
            ))}
          </ScrollArea>
        </div>
      ) : null}
      {previewPhase === 'error' ? <p className="w-full text-xs text-destructive">{t('historyPicker.previewFailed')}</p> : null}
    </div>
  );

  if (mobile) {
    return (
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent side="bottom" className="flex h-[100dvh] w-full flex-col p-0">
          <SheetHeader className="border-b px-4 py-3 text-left">
            <SheetTitle>{t('historyPicker.title')}</SheetTitle>
            <SheetDescription className="sr-only">{t('historyPicker.description')}</SheetDescription>
          </SheetHeader>
          <div className="min-h-0 flex-1">{sourceId ? recordList : sessionList}</div>
          <SheetFooter className="mt-0 p-0">{footer}</SheetFooter>
        </SheetContent>
      </Sheet>
    );
  }
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex h-[80dvh] w-[min(960px,calc(100vw-2rem))] max-w-none flex-col p-0" aria-describedby={undefined}>
        <DialogHeader className="border-b px-4 py-3 text-left">
          <DialogTitle className="flex items-center gap-2"><History className="h-4 w-4" />{t('historyPicker.title')}</DialogTitle>
          <DialogDescription className="sr-only">{t('historyPicker.description')}</DialogDescription>
        </DialogHeader>
        <div className="flex min-h-0 flex-1">
          <div className="w-[280px] shrink-0 border-r">{sessionList}</div>
          <div className="min-w-0 flex-1">{recordList}</div>
        </div>
        <DialogFooter className="mt-0 p-0">{footer}</DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
