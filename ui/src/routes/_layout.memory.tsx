import { createFileRoute } from '@tanstack/react-router';
import { DemoBanner } from '@/components/demo/DemoBanner';
import { MemoryDemoView } from '@/components/demo/MemoryDemoView';
export const Route = createFileRoute('/_layout/memory')({ component: () => <div className="flex h-full flex-col"><DemoBanner/><div className="min-h-0 flex-1"><MemoryDemoView/></div></div> });
