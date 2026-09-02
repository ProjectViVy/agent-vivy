/**
 * 轨迹面板：会话选择 + 工具栏 + 总览时间轴 + 账本（右开详情）的组合根组件。
 * 数据来自 `trajectory/session` RPC 真实投影（UI-TRAJ / UI-TRAJECTORY-DEMO）。
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { listSessions, type Session } from '@/lib/api';
import { useTranslation } from '@/i18n';
import { loadSessionTrajectory, type SessionTrajectory } from './trajectory-session';
import type { TrajectoryRecord, TrajectoryTimeRange, TrajectoryTimelineMode } from './trajectory-types';
import { trajectoryTimelineFocusIndexes } from './trajectory-utils';
import { TrajectoryToolbar } from './TrajectoryToolbar';
import { TrajectoryTimeline } from './TrajectoryTimeline';
import { TrajectoryLedger } from './TrajectoryLedger';
import { TrajectoryDetailPanel } from './TrajectoryDetailPanel';
import { DemoLoadError } from '@/components/demo/DemoBanner';

const EMPTY_TURNS: ReadonlySet<number> = new Set();
const EMPTY_ASSISTANTS: ReadonlySet<string> = new Set();

function matchesQuery(record: TrajectoryRecord, query: string): boolean {
  const needle = query.toLowerCase();
  return record.text.toLowerCase().includes(needle)
    || (record.result ?? '').toLowerCase().includes(needle)
    || record.kind.toLowerCase().includes(needle)
    || (record.model ?? '').toLowerCase().includes(needle);
}

/** 折叠候选：回合内除系统行外不止一条记录的回合。 */
function collapsibleTurnIds(records: readonly TrajectoryRecord[]): readonly number[] {
  const counts = new Map<number, number>();
  for (const record of records) {
    if (record.turn === null || record.kind === 'system') continue;
    counts.set(record.turn, (counts.get(record.turn) ?? 0) + 1);
  }
  return [...counts.entries()]
    .filter(([, count]) => count > 1)
    .map(([turn]) => turn);
}

/** 折叠候选：其后紧跟同组工具/子工具行的 ASSISTANT 记录。 */
function collapsibleAssistantIds(records: readonly TrajectoryRecord[]): readonly string[] {
  const ids: string[] = [];
  for (let index = 0; index < records.length; index += 1) {
    const record = records[index];
    if (record.kind !== 'message') continue;
    const next = records[index + 1];
    if (next !== undefined && next.turn === record.turn && next.group === record.group
      && (next.kind === 'tool' || next.kind === 'subtool')) {
      ids.push(record.id);
    }
  }
  return ids;
}

