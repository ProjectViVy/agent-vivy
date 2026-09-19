/**
 * 人格页面 - 由 vivy/persona Module 装配
 * 管理 AI 助手的 7 份人格 Markdown 文档
 */

import { DemoBanner } from '@/components/demo/DemoBanner';
import { usePluginTranslation } from '@vivy/ui-sdk';
import { PersonaMemoryView } from './view';

export function PersonaPage() {
  const { t } = usePluginTranslation();
  return (
    <div className="h-full flex flex-col">
      <DemoBanner />
      {/* 顶部标题 */}
      <div className="border-b px-4 py-4 sm:px-6">
        <h1 className="text-lg font-semibold">{t('plugin.vivy/persona.title')}</h1>
        <p className="text-sm text-muted-foreground mt-0.5">
          {t('plugin.vivy/persona.subtitle')}
        </p>
      </div>

      {/* 内容区 */}
      <div className="flex-1 overflow-hidden">
        <PersonaMemoryView />
      </div>
    </div>
  );
}