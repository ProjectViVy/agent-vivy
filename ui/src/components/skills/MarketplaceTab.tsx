import { useCallback, useEffect, useRef, useState } from 'react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { useTranslation } from '@/i18n';
import { Download, LoaderCircle, PackageOpen, RefreshCw, Search, SearchCheck } from 'lucide-react';
import * as api from '@/lib/api';

interface MarketplaceTabProps {
  installedNames: string[];
  onInstalledChange: () => void;
}

/** skills.sh slug — the last segment of `owner/repo/slug`. */
function slugOf(id: string): string {
  return id.split('/')[2] ?? id;
}

function formatCount(count: number): string {
  if (count >= 1_000_000) return `${(count / 1_000_000).toFixed(1)}m`;
  if (count >= 1_000) return `${(count / 1_000).toFixed(1)}k`;
  return String(count);
}

export function MarketplaceTab({ installedNames, onInstalledChange }: MarketplaceTabProps) {
  const { t } = useTranslation();
  const [query, setQuery] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const [results, setResults] = useState<api.MarketplaceSkill[]>([]);
  const [featured, setFeatured] = useState<api.MarketplaceFeatured | null>(null);
  const [loading, setLoading] = useState(false);
	const [queryError, setQueryError] = useState<string | null>(null);
	const [operations, setOperations] = useState<Record<string, { busy: boolean; message?: string; error?: string }>>({});
	const requestId = useRef(0);
  const [checks, setChecks] = useState<Record<string, api.MarketplaceUpdateCheck>>({});

  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(query), 300);
    return () => window.clearTimeout(timer);
  }, [query]);

  const loadFeatured = useCallback(async () => {
	const id = ++requestId.current;
    try {
      setLoading(true);
	  setQueryError(null);
	  const next = await api.featuredMarketplaceSkills();
	  if (id === requestId.current) setFeatured(next);
    } catch (err) {
	  if (id === requestId.current) setQueryError(err instanceof Error ? err.message : t('skills.marketplace.loadFailed'));
    } finally {
	  if (id === requestId.current) setLoading(false);
    }
  }, [t]);

  const search = useCallback(async (term: string) => {
	const id = ++requestId.current;
    try {
      setLoading(true);
	  setQueryError(null);
      const data = await api.searchMarketplaceSkills(term);
	  if (id === requestId.current) setResults(data.skills);
    } catch (err) {
	  if (id === requestId.current) setQueryError(err instanceof Error ? err.message : t('skills.marketplace.loadFailed'));
    } finally {
	  if (id === requestId.current) setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    const term = debouncedQuery.trim();
    if (term.length < 2) {
      setResults([]);
	  if (featured === null) void loadFeatured();
	  else { requestId.current += 1; setLoading(false); setQueryError(null); }
      return;
    }
    void search(term);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedQuery]);

  const install = useCallback(async (id: string, mode?: 'create' | 'upgrade') => {
	const slug = slugOf(id);
    try {
	  setOperations((prev) => ({ ...prev, [slug]: { busy: true } }));
      const result = await api.installMarketplaceSkill(id, mode);
      setChecks((prev) => {
        const next = { ...prev };
        delete next[slugOf(id)];
        return next;
      });
	  const message = result.outcome === 'upgraded' ? t('skills.marketplace.outcomeUpgraded') : result.outcome === 'up_to_date' ? t('skills.marketplace.outcomeUpToDate') : undefined;
	  setOperations((prev) => ({ ...prev, [slug]: { busy: false, message } }));
      onInstalledChange();
    } catch (err) {
	  setOperations((prev) => ({ ...prev, [slug]: { busy: false, error: err instanceof Error ? err.message : t('skills.marketplace.installFailed') } }));
    }
  }, [onInstalledChange, t]);

  const checkUpdate = useCallback(async (id: string) => {
    const slug = slugOf(id);
    try {
	  setOperations((prev) => ({ ...prev, [slug]: { busy: true } }));
      const check = await api.checkMarketplaceUpdate(slug);
      setChecks((prev) => ({ ...prev, [slug]: check }));
	  setOperations((prev) => ({ ...prev, [slug]: { busy: false } }));
    } catch (err) {
	  setOperations((prev) => ({ ...prev, [slug]: { busy: false, error: err instanceof Error ? err.message : t('skills.marketplace.checkFailed') } }));
    }
  }, [t]);

  const entries = debouncedQuery.trim().length >= 2 ? results : (featured?.skills ?? []);
  const snapshotDate = featured ? new Date(featured.generated_at).toLocaleDateString() : '';

  return (
    <div className="flex h-full min-h-0 flex-col gap-3">
      <div className="flex gap-2">
        <div className="relative min-w-0 flex-1">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={t('skills.marketplace.searchPlaceholder')}
            className="pl-9"
          />
        </div>
        <Button variant="outline" size="icon" disabled={loading} onClick={() => void (debouncedQuery.trim().length >= 2 ? search(debouncedQuery.trim()) : loadFeatured())}>
          <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
        </Button>
      </div>

      <p className="text-xs text-muted-foreground">
        {debouncedQuery.trim().length < 2
          ? t('skills.marketplace.searchHint')
          : t('skills.marketplace.resultsHint', { count: results.length })}
        {debouncedQuery.trim().length < 2 && featured && ` · ${t('skills.marketplace.featuredSnapshot', { date: snapshotDate })}`}
      </p>

	  {queryError && (
        <Card>
          <CardContent className="flex items-center justify-between gap-3 py-3 text-sm">
			<span className="min-w-0 break-all text-destructive">{queryError}</span>
            <Button variant="outline" size="sm" onClick={() => void (debouncedQuery.trim().length >= 2 ? search(debouncedQuery.trim()) : loadFeatured())}>
              {t('common.retry')}
            </Button>
          </CardContent>
        </Card>
      )}
      <div className="min-h-0 flex-1 overflow-auto">
        {loading && entries.length === 0 ? (
          <div className="space-y-2">
            {[1, 2, 3].map((row) => (
              <Card key={row}><CardContent className="py-4 text-sm text-muted-foreground">…</CardContent></Card>
            ))}
          </div>
        ) : entries.length === 0 ? (
          <Card>
            <CardContent className="py-12 text-center text-muted-foreground">
              <PackageOpen className="mx-auto mb-3 h-10 w-10 opacity-50" />
              {debouncedQuery.trim().length >= 2 ? t('skills.marketplace.noResults') : t('skills.marketplace.empty')}
            </CardContent>
          </Card>
        ) : (
          <div className="space-y-2">
            {entries.map((entry) => {
              const slug = slugOf(entry.id);
              const installed = installedNames.includes(slug);
              const check = checks[slug];
			  const operation = operations[slug];
			  const busy = operation?.busy ?? false;
              return (
                <Card key={entry.id}>
                  <CardContent className="flex items-center justify-between gap-3 py-3">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="min-w-0 truncate font-medium">{entry.name}</span>
                        <Badge variant="secondary" className="shrink-0">{formatCount(entry.installs)}</Badge>
                      </div>
                      <p className="mt-0.5 truncate text-xs text-muted-foreground">{entry.source}</p>
					  {operation?.error ? <p className="mt-1 text-xs text-destructive" role="alert">{operation.error}</p> : null}
					  {operation?.message ? <p className="mt-1 text-xs text-muted-foreground">{operation.message}</p> : null}
                    </div>
                    {!installed ? (
                      <Button size="sm" disabled={busy} onClick={() => void install(entry.id)}>
                        {busy ? <LoaderCircle className="mr-2 h-4 w-4 animate-spin" /> : <Download className="mr-2 h-4 w-4" />}
                        {busy ? t('skills.marketplace.installing') : t('skills.marketplace.install')}
                      </Button>
                    ) : check?.status === 'upgrade_available' ? (
                      <Button size="sm" disabled={busy} onClick={() => void install(entry.id, 'upgrade')}>
                        {busy ? <LoaderCircle className="mr-2 h-4 w-4 animate-spin" /> : <Download className="mr-2 h-4 w-4" />}
                        {busy ? t('skills.marketplace.upgrading') : t('skills.marketplace.upgrade')}
                      </Button>
                    ) : (
                      <div className="flex shrink-0 items-center gap-2">
                        {check?.status === 'up_to_date' ? <Badge variant="outline">{t('skills.marketplace.upToDate')}</Badge> : null}
                        {check?.status === 'unmanaged' ? <span className="max-w-40 text-right text-xs text-muted-foreground">{t('skills.marketplace.unmanagedHint')}</span> : null}
                        <Button variant="outline" size="sm" disabled={busy} onClick={() => void checkUpdate(entry.id)}>
                          {busy ? <LoaderCircle className="mr-2 h-4 w-4 animate-spin" /> : <SearchCheck className="mr-2 h-4 w-4" />}
                          {t('skills.marketplace.checkUpdate')}
                        </Button>
                      </div>
                    )}
                  </CardContent>
                </Card>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
