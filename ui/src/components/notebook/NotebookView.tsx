import { useEffect, useState } from 'react';
import { BookOpen, Calendar, FileText, Search, Sparkles } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { generateNotebookReport, getNotebookReports, searchSessions } from '@/lib/demo-api';
import type { NotebookReport, ReportPeriod, SessionSearchHit } from '@/lib/types';
import { cn } from '@/lib/utils';

const PERIODS: Array<{ value: ReportPeriod; label: string }> = [{ value: 'daily', label: '日报' }, { value: 'weekly', label: '周报' }, { value: 'monthly', label: '月报' }];

export function NotebookView() {
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
  useEffect(() => { let current = true; setLoadingReports(true); setReportError(null); void getNotebookReports(period).then((items) => { if (current) { setReports(items); setSelectedReport(items[0] ?? null); } }).catch((cause) => { if (current) setReportError(cause instanceof Error ? cause.message : String(cause)); }).finally(() => { if (current) setLoadingReports(false); }); return () => { current = false; }; }, [period, reloadKey]);
  const generate = async () => { setGenerating(true); setReportError(null); try { const report = await generateNotebookReport(period); setReports((items) => [report, ...items]); setSelectedReport(report); } catch (cause) { setReportError(cause instanceof Error ? cause.message : String(cause)); } finally { setGenerating(false); } };
  const search = async () => { const next = query.trim(); if (!next) return; setSearching(true); setSearchError(null); try { const response = await searchSessions(next); setResults(response.hits); setSubmittedQuery(next); } catch (cause) { setSearchError(cause instanceof Error ? cause.message : String(cause)); } finally { setSearching(false); } };
  return <div className="flex h-full min-h-0 flex-col md:flex-row">
    <aside className="flex h-80 w-full shrink-0 flex-col border-b bg-sidebar md:h-auto md:w-80 md:border-b-0 md:border-r"><Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as typeof activeTab)} className="flex min-h-0 flex-1 flex-col"><div className="border-b p-3"><TabsList className="w-full"><TabsTrigger value="reports" className="flex-1">报告</TabsTrigger><TabsTrigger value="search" className="flex-1">搜索</TabsTrigger></TabsList></div>
      <TabsContent value="reports" className="m-0 flex min-h-0 flex-1 flex-col"><div className="flex items-center gap-2 border-b p-3"><Tabs value={period} onValueChange={(value) => setPeriod(value as ReportPeriod)} className="min-w-0 flex-1"><TabsList className="w-full">{PERIODS.map((item) => <TabsTrigger key={item.value} value={item.value} className="flex-1 text-xs">{item.label}</TabsTrigger>)}</TabsList></Tabs><Button size="icon" variant="outline" title={`生成${PERIODS.find((item) => item.value === period)?.label}`} disabled={generating} onClick={() => void generate()}><Sparkles className="h-4 w-4"/></Button></div><ScrollArea className="min-h-0 flex-1 p-2">{reportError ? <div className="p-3 text-sm text-destructive"><p>{reportError}</p><Button className="mt-2" size="sm" variant="outline" onClick={() => setReloadKey((value) => value + 1)}>重试</Button></div> : loadingReports ? <div className="space-y-2 p-2"><div className="h-16 animate-pulse rounded-lg bg-muted"/><div className="h-16 animate-pulse rounded-lg bg-muted"/></div> : reports.length ? reports.map((report) => <button key={report.id} onClick={() => setSelectedReport(report)} className={cn('mb-1 w-full rounded-lg p-3 text-left', selectedReport?.id === report.id ? 'bg-sidebar-accent' : 'hover:bg-sidebar-accent/50')}><div className="flex items-center gap-2"><FileText className="h-3.5 w-3.5"/><span className="truncate text-sm font-medium">{report.title}</span></div><p className="mt-1 truncate text-xs text-muted-foreground">{report.summary}</p><Badge variant="outline" className="mt-2">{report.date}</Badge></button>) : <p className="py-10 text-center text-sm text-muted-foreground">暂无报告</p>}</ScrollArea></TabsContent>
      <TabsContent value="search" className="m-0 flex min-h-0 flex-1 flex-col"><div className="flex gap-2 border-b p-3"><Input value={query} placeholder="搜索会话内容..." onChange={(event) => setQuery(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void search(); }}/><Button size="icon" title="搜索" disabled={searching || !query.trim()} onClick={() => void search()}><Search className="h-4 w-4"/></Button></div><ScrollArea className="min-h-0 flex-1 p-2">{searchError ? <div className="p-3 text-sm text-destructive"><p>{searchError}</p><Button className="mt-2" size="sm" variant="outline" onClick={() => void search()}>重试</Button></div> : searching ? <div className="space-y-2 p-2"><div className="h-20 animate-pulse rounded-lg bg-muted"/><div className="h-20 animate-pulse rounded-lg bg-muted"/></div> : results.length ? results.map((hit) => <article key={`${hit.session_id}-${hit.message_index}`} className="mb-2 rounded-lg border bg-card p-3"><div className="flex items-center gap-2 text-xs text-muted-foreground"><Calendar className="h-3.5 w-3.5"/>{new Date(hit.timestamp).toLocaleString()}</div><p className="mt-2 text-sm">{hit.snippet}</p></article>) : submittedQuery ? <p className="py-10 text-center text-sm text-muted-foreground">未找到“{submittedQuery}”</p> : <p className="py-10 text-center text-sm text-muted-foreground">输入关键词搜索本地演示会话</p>}</ScrollArea></TabsContent>
    </Tabs></aside>
    <main className="min-w-0 flex-1 overflow-auto">{selectedReport ? <div className="mx-auto max-w-3xl p-6"><div className="flex items-center gap-2"><BookOpen className="h-5 w-5 text-primary"/><h1 className="text-xl font-bold">{selectedReport.title}</h1></div><div className="my-4 flex flex-wrap gap-2"><Badge>{PERIODS.find((item) => item.value === selectedReport.period)?.label}</Badge><Badge variant="secondary">{selectedReport.date}</Badge>{selectedReport.generatedBy ? <Badge variant="outline">生成者: {selectedReport.generatedBy}</Badge> : null}</div><article className="rounded-xl border bg-card p-6"><pre className="whitespace-pre-wrap font-sans text-sm leading-7">{selectedReport.content}</pre></article></div> : <div className="flex h-full items-center justify-center text-muted-foreground"><div className="text-center"><BookOpen className="mx-auto mb-2 h-12 w-12 opacity-40"/><p>选择一份报告查看详情，或搜索会话内容</p></div></div>}</main>
  </div>;
}
