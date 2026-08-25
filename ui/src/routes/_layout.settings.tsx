import { createFileRoute } from '@tanstack/react-router';
import { SettingsView, isSettingsTab, type SettingsTab } from '@/components/settings/SettingsView';

export const Route = createFileRoute('/_layout/settings')({
  // ?tab=model 之类的深链用于欢迎向导完成后的定向落地；非法值静默丢弃。
  validateSearch: (search: Record<string, unknown>): { tab?: SettingsTab } =>
    isSettingsTab(search.tab) ? { tab: search.tab } : {},
  component: () => <SettingsView initialTab={Route.useSearch().tab} />,
});
