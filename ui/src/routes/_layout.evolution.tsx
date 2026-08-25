/**
 * 进化页面 - 复用 _layout 共享壳
 */
import { createFileRoute } from '@tanstack/react-router';
import { EvolutionView } from '@/components/evolution/EvolutionView';
import { DemoBanner } from '@/components/demo/DemoBanner';
import { useTranslation } from '@/i18n';

export const Route = createFileRoute('/_layout/evolution')({
  component: EvolutionPage,
});

function EvolutionPage() {
  const { t } = useTranslation();
  return (
    <div className="h-full flex flex-col">
      <DemoBanner />
      {/* 顶部标题 */}
      <div className="border-b px-4 py-4 sm:px-6">
        <h1 className="text-lg font-semibold">{t('evolution.title')}</h1>
        <p className="text-sm text-muted-foreground mt-0.5">{t('evolution.subtitle')}</p>
      </div>
      {/* 内容区 */}
      <div className="flex-1 overflow-hidden">
        <EvolutionView />
      </div>
    </div>
  );
}
