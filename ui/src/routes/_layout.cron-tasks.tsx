import { createFileRoute } from '@tanstack/react-router';
import { CronTaskManagementView } from '@/components/cron/CronTaskManagementView';
export const Route = createFileRoute('/_layout/cron-tasks')({ component: () => <div className="min-h-0 flex-1"><CronTaskManagementView/></div> });
