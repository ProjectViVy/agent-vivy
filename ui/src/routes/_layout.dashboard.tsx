import { createFileRoute } from '@tanstack/react-router';
import { DashboardDemoView } from '@/components/demo/DashboardDemoView';
import { DemoBanner } from '@/components/demo/DemoBanner';
export const Route = createFileRoute('/_layout/dashboard')({ component: () => <div className="flex h-full flex-col"><DemoBanner/><div className="min-h-0 flex-1"><DashboardDemoView/></div></div> });
