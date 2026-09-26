import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { AlertTriangle, Brain, Search } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { MasterDetail } from '@/components/layout/MasterDetail';
import { dateTimeLocale } from '@/i18n';
import { usePluginHost, usePluginTranslation, type UITranslator } from '@vivy/ui-sdk';
import { MemoryClient, type MemoryEntry } from './memory-client';

const SEARCH_DEBOUNCE_MS = 250;

const KNOWN_TRUST = new Set(['applied_authority', 'user_asserted', 'observed', 'inferred', 'untrusted', 'unknown']);

export function MemoryView() {
  const host = usePluginHost();
  const { t } = usePluginTranslation();
  const client = useMemo(() => (host ? MemoryClient.fromRPC(host.rpc) : null), [host]);
  const [memories, setMemories] = useState<MemoryEntry[]>([]);
  const [query, setQuery] = useState('');
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
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

  if (!host) {
    return (
      <div className="h-full overflow-auto p-6">
        <p className="text-sm text-muted-foreground">{t('plugin.vivy/memory.unavailable')}</p>
      </div>
    );
  }

  if (error) {
    return (
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
    );
  }

  return (
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
          <div className="p-4 sm:p-6">
            <Card className="mx-auto max-w-3xl">
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Brain className="h-5 w-5 shrink-0 text-primary" />
                  <span className="min-w-0 truncate">{firstLine(selected.content)}</span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <Badge>{trustLabel(t, selected.trust)}</Badge>
                <p className="mt-4 leading-7 break-words whitespace-pre-wrap">{selected.content}</p>
                <p className="mt-6 text-xs text-muted-foreground">
                  {t('plugin.vivy/memory.updatedAt', { date: formatDateTime(selected.updated_at) })}
                  {' · '}
                  {t('plugin.vivy/memory.revision', { revision: selected.revision })}
                  {selected.provenance ? ` · ${t('plugin.vivy/memory.provenance', { source: selected.provenance })}` : ''}
                </p>
              </CardContent>
            </Card>
          </div>
        ) : (
          <div className="flex h-full items-center justify-center p-6 text-muted-foreground">{t('plugin.vivy/memory.selectHint')}</div>
        )
      }
    />
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
