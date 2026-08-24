import { createFileRoute } from '@tanstack/react-router';
import { SkillsView } from '@/components/skills/SkillsView';
import { DemoBanner } from '@/components/demo/DemoBanner';
export const Route = createFileRoute('/_layout/skills')({ component: () => <div className="flex h-full flex-col"><DemoBanner/><div className="min-h-0 flex-1"><SkillsView/></div></div> });
