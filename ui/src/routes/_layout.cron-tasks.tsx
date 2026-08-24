import { createFileRoute } from '@tanstack/react-router';
import { CronTaskManagementView } from '@/components/cron/CronTaskManagementView';
import { DemoBanner } from '@/components/demo/DemoBanner';
export const Route = createFileRoute('/_layout/cron-tasks')({ component: () => <div className="flex h-full flex-col"><DemoBanner/><div className="min-h-0 flex-1"><CronTaskManagementView/></div></div> });
