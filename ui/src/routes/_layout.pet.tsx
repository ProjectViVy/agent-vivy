import { createFileRoute } from '@tanstack/react-router';
import { DemoBanner } from '@/components/demo/DemoBanner';
import { PetDemoView } from '@/components/demo/PetDemoView';
export const Route = createFileRoute('/_layout/pet')({ component: () => <div className="flex h-full flex-col"><DemoBanner/><div className="min-h-0 flex-1"><PetDemoView/></div></div> });