/** 中控台「轨迹」面板（trajectory/session 真实数据）。 */
export function TrajectoryPanel() {
  const { t } = useTranslation();
  const [sessions, setSessions] = useState<Session[] | null>(null);
  const [sessionsError, setSessionsError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(null);

  const [trajectory, setTrajectory] = useState<SessionTrajectory | null>(null);
  const [trajectoryError, setTrajectoryError] = useState<string | null>(null);
  const [loadingTrajectory, setLoadingTrajectory] = useState(false);

  const [mode, setMode] = useState<TrajectoryTimelineMode>('sequence');
  const [collapsedTurns, setCollapsedTurns] = useState<ReadonlySet<number>>(EMPTY_TURNS);
  const [collapsedAssistants, setCollapsedAssistants] = useState<ReadonlySet<string>>(EMPTY_ASSISTANTS);
  const [searchQuery, setSearchQuery] = useState('');
  const [range, setRange] = useState<TrajectoryTimeRange | null>(null);
  const [selectedRecordId, setSelectedRecordId] = useState<string | null>(null);
  const [selectedRequestNumber, setSelectedRequestNumber] = useState<number | null>(null);

  // Track whether the session list ever loaded so errors after a success
  // keep the old panel visible instead of replacing it.
  const sessionsLoaded = useRef(false);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setSessionsError(null);
      try {
        const view = await listSessions();
        if (!cancelled) {
          setSessions(view.sessions);
          sessionsLoaded.current = true;
        }
      } catch (cause) {
        if (!cancelled) setSessionsError(cause instanceof Error ? cause.message : String(cause));
      }
    };
    void load();
    return () => { cancelled = true; };
  }, [reloadKey]);

  useEffect(() => {
    if (selectedSessionId === null) return;
    let cancelled = false;
    const load = async () => {
      setLoadingTrajectory(true);
      setTrajectoryError(null);
      try {
        const next = await loadSessionTrajectory(selectedSessionId);
        if (!cancelled) setTrajectory(next);
      } catch (cause) {
        if (!cancelled) {
          setTrajectoryError(cause instanceof Error ? cause.message : String(cause));
          setTrajectory(null);
        }
      } finally {
        if (!cancelled) setLoadingTrajectory(false);
      }
    };
    void load();
    return () => { cancelled = true; };
  }, [selectedSessionId, reloadKey]);

  const records = trajectory?.records ?? [];
  const requests = trajectory?.requests ?? [];

  // 会话列表就绪后默认选中第一个会话。
  useEffect(() => {
    if (selectedSessionId === null && sessions !== null && sessions.length > 0) {
      setSelectedSessionId(sessions[0]!.id);
    }
  }, [selectedSessionId, sessions]);

  const selectSession = (sessionId: string) => {
    if (sessionId === selectedSessionId) return;
    setMode('sequence');
    setCollapsedTurns(EMPTY_TURNS);
    setCollapsedAssistants(EMPTY_ASSISTANTS);
    setSearchQuery('');
    setRange(null);
    setSelectedRecordId(null);
    setSelectedRequestNumber(null);
    setTrajectory(null);
    setSelectedSessionId(sessionId);
  };

  const refresh = useCallback(() => setReloadKey((v) => v + 1), []);

  const searchActive = searchQuery.trim() !== '';

  const visibleRecords = useMemo(() => {
    if (!searchActive) return records;
    return records.filter((record) => matchesQuery(record, searchQuery.trim()));
  }, [records, searchActive, searchQuery]);

  const searchMatchIndexes = useMemo(() => {
    if (!searchActive) return null;
    return new Set(visibleRecords.map((record) => record.index));
  }, [searchActive, visibleRecords]);

  const focusIndexes = useMemo(
    () => range === null ? null : trajectoryTimelineFocusIndexes(records, range, mode),
    [records, range, mode],
  );

  const collapsibleTurns = useMemo(() => collapsibleTurnIds(records), [records]);
  const collapsibleAssistants = useMemo(() => collapsibleAssistantIds(records), [records]);

  const allTurnsCollapsed = collapsibleTurns.length > 0
    && collapsibleTurns.every((turn) => collapsedTurns.has(turn));
  const allAssistantsCollapsed = collapsibleAssistants.length > 0
    && collapsibleAssistants.every((id) => collapsedAssistants.has(id));

  const toggleAllTurns = () => {
    setCollapsedTurns((current) => {
      const next = new Set(current);
      for (const turn of collapsibleTurns) {
        if (allTurnsCollapsed) next.delete(turn);
        else next.add(turn);
      }
      return next;
    });
  };

  const toggleAllAssistants = () => {
    setCollapsedAssistants((current) => {
      const next = new Set(current);
      for (const id of collapsibleAssistants) {
        if (allAssistantsCollapsed) next.delete(id);
        else next.add(id);
      }
      return next;
    });
  };

  const toggleTurn = (turn: number) => {
    setCollapsedTurns((current) => {
      const next = new Set(current);
      if (next.has(turn)) next.delete(turn);
      else next.add(turn);
      return next;
    });
  };

  const toggleAssistant = (id: string) => {
    setCollapsedAssistants((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  /** 请求号 → 该请求的起始 ASSISTANT 记录（同 turn+group 内首条 message 行）。 */
  const requestNumberByRecordId = useMemo(() => {
    const map = new Map<string, number>();
    for (const request of requests) {
      const record = records.find((candidate) =>
        candidate.turn === request.turn && candidate.group === request.group && candidate.kind === 'message');
      if (record !== undefined) map.set(record.id, request.number);
    }
    return map;
  }, [records, requests]);

  const resetSelection = () => {
    setSelectedRecordId(null);
    setSelectedRequestNumber(null);
  };

  const handleTimelineRecordSelect = (index: number) => {
    setRange(null);
    const record = records.find((candidate) => candidate.index === index);
    if (record === undefined) return;
    resetSelection();
    setSelectedRecordId(record.id);
  };

  const handleRecordSelect = (id: string) => {
    setSelectedRecordId(id);
    setSelectedRequestNumber(null);
  };

  const handleRequestSelect = (number: number) => {
    setSelectedRequestNumber(number);
    setSelectedRecordId(null);
  };

  const selectedRecord = selectedRecordId === null
    ? null
    : records.find((record) => record.id === selectedRecordId) ?? null;
  const selectedRequest = selectedRequestNumber === null
    ? null
    : requests.find((request) => request.number === selectedRequestNumber) ?? null;

  const requestTools = useMemo(() => {
    if (selectedRequest === null) return [];
    return records.filter((record) =>
      record.turn === selectedRequest.turn && record.group === selectedRequest.group
      && (record.kind === 'tool' || record.kind === 'subtool'));
  }, [records, selectedRequest]);
  const requestAssistant = useMemo(() => {
    if (selectedRequest === null) return null;
    return records.find((record) =>
      record.turn === selectedRequest.turn && record.group === selectedRequest.group
      && record.kind === 'message') ?? null;
  }, [records, selectedRequest]);

  // 会话列表首载失败：整体错误态。
  if (sessionsError && !sessionsLoaded.current) {
    return <DemoLoadError message={sessionsError} onRetry={refresh} />;
  }

  // 会话列表首载中：骨架。
  if (sessions === null) {
    return <div className="h-10 animate-pulse rounded-md bg-muted" />;
  }

  // 无会话：空态提示。
  if (sessions.length === 0) {
    return <p className="p-6 text-sm text-muted-foreground">{t('trajectory.sessionEmpty')}</p>;
  }

  return (
    <div className="flex h-full flex-col" data-trajectory-panel="">
      <div className="flex items-center gap-2 border-b px-3 py-2">
        <span className="text-xs text-muted-foreground">{t('trajectory.pickSession')}</span>
        <Select value={selectedSessionId ?? undefined} onValueChange={selectSession}>
          <SelectTrigger className="h-8 w-64" data-trajectory-session-select="">
            <SelectValue placeholder={t('trajectory.pickSession')} />
          </SelectTrigger>
          <SelectContent>
            {sessions.map((session) => (
              <SelectItem key={session.id} value={session.id}>{session.title}</SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button variant="ghost" size="sm" className="h-8" onClick={refresh}>
          {t('trajectory.refresh')}
        </Button>
        {loadingTrajectory && <span className="h-4 w-4 animate-spin rounded-full border-2 border-muted-foreground border-t-transparent" aria-hidden />}
      </div>
      {trajectoryError ? (
        <DemoLoadError message={trajectoryError} onRetry={refresh} />
      ) : loadingTrajectory && trajectory === null ? (
        <div className="space-y-2 p-3">
          <div className="h-8 animate-pulse rounded-md bg-muted" />
          <div className="h-24 animate-pulse rounded-md bg-muted" />
        </div>
      ) : (
        <>
          <TrajectoryToolbar
            mode={mode}
            onModeChange={(next) => {
              setMode(next);
              setRange(null);
            }}
            allTurnsCollapsed={allTurnsCollapsed}
            collapsibleTurnCount={collapsibleTurns.length}
            onToggleAllTurns={toggleAllTurns}
            allAssistantsCollapsed={allAssistantsCollapsed}
            collapsibleAssistantCount={collapsibleAssistants.length}
            onToggleAllAssistants={toggleAllAssistants}
            searchQuery={searchQuery}
            onSearchQueryChange={setSearchQuery}
          />
          <TrajectoryTimeline
            records={records}
            mode={mode}
            range={range}
            focusIndexes={focusIndexes}
            selectedIndex={selectedRecord === null
              ? null
              : selectedRecord.index}
            searchMatchIndexes={searchMatchIndexes}
            onRangeChange={setRange}
            onRecordSelect={handleTimelineRecordSelect}
          />
          <div className="flex min-h-0 flex-1">
            <div className="min-w-0 flex-1 overflow-auto" data-trajectory-scroll="">
              <TrajectoryLedger
                records={visibleRecords}
                collapsedTurns={searchActive ? EMPTY_TURNS : collapsedTurns}
                collapsedAssistants={searchActive ? EMPTY_ASSISTANTS : collapsedAssistants}
                focusIndexes={focusIndexes}
                selectedRecordId={selectedRecordId}
                requestNumberByRecordId={requestNumberByRecordId}
                onToggleTurn={toggleTurn}
                onToggleAssistant={toggleAssistant}
                onSelectRecord={handleRecordSelect}
                onSelectRequest={handleRequestSelect}
              />
            </div>
            {(selectedRecord !== null || selectedRequest !== null) && (
              <TrajectoryDetailPanel
                request={selectedRequest}
                record={selectedRecord}
                requestTools={requestTools}
                requestAssistant={requestAssistant}
                onClose={resetSelection}
              />
            )}
          </div>
        </>
      )}
    </div>
  );
}
