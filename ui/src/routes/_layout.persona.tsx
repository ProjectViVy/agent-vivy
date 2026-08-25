/**
 * 人格页面 - 复用 _layout 共享壳
 * 管理 AI 助手的 7 份人格 Markdown 文档
 */

import { createFileRoute } from '@tanstack/react-router';
import { PersonaMemoryView } from '@/components/persona-memory/PersonaMemoryView';
import { DemoBanner } from '@/components/demo/DemoBanner';
import { useTranslation } from '@/i18n';

export const Route = createFileRoute('/_layout/persona')({
  component: PersonaPage,
});

function PersonaPage() {
  const { t } = useTranslation();
  return (
    <div className="h-full flex flex-col">
      <DemoBanner />
      {/* 顶部标题 */}
      <div className="border-b px-4 py-4 sm:px-6">
        <h1 className="text-lg font-semibold">{t('persona.title')}</h1>
        <p className="text-sm text-muted-foreground mt-0.5">
          {t('persona.subtitle')}
        </p>
      </div>

      {/* 内容区 */}
      <div className="flex-1 overflow-hidden">
        <PersonaMemoryView />
      </div>
    </div>
  );
}
