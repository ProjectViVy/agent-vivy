import { createFileRoute } from '@tanstack/react-router';
import { DemoBanner } from '@/components/demo/DemoBanner';
import { NotebookView } from '@/components/notebook/NotebookView';
export const Route = createFileRoute('/_layout/notebook')({ component: () => <div className="flex h-full flex-col"><DemoBanner/><div className="min-h-0 flex-1"><NotebookView/></div></div> });
