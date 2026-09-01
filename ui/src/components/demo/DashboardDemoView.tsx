import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { TrajectoryPanel } from '@/components/trajectory/TrajectoryPanel';
import { useTranslation } from '@/i18n';
import { TokenStatsPanel } from './TokenStatsPanel';

export function DashboardDemoView() {
  const { t } = useTranslation();

  return (
    <div className="h-full overflow-auto p-4 sm:p-6">
      <div className="mx-auto max-w-4xl">
        <h1 className="text-2xl font-bold">{t('dashboard.title')}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t('dashboard.subtitle')}</p>
        <Tabs defaultValue="token" className="mt-6">
          <TabsList className="w-full justify-start overflow-x-auto">
            <TabsTrigger value="token">{t('dashboard.token')}</TabsTrigger>
            <TabsTrigger value="trajectory">{t('dashboard.trajectory')}</TabsTrigger>
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
        </Tabs>
      </div>
    </div>
  );
}
