import { useEffect, useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { AuditPanel } from '@/components/audit/AuditPanel';
import { getDemoDashboard } from '@/lib/demo-api';
import type { DemoDashboardSnapshot } from '@/lib/types';
import { DemoLoadError } from './DemoBanner';
import { TokenStatsPanel } from './TokenStatsPanel';

export function DashboardDemoView() {
  const [snapshot, setSnapshot] = useState<DemoDashboardSnapshot | null>(null);
  const [error, setError] = useState<string | null>(null);
  const load = async () => {
    setError(null);
    try {
      setSnapshot(await getDemoDashboard());
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  };
  useEffect(() => { void load(); }, []);

  return (
    <div className="h-full overflow-auto p-4 sm:p-6">
      <div className="mx-auto max-w-4xl">
        <h1 className="text-2xl font-bold">中控台</h1>
        <p className="mt-1 text-sm text-muted-foreground">运行状态、Token 用量与审计分区展示。</p>
        <Tabs defaultValue="overview" className="mt-6">
          <TabsList className="w-full justify-start overflow-x-auto">
            <TabsTrigger value="overview">概览</TabsTrigger>
            <TabsTrigger value="token">Token</TabsTrigger>
            <TabsTrigger value="audit">审计</TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="space-y-4">
            {error ? (
              <DemoLoadError message={error} onRetry={() => void load()} />
            ) : (
              <>
                <Card>
                  <CardHeader>
                    <CardTitle>运行状态</CardTitle>
                    <CardDescription>当前会话、运行与待审批数量。</CardDescription>
                  </CardHeader>
                  <CardContent>
                    {snapshot ? (
                      <div className="grid gap-4 text-sm sm:grid-cols-3">
                        <div>
                          <p className="text-muted-foreground">会话</p>
                          <p className="mt-1 text-lg font-semibold tabular-nums">{snapshot.sessionCount}</p>
                        </div>
                        <div>
                          <p className="text-muted-foreground">活跃运行</p>
                          <p className="mt-1 text-lg font-semibold tabular-nums">{snapshot.activeRuns}</p>
                        </div>
                        <div>
                          <p className="text-muted-foreground">待处理 Review</p>
                          <p className="mt-1 text-lg font-semibold tabular-nums">{snapshot.pendingReviews}</p>
                        </div>
                      </div>
                    ) : (
                      <div className="h-16 animate-pulse rounded-md bg-muted" />
                    )}
                  </CardContent>
                </Card>
                <Card>
                  <CardHeader>
                    <CardTitle>近期活动</CardTitle>
                    <CardDescription>最近完成的演示任务。</CardDescription>
                  </CardHeader>
                  <CardContent className="divide-y">
                    {snapshot?.recentActivity.map((item) => (
                      <div key={item.id} className="flex items-start justify-between gap-3 py-4 first:pt-0 last:pb-0">
                        <div className="min-w-0">
                          <p className="font-medium">{item.title}</p>
                          <p className="text-sm text-muted-foreground">{item.detail}</p>
                        </div>
                        <span className="shrink-0 text-xs text-muted-foreground">{item.occurredAt}</span>
                      </div>
                    ))}
                  </CardContent>
                </Card>
              </>
            )}
          </TabsContent>

          <TabsContent value="token" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>Token 统计</CardTitle>
                <CardDescription>当前周期的用量、模型分布、趋势和会话明细。</CardDescription>
              </CardHeader>
              <CardContent>
                <TokenStatsPanel />
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="audit" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>审计日志</CardTitle>
                <CardDescription>按类型查看演示日志。</CardDescription>
              </CardHeader>
              <CardContent className="h-[480px] p-0">
                <AuditPanel />
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}
