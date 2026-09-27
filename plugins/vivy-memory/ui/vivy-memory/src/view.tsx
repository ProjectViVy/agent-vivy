import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { AlertTriangle, Brain, Plus, Search, Trash2 } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { MasterDetail } from '@/components/layout/MasterDetail';
import { Separator } from '@/components/ui/separator';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { dateTimeLocale } from '@/i18n';
import { usePluginHost, usePluginTranslation, type UITranslator } from '@vivy/ui-sdk';
import {
  MemoryClient,
  type MemoryEntry,
  type MemoryOutcome,
  type MemoryStatusOutcome,
} from './memory-client';
import { MemoryContentDialog, MemoryDeleteDialog } from './dialogs';
import { MemoryRulesPanel } from './rules';

const SEARCH_DEBOUNCE_MS = 250;

const KNOWN_TRUST = new Set(['applied_authority', 'user_asserted', 'observed', 'inferred', 'untrusted', 'unknown']);

type DialogState =
  | { readonly kind: 'add' }
  | { readonly kind: 'edit'; readonly entry: MemoryEntry }
  | { readonly kind: 'delete'; readonly entry: MemoryEntry }
  | null;

export function MemoryView() {
  const host = usePluginHost();
  const { t } = usePluginTranslation();
  const client = useMemo(() => (host ? MemoryClient.fromRPC(host.rpc) : null), [host]);
  const [memories, setMemories] = useState<MemoryEntry[]>([]);
  const [query, setQuery] = useState('');
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tab, setTab] = useState<'records' | 'rules'>('records');
  const [dialog, setDialog] = useState<DialogState>(null);
  const [status, setStatus] = useState<MemoryStatusOutcome | null>(null);
  const requestSeq = useRef(0);

  // The search box drives vivy.memory.search; an empty box lists. A failed
  // invoke surfaces as an error state — never as fabricated rows.
  const load = useCallback(async (needle: string) => {
    if (!client) return;
    const seq = ++requestSeq.current;
    setLoading(true);
    setError(null);
    try {
      const outcome = needle ? await client.search({ query: needle }) : await client.list({});
      if (seq !== requestSeq.current) return;
      if (outcome.status === 'listed') {
        const entries = [...(outcome.entries ?? [])];
        setMemories(entries);
        setSelectedId((current) => (current && entries.some((item) => item.id === current) ? current : null));
      } else {
        setError(outcome.reason ?? outcome.status);
      }
    } catch (cause) {
      if (seq !== requestSeq.current) return;
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      if (seq === requestSeq.current) setLoading(false);
    }
  }, [client]);

  const loadStatus = useCallback(async () => {
    if (!client) return;
    try {
      const outcome = await client.status();
      if (outcome.status === 'listed') setStatus(outcome);
    } catch {
      // Status is informational; a failing list read already covers the outage.
    }
  }, [client]);

  useEffect(() => { void loadStatus(); }, [loadStatus]);

  const firstLoad = useRef(true);
  useEffect(() => {
    const needle = query.trim();
    if (firstLoad.current) {
      firstLoad.current = false;
      void load(needle);
      return undefined;
    }
    const timer = setTimeout(() => { void load(needle); }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [load, query]);

  const selected = memories.find((item) => item.id === selectedId) ?? null;

  // Write outcomes reload the authoritative list; add selects the new record.
  const onApplied = useCallback(async (outcome: MemoryOutcome) => {
    setDialog(null);
    if (outcome.status === 'applied' && outcome.entry) {
      setSelectedId(outcome.entry.id);
    } else if (dialog?.kind === 'delete') {
      setSelectedId(null);
    }
    await load(query.trim());
  }, [dialog, load, query]);

  // A CAS conflict rebases the dialog onto the freshest record; the caller
  // retries against the new base revision instead of overwriting.
  const onRebase = useCallback(async (id: string) => {
    if (!client) return;
    try {
      const outcome = await client.get({ id });
      if (outcome.status === 'listed' && outcome.entries?.[0]) {
        const fresh = outcome.entries[0];
        setDialog((current) => (current && current.kind !== 'add' ? { kind: current.kind, entry: fresh } : current));
        return;
      }
      // The record is gone (deleted elsewhere): close and refresh the list.
      setDialog(null);
      setSelectedId(null);
      await load(query.trim());
    } catch {
      setDialog(null);
      await load(query.trim());
    }
  }, [client, load, query]);

  if (!host || !client) {
    return (
      <div className="h-full overflow-auto p-6">
        <p className="text-sm text-muted-foreground">{t('plugin.vivy/memory.unavailable')}</p>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <Tabs value={tab} onValueChange={(value) => setTab(value as typeof tab)} className="flex h-full min-h-0 flex-col">
        <div className="flex items-center gap-2 border-b px-3 py-2">
          <TabsList className="h-8">
            <TabsTrigger value="records" className="text-xs">{t('plugin.vivy/memory.tabRecords')}</TabsTrigger>
            <TabsTrigger value="rules" className="text-xs">{t('plugin.vivy/memory.tabRules')}</TabsTrigger>
          </TabsList>
          <div className="ml-auto flex items-center gap-2">
            {status ? (
              <span className="hidden text-xs text-muted-foreground sm:inline">
                {t('plugin.vivy/memory.statusLine', { revision: status.startup_revision ?? 0 })}
                {status.database_present ? '' : ` · ${t('plugin.vivy/memory.noDatabase')}`}
              </span>
            ) : null}
            {tab === 'records' ? (
              <Button size="sm" variant="outline" onClick={() => setDialog({ kind: 'add' })}>
                <Plus className="mr-1.5 h-3.5 w-3.5" />{t('plugin.vivy/memory.addTitle')}
              </Button>
            ) : null}
          </div>
        </div>
        <TabsContent value="records" className="mt-0 min-h-0 flex-1">
          {error ? (
            <div className="h-full overflow-auto p-6">
              <div className="mx-auto max-w-xl">
                <div className="rounded-xl border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive" role="alert">
                  <div className="flex items-center gap-2">
                    <AlertTriangle className="h-4 w-4" />
                    <strong>{t('plugin.vivy/memory.loadFailed')}</strong>
                  </div>
                  <p className="mt-1">{error}</p>
                  <Button className="mt-3" size="sm" variant="outline" onClick={() => void load(query.trim())}>{t('common.retry')}</Button>
                </div>
              </div>
            </div>
          ) : (
            <MasterDetail
              selected={selectedId !== null}
              onBack={() => setSelectedId(null)}
              master={
                <section className="flex h-full min-h-0 flex-col border-b bg-sidebar md:border-b-0 md:border-r">
                  <div className="border-b p-3">
                    <div className="relative">
                      <Search className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                      <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t('plugin.vivy/memory.searchPlaceholder')} className="pl-9" />
                    </div>
                  </div>
                  <div className="min-h-0 flex-1 overflow-auto p-2">
                    {loading ? (
                      <div className="space-y-2 p-2">
                        <div className="h-16 animate-pulse rounded-lg bg-muted" />
                        <div className="h-16 animate-pulse rounded-lg bg-muted" />
                      </div>
                    ) : memories.length ? memories.map((item) => (
                      <button
                        key={item.id}
                        type="button"
                        onClick={() => setSelectedId(item.id)}
                        className={`mb-1 w-full rounded-lg p-3 text-left ${selectedId === item.id ? 'bg-sidebar-accent' : 'hover:bg-sidebar-accent/50'}`}
                      >
                        <div className="flex items-center justify-between gap-2">
                          <span className="truncate text-sm font-medium">{firstLine(item.content)}</span>
                          <Badge variant="outline">{trustLabel(t, item.trust)}</Badge>
                        </div>
                        <p className="mt-1 truncate text-xs text-muted-foreground">{formatDateTime(item.updated_at)}</p>
                      </button>
                    )) : (
                      <p className="px-3 py-10 text-center text-sm text-muted-foreground">{query ? t('plugin.vivy/memory.noMatch') : t('plugin.vivy/memory.empty')}</p>
                    )}
                  </div>
                </section>
              }
              detail={
                selected ? (
                  <div className="h-full overflow-auto p-4 sm:p-6">
                    <Card className="mx-auto max-w-3xl">
                      <CardHeader>
                        <CardTitle className="flex items-center gap-2">
                          <Brain className="h-5 w-5 shrink-0 text-primary" />
                          <span className="min-w-0 truncate">{firstLine(selected.content)}</span>
                          <span className="ml-auto flex shrink-0 gap-2">
                            <Button size="sm" variant="outline" onClick={() => setDialog({ kind: 'edit', entry: selected })}>{t('common.edit')}</Button>
                            <Button size="sm" variant="outline" onClick={() => setDialog({ kind: 'delete', entry: selected })}>
                              <Trash2 className="h-3.5 w-3.5" />
                              <span className="sr-only">{t('common.delete')}</span>
                            </Button>
                          </span>
                        </CardTitle>
                      </CardHeader>
                      <CardContent>
                        <div className="flex flex-wrap items-center gap-2">
                          <Badge>{trustLabel(t, selected.trust)}</Badge>
                          {selected.provenance ? <Badge variant="outline">{t('plugin.vivy/memory.provenance', { source: selected.provenance })}</Badge> : null}
                        </div>
                        <p className="mt-4 leading-7 break-words whitespace-pre-wrap">{selected.content}</p>
                        {selected.evidence_refs.length ? (
                          <>
                            <Separator className="my-4" />
                            <p className="text-xs font-medium text-muted-foreground">{t('plugin.vivy/memory.evidence')}</p>
                            <ul className="mt-2 space-y-1.5">
                              {selected.evidence_refs.map((ref) => (
                                <li key={ref.id} className="text-xs text-muted-foreground">
                                  <span className="font-medium text-foreground">{ref.source}</span>{' '}
                                  <span className="break-all">{ref.uri}</span>
                                  {ref.excerpt ? <span className="block break-words">{ref.excerpt}</span> : null}
                                </li>
                              ))}
                            </ul>
                          </>
                        ) : null}
                        <Separator className="my-4" />
                        <p className="text-xs text-muted-foreground">
                          <span className="break-all font-mono">{selected.id}</span>
                        </p>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {t('plugin.vivy/memory.updatedAt', { date: formatDateTime(selected.updated_at) })}
                          {' · '}
                          {t('plugin.vivy/memory.createdAt', { date: formatDateTime(selected.created_at) })}
                          {' · '}
                          {t('plugin.vivy/memory.revision', { revision: selected.revision })}
                        </p>
                      </CardContent>
                    </Card>
                  </div>
                ) : (
                  <div className="flex h-full items-center justify-center p-6 text-muted-foreground">{t('plugin.vivy/memory.selectHint')}</div>
                )
              }
            />
          )}
        </TabsContent>
        <TabsContent value="rules" className="mt-0 min-h-0 flex-1">
          <MemoryRulesPanel client={client} t={t} />
        </TabsContent>
      </Tabs>

      {dialog?.kind === 'add' ? (
        <MemoryContentDialog mode="add" client={client} t={t} onClose={() => setDialog(null)} onApplied={onApplied} onRebase={onRebase} />
      ) : null}
      {dialog?.kind === 'edit' ? (
        <MemoryContentDialog
          key={`${dialog.entry.id}@${dialog.entry.revision}`}
          mode="edit"
          entry={dialog.entry}
          client={client}
          t={t}
          onClose={() => setDialog(null)}
          onApplied={onApplied}
          onRebase={onRebase}
        />
      ) : null}
      {dialog?.kind === 'delete' ? (
        <MemoryDeleteDialog
          key={`${dialog.entry.id}@${dialog.entry.revision}`}
          entry={dialog.entry}
          client={client}
          t={t}
          onClose={() => setDialog(null)}
          onApplied={onApplied}
          onRebase={onRebase}
        />
      ) : null}
    </div>
  );
}

function formatDateTime(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString(dateTimeLocale());
}

function firstLine(content: string): string {
  const line = content.split('\n', 1)[0]?.trim() ?? '';
  return line || content.trim();
}

function trustLabel(t: UITranslator, trust: string): string {
  return KNOWN_TRUST.has(trust) ? t(`plugin.vivy/memory.trust.${trust}`) : trust;
}
