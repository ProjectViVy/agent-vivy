import { useEffect, useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { TrajectoryPanel } from '@/components/trajectory/TrajectoryPanel';
import { getDemoDashboard } from '@/lib/demo-api';
import type { DemoDashboardSnapshot } from '@/lib/types';
import { useTranslation } from '@/i18n';
import { DemoLoadError } from './DemoBanner';
import { TokenStatsPanel } from './TokenStatsPanel';

export function DashboardDemoView() {
  const { t } = useTranslation();
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
        <h1 className="text-2xl font-bold">{t('dashboard.title')}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t('dashboard.subtitle')}</p>
        <Tabs defaultValue="token" className="mt-6">
          <TabsList className="w-full justify-start overflow-x-auto">
            <TabsTrigger value="token">{t('dashboard.token')}</TabsTrigger>
            <TabsTrigger value="trajectory">{t('dashboard.trajectory')}</TabsTrigger>
            <TabsTrigger value="overview">{t('dashboard.overview')}</TabsTrigger>
          </TabsList>

          <TabsContent value="token" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>{t('dashboard.tokenTitle')}</CardTitle>
                <CardDescription>{t('dashboard.tokenDesc')}</CardDescription>
              </CardHeader>
              <CardContent>
                <TokenStatsPanel />
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="trajectory" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>{t('dashboard.trajectoryTitle')}</CardTitle>
                <CardDescription>{t('dashboard.trajectoryDesc')}</CardDescription>
              </CardHeader>
              <CardContent className="h-[520px] p-0">
                <TrajectoryPanel />
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="overview" className="space-y-4">
            {error ? (
              <DemoLoadError message={error} onRetry={() => void load()} />
            ) : (
              <>
                <Card>
                  <CardHeader>
                    <CardTitle>{t('dashboard.statusTitle')}</CardTitle>
                    <CardDescription>{t('dashboard.statusDesc')}</CardDescription>
                  </CardHeader>
                  <CardContent>
                    {snapshot ? (
                      <div className="grid gap-4 text-sm sm:grid-cols-3">
                        <div>
                          <p className="text-muted-foreground">{t('dashboard.sessions')}</p>
                          <p className="mt-1 text-lg font-semibold tabular-nums">{snapshot.sessionCount}</p>
                        </div>
                        <div>
                          <p className="text-muted-foreground">{t('dashboard.activeRuns')}</p>
                          <p className="mt-1 text-lg font-semibold tabular-nums">{snapshot.activeRuns}</p>
                        </div>
                        <div>
                          <p className="text-muted-foreground">{t('dashboard.pendingReviews')}</p>
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
                    <CardTitle>{t('dashboard.activityTitle')}</CardTitle>
                    <CardDescription>{t('dashboard.activityDesc')}</CardDescription>
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
        </Tabs>
      </div>
    </div>
  );
}
