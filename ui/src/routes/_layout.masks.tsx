import { createFileRoute } from '@tanstack/react-router';
import { MaskManagementView } from '@/components/masks/MaskManagementView';

export const Route = createFileRoute('/_layout/masks')({ component: MaskManagementView });
