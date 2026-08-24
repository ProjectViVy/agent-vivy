/**
 * 人格页面 - 复用 _layout 共享壳
 * 管理 AI 助手的 7 份人格 Markdown 文档
 */

import { createFileRoute } from '@tanstack/react-router';
import { PersonaMemoryView } from '@/components/persona-memory/PersonaMemoryView';
import { DemoBanner } from '@/components/demo/DemoBanner';

export const Route = createFileRoute('/_layout/persona')({
  component: PersonaPage,
});

function PersonaPage() {
  return (
    <div className="h-full flex flex-col">
      <DemoBanner />
      {/* 顶部标题 */}
      <div className="border-b px-4 py-4 sm:px-6">
        <h1 className="text-lg font-semibold">人格</h1>
        <p className="text-sm text-muted-foreground mt-0.5">
          七份人格 Markdown 文档 · 当前 / 待审 / 历史
        </p>
      </div>

      {/* 内容区 */}
      <div className="flex-1 overflow-hidden">
        <PersonaMemoryView />
      </div>
    </div>
  );
}
