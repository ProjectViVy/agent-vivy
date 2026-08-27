/**
 * 轨迹面板工具栏：复刻 DeepSeek Harness `TrajectoryToolbar` 的可见结构
 * （Duration 切换 / 全部回合折叠 / 全部调用折叠 / 轨迹搜索）。
 */

import { Clock3, Search, UnfoldVertical, FoldVertical } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useTranslation } from '@/i18n';
import type { TrajectoryTimelineMode } from './trajectory-demo-data';

export interface TrajectoryToolbarProps {
  mode: TrajectoryTimelineMode;
  onModeChange: (mode: TrajectoryTimelineMode) => void;
  allTurnsCollapsed: boolean;
  collapsibleTurnCount: number;
  onToggleAllTurns: () => void;
  allAssistantsCollapsed: boolean;
  collapsibleAssistantCount: number;
  onToggleAllAssistants: () => void;
  searchQuery: string;
  onSearchQueryChange: (query: string) => void;
}

/** 渲染粘性轨迹工具栏。 */
export function TrajectoryToolbar({
  mode,
  onModeChange,
  allTurnsCollapsed,
  collapsibleTurnCount,
  onToggleAllTurns,
  allAssistantsCollapsed,
  collapsibleAssistantCount,
  onToggleAllAssistants,
  searchQuery,
  onSearchQueryChange,
}: TrajectoryToolbarProps) {
  const { t } = useTranslation();
  const useActualDuration = mode === 'duration';

  return (
    <div
      className="flex items-center justify-between gap-3 border-b px-4 py-2"
      role="toolbar"
      aria-label={t('trajectory.toolbarAria')}
    >
      <div className="flex flex-wrap items-center gap-1.5">
        <Button
          type="button"
          size="sm"
          variant="ghost"
          aria-pressed={useActualDuration}
          title={useActualDuration ? t('trajectory.useEqualWidth') : t('trajectory.useActualDuration')}
          onClick={() => onModeChange(useActualDuration ? 'sequence' : 'duration')}
          className="gap-1.5 px-2 text-xs"
        >
          <Clock3 className="h-3.5 w-3.5" aria-hidden="true" />
          {t('trajectory.duration')}
        </Button>
        <span className="mx-1 h-4 w-px bg-border" aria-hidden="true" />
        <Button
          type="button"
          size="sm"
          variant="ghost"
          aria-pressed={allTurnsCollapsed}
          disabled={collapsibleTurnCount === 0}
          title={allTurnsCollapsed ? t('trajectory.expandTurns') : t('trajectory.collapseTurns')}
          onClick={onToggleAllTurns}
          className="gap-1.5 px-2 text-xs"
        >
          {allTurnsCollapsed
            ? <UnfoldVertical className="h-3.5 w-3.5" aria-hidden="true" />
            : <FoldVertical className="h-3.5 w-3.5" aria-hidden="true" />}
          {t('trajectory.turns')}
        </Button>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          aria-pressed={allAssistantsCollapsed}
          disabled={collapsibleAssistantCount === 0}
          title={allAssistantsCollapsed ? t('trajectory.expandCalls') : t('trajectory.collapseCalls')}
          onClick={onToggleAllAssistants}
          className="gap-1.5 px-2 text-xs"
        >
          {allAssistantsCollapsed
            ? <UnfoldVertical className="h-3.5 w-3.5" aria-hidden="true" />
            : <FoldVertical className="h-3.5 w-3.5" aria-hidden="true" />}
          {t('trajectory.calls')}
        </Button>
      </div>
      <div className="relative">
        <Search
          className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground"
          aria-hidden="true"
        />
        <Input
          type="search"
          value={searchQuery}
          onChange={(event) => onSearchQueryChange(event.target.value)}
          aria-label={t('trajectory.searchAria')}
          placeholder={t('trajectory.searchPlaceholder')}
          className="h-8 w-52 pl-8 text-xs"
        />
      </div>
    </div>
  );
}