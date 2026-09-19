/**
 * 进化页面 - 由 vivy/evolution Module 装配
 */

import { DemoBanner } from '@/components/demo/DemoBanner';
import { usePluginTranslation } from '@vivy/ui-sdk';
import { EvolutionView } from './view';

export function EvolutionPage() {
  const { t } = usePluginTranslation();
  return (
    <div className="h-full flex flex-col">
      <DemoBanner />
      {/* 顶部标题 */}
      <div className="border-b px-4 py-4 sm:px-6">
        <h1 className="text-lg font-semibold">{t('plugin.vivy/evolution.title')}</h1>
        <p className="text-sm text-muted-foreground mt-0.5">{t('plugin.vivy/evolution.subtitle')}</p>
      </div>

      {/* 内容区 */}
      <div className="min-h-0 flex-1">
        <EvolutionView />
      </div>
    </div>
  );
}
