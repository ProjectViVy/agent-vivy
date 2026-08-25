import { useEffect, useState } from 'react';
import { BookOpen, Calendar, FileText, Search, Sparkles } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { MasterDetail } from '@/components/layout/MasterDetail';
import { generateNotebookReport, getNotebookReports, searchSessions } from '@/lib/demo-api';
import type { NotebookReport, ReportPeriod, SessionSearchHit } from '@/lib/types';
import { useTranslation } from '@/i18n';
import { cn } from '@/lib/utils';

const PERIODS: ReportPeriod[] = ['daily', 'weekly', 'monthly'];

export function NotebookView() {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<'reports' | 'search'>('reports');
  const [period, setPeriod] = useState<ReportPeriod>('daily');
  const [reports, setReports] = useState<NotebookReport[]>([]);
  const [selectedReport, setSelectedReport] = useState<NotebookReport | null>(null);
  const [loadingReports, setLoadingReports] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [reportError, setReportError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const [query, setQuery] = useState('');
  const [submittedQuery, setSubmittedQuery] = useState('');
  const [results, setResults] = useState<SessionSearchHit[]>([]);
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);

  useEffect(() => {
    let current = true;
    setLoadingReports(true);
    setReportError(null);
    void getNotebookReports(period).then((items) => {
      if (!current) return;
      setReports(items);
      setSelectedReport((currentReport) => {
        if (currentReport && items.some((item) => item.id === currentReport.id)) return currentReport;
        return null;
      });
    }).catch((cause) => {
      if (current) setReportError(cause instanceof Error ? cause.message : String(cause));
    }).finally(() => {
      if (current) setLoadingReports(false);
    });
    return () => { current = false; };
  }, [period, reloadKey]);

  const generate = async () => {
    setGenerating(true);
    setReportError(null);
    try {
      const report = await generateNotebookReport(period);
      setReports((items) => [report, ...items]);
      setSelectedReport(report);
    } catch (cause) {
      setReportError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setGenerating(false);
    }
  };

  const search = async () => {
    const next = query.trim();
    if (!next) return;
    setSearching(true);
    setSearchError(null);
    try {
      const response = await searchSessions(next);
      setResults(response.hits);
      setSubmittedQuery(next);
    } catch (cause) {
      setSearchError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setSearching(false);
    }
  };

  return (
    <MasterDetail
      selected={selectedReport !== null}
      onBack={() => setSelectedReport(null)}
      master={
        <aside className="flex h-full min-h-0 flex-col border-b bg-sidebar md:border-b-0 md:border-r">
          <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as typeof activeTab)} className="flex min-h-0 flex-1 flex-col">
            <div className="border-b p-3">
              <TabsList className="w-full">
                <TabsTrigger value="reports" className="flex-1">{t('notebook.reports')}</TabsTrigger>
                <TabsTrigger value="search" className="flex-1">{t('notebook.search')}</TabsTrigger>
              </TabsList>
            </div>
            <TabsContent value="reports" className="m-0 flex min-h-0 flex-1 flex-col">
              <div className="flex items-center gap-2 border-b p-3">
                <Tabs value={period} onValueChange={(value) => setPeriod(value as ReportPeriod)} className="min-w-0 flex-1">
                  <TabsList className="w-full">
                    {PERIODS.map((item) => <TabsTrigger key={item} value={item} className="flex-1 text-xs">{t(`notebook.periods.${item}`)}</TabsTrigger>)}
                  </TabsList>
                </Tabs>
                <Button size="icon" variant="outline" title={t('notebook.generate', { period: t(`notebook.periods.${period}`) })} disabled={generating} onClick={() => void generate()}>
                  <Sparkles className="h-4 w-4" />
                </Button>
              </div>
              <ScrollArea className="min-h-0 flex-1 p-2">
                {reportError ? (
                  <div className="p-3 text-sm text-destructive">
                    <p>{reportError}</p>
                    <Button className="mt-2" size="sm" variant="outline" onClick={() => setReloadKey((value) => value + 1)}>{t('common.retry')}</Button>
                  </div>
                ) : loadingReports ? (
                  <div className="space-y-2 p-2">
                    <div className="h-16 animate-pulse rounded-lg bg-muted" />
                    <div className="h-16 animate-pulse rounded-lg bg-muted" />
                  </div>
                ) : reports.length ? reports.map((report) => (
                  <button
                    key={report.id}
                    type="button"
                    onClick={() => setSelectedReport(report)}
                    className={cn('mb-1 w-full rounded-lg p-3 text-left', selectedReport?.id === report.id ? 'bg-sidebar-accent' : 'hover:bg-sidebar-accent/50')}
                  >
                    <div className="flex items-center gap-2">
                      <FileText className="h-3.5 w-3.5 shrink-0" />
                      <span className="truncate text-sm font-medium">{report.title}</span>
                    </div>
                    <p className="mt-1 truncate text-xs text-muted-foreground">{report.summary}</p>
                    <Badge variant="outline" className="mt-2">{report.date}</Badge>
                  </button>
                )) : <p className="py-10 text-center text-sm text-muted-foreground">{t('notebook.emptyReports')}</p>}
              </ScrollArea>
            </TabsContent>
            <TabsContent value="search" className="m-0 flex min-h-0 flex-1 flex-col">
              <div className="flex gap-2 border-b p-3">
                <Input value={query} placeholder={t('notebook.searchPlaceholder')} onChange={(event) => setQuery(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void search(); }} />
                <Button size="icon" title={t('notebook.searchButton')} disabled={searching || !query.trim()} onClick={() => void search()}>
                  <Search className="h-4 w-4" />
                </Button>
              </div>
              <ScrollArea className="min-h-0 flex-1 p-2">
                {searchError ? (
                  <div className="p-3 text-sm text-destructive">
                    <p>{searchError}</p>
                    <Button className="mt-2" size="sm" variant="outline" onClick={() => void search()}>{t('common.retry')}</Button>
                  </div>
                ) : searching ? (
                  <div className="space-y-2 p-2">
                    <div className="h-20 animate-pulse rounded-lg bg-muted" />
                    <div className="h-20 animate-pulse rounded-lg bg-muted" />
                  </div>
                ) : results.length ? results.map((hit) => (
                  <article key={`${hit.session_id}-${hit.message_index}`} className="mb-2 rounded-lg border bg-card p-3">
                    <div className="flex items-center gap-2 text-xs text-muted-foreground">
                      <Calendar className="h-3.5 w-3.5" />
                      {new Date(hit.timestamp).toLocaleString()}
                    </div>
                    <p className="mt-2 text-sm">{hit.snippet}</p>
                  </article>
                )) : submittedQuery ? (
                  <p className="py-10 text-center text-sm text-muted-foreground">{t('notebook.noResults', { query: submittedQuery })}</p>
                ) : (
                  <p className="py-10 text-center text-sm text-muted-foreground">{t('notebook.searchHint')}</p>
                )}
              </ScrollArea>
            </TabsContent>
          </Tabs>
        </aside>
      }
      detail={
        selectedReport ? (
          <div className="mx-auto max-w-3xl p-4 sm:p-6">
            <div className="flex items-center gap-2">
              <BookOpen className="h-5 w-5 shrink-0 text-primary" />
              <h1 className="min-w-0 text-xl font-bold">{selectedReport.title}</h1>
            </div>
            <div className="my-4 flex flex-wrap gap-2">
              <Badge>{t(`notebook.periods.${selectedReport.period}`)}</Badge>
              <Badge variant="secondary">{selectedReport.date}</Badge>
              {selectedReport.generatedBy ? <Badge variant="outline">{t('notebook.generatedBy', { name: selectedReport.generatedBy })}</Badge> : null}
            </div>
            <article className="rounded-xl border bg-card p-4 sm:p-6">
              <pre className="whitespace-pre-wrap break-words font-sans text-sm leading-7">{selectedReport.content}</pre>
            </article>
          </div>
        ) : (
          <div className="flex h-full items-center justify-center p-6 text-muted-foreground">
            <div className="text-center">
              <BookOpen className="mx-auto mb-2 h-12 w-12 opacity-40" />
              <p>{t('notebook.selectHint')}</p>
            </div>
          </div>
        )
      }
    />
  );
}
