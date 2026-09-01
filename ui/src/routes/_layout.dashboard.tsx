import { createFileRoute } from '@tanstack/react-router';
import { DashboardView } from '@/components/dashboard/DashboardView';
export const Route = createFileRoute('/_layout/dashboard')({ component: () => <div className="flex h-full flex-col"><div className="min-h-0 flex-1"><DashboardView/></div></div> });
