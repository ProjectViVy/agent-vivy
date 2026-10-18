import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react';
import { usePluginHost, usePluginTranslation, type FullUIHost, type UITranslator } from '@vivy/ui-sdk';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent } from '@/components/ui/card';
import { Textarea } from '@/components/ui/textarea';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Dialog, DialogBody, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { MemoryView } from './view';
import { RecapClient, type RecapHead, type RecapOutcome, type RecapSource } from './recap-client';

type Draft = { readonly text: string; readonly base: number };
type Tab = 'active' | 'history' | 'long-term';
type WriteIntent = { readonly kind: 'clear' | 'restore'; readonly base: number; readonly revision?: number };
type DialogIntent = WriteIntent | { readonly kind: 'reload' };
const prefix = 'plugin.vivy/memory.recap.';
const message = (cause: unknown) => cause instanceof Error ? cause.message : String(cause);
const noSubscription = () => () => undefined;
// Backend reason codes never reach the user raw; unknown codes read as generic.
const reasonKey = (reason?: string) => reason === 'recap_scope_denied' || reason === 'recap_commit_unknown' ? `reason.${reason}` : 'reason.generic';

export function RecapWorkspace() {
 const host = usePluginHost();
 const { t } = usePluginTranslation();
 const client = useMemo(() => host ? RecapClient.fromRPC(host.rpc) : null, [host]);
 const subscribe = useCallback((listener: () => void) => host?.store.subscribe(listener) ?? noSubscription(), [host]);
 const session = useSyncExternalStore(subscribe, () => host?.store.getState().activeSessionId ?? null, () => null);
 // Drafts are ephemeral editing state only. A session switch cannot move an
 // unsaved draft into another authorization context or lose it on return.
 const drafts = useRef(new Map<string, Draft>());
 return <RecapEditor key={session ?? 'unbound'} host={host ?? null} client={client} session={session} t={t} drafts={drafts.current} />;
}

