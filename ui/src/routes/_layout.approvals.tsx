import { createFileRoute } from '@tanstack/react-router';
import { ApprovalsView } from '@/components/approvals/ApprovalsView';
export const Route = createFileRoute('/_layout/approvals')({ component: ApprovalsView });
