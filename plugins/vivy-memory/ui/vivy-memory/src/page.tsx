/**
 * 记忆页面 - 由 vivy/memory Module 装配
 */

import { DemoBanner } from '@/components/demo/DemoBanner';
import { MemoryDemoView } from './view';

export function MemoryPage() {
  return (
    <div className="h-full flex flex-col">
      <DemoBanner />
      {/* 内容区 */}
      <div className="min-h-0 flex-1">
        <MemoryDemoView />
      </div>
    </div>
  );
}
