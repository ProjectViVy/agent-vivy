import { useCallback, useEffect, useState } from 'react';
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
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [checks, setChecks] = useState<Record<string, api.MarketplaceUpdateCheck>>({});

  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(query), 300);
    return () => window.clearTimeout(timer);
  }, [query]);

  const loadFeatured = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      setFeatured(await api.featuredMarketplaceSkills());
    } catch (err) {
      setError(err instanceof Error ? err.message : t('skills.marketplace.loadFailed'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  const search = useCallback(async (term: string) => {
    try {
      setLoading(true);
      setError(null);
      const data = await api.searchMarketplaceSkills(term);
      setResults(data.skills);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('skills.marketplace.loadFailed'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    const term = debouncedQuery.trim();
    if (term.length < 2) {
      setResults([]);
      if (featured === null) void loadFeatured();
      return;
    }
    void search(term);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedQuery]);

  const install = useCallback(async (id: string, mode?: 'create' | 'upgrade') => {
    try {
      setBusyId(id);
      setError(null);
      setNotice(null);
      const result = await api.installMarketplaceSkill(id, mode);
      setChecks((prev) => {
        const next = { ...prev };
        delete next[slugOf(id)];
        return next;
      });
      if (result.outcome === 'upgraded') setNotice(t('skills.marketplace.outcomeUpgraded'));
      else if (result.outcome === 'up_to_date') setNotice(t('skills.marketplace.outcomeUpToDate'));
      onInstalledChange();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('skills.marketplace.installFailed'));
    } finally {
      setBusyId(null);
    }
  }, [onInstalledChange, t]);

  const checkUpdate = useCallback(async (id: string) => {
    const slug = slugOf(id);
    try {
      setBusyId(id);
      setError(null);
      setNotice(null);
      const check = await api.checkMarketplaceUpdate(slug);
      setChecks((prev) => ({ ...prev, [slug]: check }));
    } catch (err) {
      setError(err instanceof Error ? err.message : t('skills.marketplace.checkFailed'));
    } finally {
      setBusyId(null);
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

      {error && (
        <Card>
          <CardContent className="flex items-center justify-between gap-3 py-3 text-sm">
            <span className="min-w-0 break-all text-destructive">{error}</span>
            <Button variant="outline" size="sm" onClick={() => void (debouncedQuery.trim().length >= 2 ? search(debouncedQuery.trim()) : loadFeatured())}>
              {t('common.retry')}
            </Button>
          </CardContent>
        </Card>
      )}
      {notice && (
        <Card>
          <CardContent className="py-3 text-sm">{notice}</CardContent>
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
              const busy = busyId === entry.id;
              return (
                <Card key={entry.id}>
                  <CardContent className="flex items-center justify-between gap-3 py-3">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="min-w-0 truncate font-medium">{entry.name}</span>
                        <Badge variant="secondary" className="shrink-0">{formatCount(entry.installs)}</Badge>
                      </div>
                      <p className="mt-0.5 truncate text-xs text-muted-foreground">{entry.source}</p>
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
