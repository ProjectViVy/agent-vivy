/**
 * 记事本页面 - 由 vivy/notebook Module 装配
 */

import { DemoBanner } from '@/components/demo/DemoBanner';
import { NotebookView } from './view';

export function NotebookPage() {
  return (
    <div className="h-full flex flex-col">
      <DemoBanner />
      {/* 内容区 */}
      <div className="min-h-0 flex-1">
        <NotebookView />
      </div>
    </div>
  );
}