function RecapEditor({ host, client, session, t, drafts }: {
 readonly host: FullUIHost | null; readonly client: RecapClient | null;
 readonly session: string | null; readonly t: UITranslator; readonly drafts: Map<string, Draft>;
}) {
 const copy = (key: string) => t(prefix + key);
 const [tab, setTab] = useState<Tab>('active');
 const [head, setHead] = useState<RecapHead | null>(null);
 const [loaded, setLoaded] = useState<RecapOutcome['loaded']>();
 const [draft, setDraft] = useState<Draft | null>(() => session ? drafts.get(session) ?? null : null);
 const [editing, setEditing] = useState(false);
 const [conflict, setConflict] = useState<RecapHead | null>(null);
 const [error, setError] = useState<string | null>(null);
 const [notice, setNotice] = useState<string | null>(null);
 const [loading, setLoading] = useState(false);
 const [busy, setBusy] = useState(false);
 const [history, setHistory] = useState<readonly RecapHead[]>([]);
 const [nextBefore, setNextBefore] = useState<number | undefined>();
 const [preview, setPreview] = useState<RecapHead | null>(null);
 const [sources, setSources] = useState<readonly RecapSource[] | null>(null);
 const [sourceReason, setSourceReason] = useState<string | null>(null);
 const [sourceRevision,setSourceRevision]=useState<number|null>(null);
 const sourceRequest=useRef(0);
 const revisionRequest=useRef(0);
 const [dialog, setDialog] = useState<DialogIntent | null>(null);
 const [conflictIntent,setConflictIntent]=useState<WriteIntent|null>(null);
 const request = useRef(0);
 const draftRef = useRef(draft); draftRef.current = draft;
 const intent = useRef<{ fingerprint: string; operationId: string } | null>(null);
 const mounted = useRef(true);
 useEffect(() => { mounted.current = true; return () => { mounted.current = false; request.current++; }; }, []);
 const changeDraft = (value: Draft | null) => {
  draftRef.current = value; setDraft(value);
  if (session) { if (value) drafts.set(session, value); else drafts.delete(session); }
 };
 const acceptHead = useCallback((value: RecapHead) => {
  setHead(current => !current || value.revision >= current.revision ? value : current);
 }, []);
 const refresh = useCallback(async () => {
  if (!client || !session) return;
  const seq = ++request.current; setLoading(true);
  try {
   const outcome = await client.invoke('read', session);
   if (!mounted.current || seq !== request.current) return;
   setLoaded(outcome.loaded_status === 'loaded' ? outcome.loaded : undefined);
   if (outcome.status !== 'listed' || !outcome.head) { setError(copy(reasonKey(outcome.reason))); setHead(null); return; }
   acceptHead(outcome.head); setError(null);
  } catch (cause) { if (mounted.current && seq === request.current) { setError(message(cause)); setLoaded(undefined); } }
  finally { if (mounted.current && seq === request.current) setLoading(false); }
 }, [client, session, acceptHead]);
 useEffect(() => {
  void refresh();
  const focus = () => { void refresh(); };
  window.addEventListener('focus', focus);
  return () => window.removeEventListener('focus', focus);
 }, [refresh]);
 const loadHistory = async (before?: number) => {
  if (!client || !session) return;
  try {
   const outcome = await client.invoke('history', session, { limit: 20, ...(before ? { before } : {}) });
   if (!mounted.current) return;
   if (outcome.status !== 'listed') { setError(copy(reasonKey(outcome.reason))); return; }
   setHistory(current => before ? [...current, ...(outcome.revisions ?? [])] : outcome.revisions ?? []);
   setNextBefore(outcome.next_before);
  } catch (cause) { if (mounted.current) setError(message(cause)); }
 };
 const selectRevision = async (revision: number) => {
  if (!client || !session) return;
  const seq=++revisionRequest.current;
  try {
   const outcome = await client.invoke('revision', session, { revision });
   if (!mounted.current) return;
   if (seq!==revisionRequest.current) return;
   if (outcome.status === 'listed' && outcome.head) {setPreview(outcome.head);setSources(null);setSourceReason(null);sourceRequest.current++;}
   else setError(copy(reasonKey(outcome.reason)));
  } catch (cause) { if (mounted.current) setError(message(cause)); }
 };
 const selectTab = (value: Tab) => { if (value === tab) return; setTab(value); if (value === 'history') void loadHistory(); };
 const write = async (action: 'edit' | 'clear' | 'restore', expected?: WriteIntent) => {
  if (!client || !session || !head || busy) return;
  const savedDraft = draftRef.current;
  if (action === 'edit' && !savedDraft) return;
  if (action !== 'edit' && !expected) return;
  const input: Record<string, unknown> = { base_revision: action === 'edit' ? savedDraft!.base : expected!.base };
  if (action === 'edit') input.text = savedDraft!.text;
  if (action === 'restore') { if (!expected?.revision) return; input.revision = expected.revision; }
  const fingerprint = JSON.stringify({ action, ...input });
  if (intent.current?.fingerprint !== fingerprint) intent.current = { fingerprint, operationId: crypto.randomUUID() };
  input.operation_id = intent.current.operationId;
  setBusy(true); setError(null); setNotice(null);
  try {
   const outcome: RecapOutcome = await client.invoke(action, session, input);
   if (!mounted.current) return;
   request.current++; setLoading(false);
   if (outcome.status === 'conflict' && outcome.head) {
    acceptHead(outcome.head); setConflict(outcome.head); setConflictIntent(expected ?? null); setDialog(null); return;
   }
   if (outcome.status !== 'applied' || !outcome.head) { setError(copy(reasonKey(outcome.reason))); return; }
   acceptHead(outcome.head); setDialog(null); setConflict(null); setConflictIntent(null); intent.current = null;
   if (action === 'edit' && draftRef.current === savedDraft) { changeDraft(null); setEditing(false); }
   setNotice(copy(outcome.activation === 'next_safe_boundary' ? 'nextSafeBoundary' : 'nextRun'));
   if (tab === 'history') await loadHistory();
  } catch (cause) { if (mounted.current) setError(message(cause)); }
  finally { if (mounted.current) setBusy(false); }
 };
 const showSources = async () => {
  const selected = tab === 'history' ? preview : head;
  if (!client || !session || !selected || selected.revision === 0) return;
  const seq=++sourceRequest.current;setSourceRevision(selected.revision);
  setSources(null); setSourceReason('loading');
  try {
   const outcome = await client.invoke('sources', session, { revision: selected.revision });
   if (!mounted.current) return;
   if (seq!==sourceRequest.current) return;
   if (outcome.status === 'listed' && outcome.sources) { setSources(outcome.sources); setSourceReason(null); }
   else setSourceReason(outcome.reason ?? outcome.status);
  } catch (cause) { if (mounted.current && seq === sourceRequest.current) setSourceReason(message(cause)); }
 };
 const editLatest = () => {
  if (!conflict || !draft) return;
  changeDraft({ text: draft.text, base: conflict.revision }); setConflict(null); setEditing(true); intent.current = null;
 };
 const copyDraft = async () => {
  if (!draft) return;
  try { await navigator.clipboard.writeText(draft.text); setNotice(copy('copied')); }
  catch (cause) { setError(message(cause)); }
 };
 const metadata = (value: RecapHead) => <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
  <Badge variant="outline">{copy('revision')} {value.revision}</Badge>
  <span>{copy(`cause.${value.cause || 'uninitialized'}`)}</span>
  {value.revision > 0 && <time dateTime={value.updated_at}>{value.updated_at}</time>}
  <span>{copy('coverage')} {value.covered_through}</span>
 </div>;
 return <div className="space-y-4">
  <Tabs value={tab} onValueChange={value => selectTab(value as Tab)}>
   <TabsList aria-label={copy('tabs')}>
    <TabsTrigger value="active" onClick={() => selectTab('active')}>{copy('active')}</TabsTrigger>
    <TabsTrigger value="history" onClick={() => selectTab('history')}>{copy('history')}</TabsTrigger>
    <TabsTrigger value="long-term" onClick={() => selectTab('long-term')}>{copy('longTerm')}</TabsTrigger>
   </TabsList>
   <TabsContent value="long-term"><MemoryView /></TabsContent>
   <TabsContent value="active" className="space-y-4">
    <div className="flex flex-wrap items-center gap-2">
     <Button variant="outline" disabled={loading || busy || !session} onClick={() => void refresh()}>{copy('refresh')}</Button>
     {draft && <Badge>{copy('unsaved')}</Badge>}
     {loading && <span role="status">{copy('loading')}</span>}
    </div>
    {!session && <p role="status">{copy('unavailableSession')}</p>}
    {head && <Card><CardContent className="space-y-4 pt-5">
     {metadata(head)}
     <p className="text-sm text-muted-foreground">{copy('nextRun')} · {loaded ? `${copy('loadedRevision')} ${loaded.revision} · ${loaded.run_id}` : copy('loadedUnavailable')} · {copy('consolidationUnavailable')}</p>
     {editing && draft ? <>
      <label className="block text-sm" htmlFor="recap-draft">{copy('draft')} · {copy('revision')} {draft.base}</label>
      <Textarea id="recap-draft" className="min-h-80 font-mono whitespace-pre-wrap" value={draft.text} onChange={event => { changeDraft({ text: event.target.value, base: draft.base }); intent.current = null; }} disabled={busy} />
      {head.revision !== draft.base && <p role="status">{copy('remoteChanged')}</p>}
      <div className="flex gap-2"><Button disabled={busy} onClick={() => void write('edit')}>{copy('save')}</Button><Button variant="outline" disabled={busy} onClick={() => setEditing(false)}>{copy('cancel')}</Button></div>
     </> : <>
      <pre data-testid="recap-full-text" className="min-h-48 whitespace-pre-wrap break-words font-sans text-sm">{head.text}</pre>
      <Button disabled={busy} onClick={() => { if (!draft) changeDraft({ text: head.text, base: head.revision }); setEditing(true); }}>{copy('edit')}</Button>
     </>}
    </CardContent></Card>}
    {!head && draft && <Card><CardContent className="space-y-3 pt-5">
     <p>{copy('draftUnavailable')}</p><label htmlFor="recap-retained-draft">{copy('draft')}</label>
     <Textarea id="recap-retained-draft" className="min-h-80 font-mono" value={draft.text} onChange={event=>{changeDraft({text:event.target.value,base:draft.base});intent.current=null;}} disabled={busy}/>
     <div className="flex gap-2"><Button disabled>{copy('save')}</Button><Button variant="outline" onClick={()=>void copyDraft()}>{copy('copyDraft')}</Button></div>
    </CardContent></Card>}
    <div className="flex flex-wrap gap-2">
     <Button variant="outline" disabled={!head || busy} onClick={() => head && setDialog({kind:'clear',base:head.revision})}>{copy('clear')}</Button>
     <Button variant="outline" disabled={!head || head.revision === 0 || busy} onClick={() => void showSources()}>{copy('sources')}</Button>
     <Button variant="ghost" onClick={() => void host?.router.navigate({ to: '/persona' })}>{copy('persona')}</Button>
    </div>
   </TabsContent>
   <TabsContent value="history" className="space-y-4">
    <p className="text-sm text-muted-foreground">{copy('historyExplanation')}</p>
    <div className="flex flex-wrap gap-2">{history.map(item => <Button key={item.revision} variant={preview?.revision === item.revision ? 'secondary' : 'outline'} data-revision={item.revision} onClick={() => void selectRevision(item.revision)}>{copy('revision')} {item.revision} · {copy(`cause.${item.cause}`)}</Button>)}</div>
    {nextBefore && <Button variant="outline" onClick={() => void loadHistory(nextBefore)}>{copy('more')}</Button>}
    {preview && <Card><CardContent className="space-y-4 pt-5">{metadata(preview)}<pre data-testid="recap-history-text" className="whitespace-pre-wrap break-words text-sm font-sans">{preview.text}</pre><div className="flex gap-2"><Button disabled={busy || !head} onClick={() => head && setDialog({kind:'restore',base:head.revision,revision:preview.revision})}>{copy('restore')}</Button><Button variant="outline" disabled={busy} onClick={() => void showSources()}>{copy('sources')}</Button></div></CardContent></Card>}
   </TabsContent>
  </Tabs>
  {conflict && <section aria-label={copy('conflict')} className="space-y-3 rounded-lg border border-amber-500/50 p-4">
   <h3 className="font-semibold">{copy('conflict')}</h3>{conflictIntent && <p>{copy(conflictIntent.kind === 'clear' ? 'conflictClear' : 'conflictRestore')} · {copy('revision')} {conflictIntent.base}{conflictIntent.revision && ` → ${conflictIntent.revision}`}</p>}
   <div className="grid gap-4 md:grid-cols-2"><div><h4>{copy('draft')}</h4><pre className="whitespace-pre-wrap break-words text-sm">{draft?.text ?? copy('noDraft')}</pre></div><div><h4>{copy('latest')} · {conflict.revision}</h4><pre className="whitespace-pre-wrap break-words text-sm">{conflict.text}</pre></div></div>
   <div className="flex flex-wrap gap-2"><Button disabled={!draft} onClick={() => void copyDraft()}>{copy('copyDraft')}</Button><Button variant="outline" onClick={() => setDialog({kind:'reload'})}>{copy('reload')}</Button><Button variant="outline" disabled={!draft} onClick={editLatest}>{copy('editLatest')}</Button></div>
  </section>}
  {(sourceReason || sources) && <section aria-label={copy('sources')} className="space-y-3 rounded-lg border p-4"><h3 className="font-semibold">{copy('sources')} · {copy('revision')} {sourceRevision}</h3>{sourceReason && <p role="status" data-reason={sourceReason}>{copy(sourceReason === 'loading' ? 'loading' : sourceReason === 'recap_scope_denied' ? 'sourceDenied' : 'sourceUnavailable')}{!['loading','recap_sources_unavailable','recap_scope_denied'].includes(sourceReason) && ` · ${sourceReason}`}</p>}{sources?.map(source => <div key={source.interaction_id}><p className="text-xs text-muted-foreground">{source.interaction_id} · {copy('revision')} {source.source_revision ?? sourceRevision}</p>{source.messages.map(item => <article key={item.id} data-testid="recap-source-message" className="my-3 rounded border p-3"><p className="text-xs text-muted-foreground">{item.role} · {item.id} {item.tool_name} {item.tool_status} {item.disposition}</p><pre className="whitespace-pre-wrap break-words font-sans text-sm">{item.content}</pre>{item.tool_arguments && <pre className="whitespace-pre-wrap break-words text-sm">{item.tool_arguments}</pre>}
    {item.tool_error && <pre className="whitespace-pre-wrap break-words text-sm">{item.tool_error}</pre>}
    {item.attachments?.map((attachment, index) => <a key={index} className="block underline" href={attachment.data_url} download={attachment.name}>{attachment.name} · {attachment.mime_type}</a>)}
    {item.file_contexts?.map((file, index) => <details key={index}><summary>{file.name} · {file.path}</summary><pre className="whitespace-pre-wrap break-words text-sm">{file.content}</pre></details>)}
    {item.references?.map(reference => <p key={reference.id}>{reference.name} · {reference.id}</p>)}</article>)}
    {source.references?.map(reference => <details key={reference.id}><summary>{reference.source_workspace} · {reference.id}</summary>{reference.items.map((item, index) => <article key={index}><p>{item.author}</p><pre className="whitespace-pre-wrap break-words text-sm">{item.text}</pre></article>)}</details>)}
    {source.corrections?.map((correction, index) => <p key={index} className="text-xs">{correction.view_session_id} · {correction.reason} · {correction.message_ids.join(', ')} → {correction.view_message_ids.join(', ')}</p>)}
   </div>)}</section>}
  {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
  {notice && <p role="status" className="text-sm text-muted-foreground">{notice}</p>}
  <Dialog open={dialog !== null} onOpenChange={open => { if (!open && !busy) setDialog(null); }}>
   <DialogContent><DialogHeader><DialogTitle>{copy(dialog?.kind === 'clear' ? 'clear' : dialog?.kind === 'restore' ? 'restore' : 'reload')}</DialogTitle></DialogHeader>
    <DialogBody><p>{copy(dialog?.kind === 'clear' ? 'clearExplanation' : dialog?.kind === 'restore' ? 'restoreExplanation' : 'reloadExplanation')}</p>{draft && dialog?.kind !== 'reload' && <p>{copy('draftRetained')}</p>}</DialogBody>
    <DialogFooter><Button variant="outline" disabled={busy} onClick={() => setDialog(null)}>{copy('dismiss')}</Button><Button disabled={busy} onClick={() => { if (dialog?.kind === 'clear') void write('clear',dialog as WriteIntent); else if (dialog?.kind === 'restore') void write('restore',dialog as WriteIntent); else { changeDraft(null); setEditing(false); setConflict(null); setConflictIntent(null); setDialog(null); intent.current = null; void refresh(); } }}>{copy(dialog?.kind === 'clear' ? 'confirmClear' : dialog?.kind === 'restore' ? 'confirmRestore' : 'confirmReload')}</Button></DialogFooter>
   </DialogContent>
  </Dialog>
 </div>;
}
