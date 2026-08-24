import { createFileRoute } from '@tanstack/react-router';
import { LifecycleView } from '@/components/lifecycle/LifecycleView';
export const Route = createFileRoute('/_layout/lifecycle')({ component: LifecycleView });
