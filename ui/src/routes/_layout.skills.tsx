import { createFileRoute } from '@tanstack/react-router';
import { SkillsView } from '@/components/skills/SkillsView';

export const Route = createFileRoute('/_layout/skills')({
  component: () => <SkillsView />,
});
