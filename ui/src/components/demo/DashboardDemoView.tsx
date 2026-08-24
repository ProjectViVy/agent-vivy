import { useEffect, useState } from 'react';
import { Activity, Clock3, MessageSquare, ShieldCheck, Waypoints } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { getDemoDashboard } from '@/lib/demo-api';
import type { DemoDashboardSnapshot } from '@/lib/types';
import { DemoLoadError } from './DemoBanner';

export function DashboardDemoView() {
  const [snapshot, setSnapshot] = useState<DemoDashboardSnapshot | null>(null);
  const [error, setError] = useState<string | null>(null);
  const load = async () => { setError(null); try { setSnapshot(await getDemoDashboard()); } catch (cause) { setError(cause instanceof Error ? cause.message : String(cause)); } };
  useEffect(() => { void load(); }, []);
  const metrics = snapshot ? [
    { label: '会话', value: snapshot.sessionCount, icon: MessageSquare },
    { label: '活跃运行', value: snapshot.activeRuns, icon: Activity },
    { label: '待处理 Review', value: snapshot.pendingReviews, icon: ShieldCheck },
    { label: 'Token 使用', value: snapshot.tokenUsage.toLocaleString(), icon: Waypoints },
  ] : [];
  return <div className="h-full overflow-auto p-6"><div className="mx-auto max-w-6xl"><div><h1 className="text-2xl font-bold">中控台</h1><p className="mt-1 text-sm text-muted-foreground">Vivy 运行概览的本地演示快照。</p></div>{error ? <div className="mt-6"><DemoLoadError message={error} onRetry={() => void load()}/></div> : <><div className="mt-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">{snapshot ? metrics.map(({ label, value, icon: Icon }) => <Card key={label}><CardContent className="flex items-center justify-between p-5"><div><p className="text-sm text-muted-foreground">{label}</p><p className="mt-1 text-2xl font-bold">{value}</p></div><Icon className="h-7 w-7 text-primary"/></CardContent></Card>) : Array.from({ length: 4 }, (_, index) => <div key={index} className="h-28 animate-pulse rounded-xl bg-muted"/>)}</div><Card className="mt-6"><CardHeader><CardTitle>近期活动</CardTitle></CardHeader><CardContent className="divide-y">{snapshot?.recentActivity.map((item) => <div key={item.id} className="flex items-start gap-3 py-4 first:pt-0 last:pb-0"><Clock3 className="mt-0.5 h-4 w-4 text-muted-foreground"/><div className="min-w-0 flex-1"><p className="font-medium">{item.title}</p><p className="text-sm text-muted-foreground">{item.detail}</p></div><span className="text-xs text-muted-foreground">{item.occurredAt}</span></div>)}</CardContent></Card></>}</div></div>;
}
