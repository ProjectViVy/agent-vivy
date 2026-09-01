import { useCallback, useEffect, useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { TrajectoryPanel } from '@/components/trajectory/TrajectoryPanel';
import { listBackgroundRuns, listReviews, listSessions } from '@/lib/api';
import { useTranslation } from '@/i18n';
import { DemoLoadError } from '@/components/demo/DemoBanner';
import { TokenStatsPanel } from '@/components/demo/TokenStatsPanel';

const TERMINAL_RUN_STATUSES: ReadonlySet<string> = new Set(['completed', 'failed', 'cancelled']);

interface OverviewCounts {
  sessions: number;
  activeRuns: number;
  pendingReviews: number;
}

export function DashboardView() {
  const { t } = useTranslation();
  const [counts, setCounts] = useState<OverviewCounts | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const load = useCallback(async () => {
    setError(null);
    try {
      const [sessions, runs, reviews] = await Promise.all([
        listSessions(),
        listBackgroundRuns(),
        listReviews({ status: 'pending' }),
      ]);
      setCounts({
        sessions: sessions.sessions.length,
        activeRuns: runs.runs.filter((run) => !TERMINAL_RUN_STATUSES.has(run.status)).length,
        pendingReviews: reviews.reviews.length,
      });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  }, []);
  useEffect(() => { void load(); }, [load, reloadKey]);

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
              <DemoLoadError message={error} onRetry={() => setReloadKey((v) => v + 1)} />
            ) : (
              <Card>
                <CardHeader>
                  <CardTitle>{t('dashboard.statusTitle')}</CardTitle>
                  <CardDescription>{t('dashboard.statusDesc')}</CardDescription>
                </CardHeader>
                <CardContent>
                  {counts ? (
                    <div className="grid gap-4 text-sm sm:grid-cols-3">
                      <div>
                        <p className="text-muted-foreground">{t('dashboard.sessions')}</p>
                        <p className="mt-1 text-lg font-semibold tabular-nums">{counts.sessions}</p>
                      </div>
                      <div>
                        <p className="text-muted-foreground">{t('dashboard.activeRuns')}</p>
                        <p className="mt-1 text-lg font-semibold tabular-nums">{counts.activeRuns}</p>
                      </div>
                      <div>
                        <p className="text-muted-foreground">{t('dashboard.pendingReviews')}</p>
                        <p className="mt-1 text-lg font-semibold tabular-nums">{counts.pendingReviews}</p>
                      </div>
                    </div>
                  ) : (
                    <div className="h-16 animate-pulse rounded-md bg-muted" />
                  )}
                </CardContent>
              </Card>
            )}
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}
