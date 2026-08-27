/**
 * 轨迹面板：工具栏 + 总览时间轴 + 账本（右开详情）的组合根组件。
 * 复刻 DeepSeek Harness `TrajectoryView` 的布局与交互（纯演示数据，无后端）。
 */

import { useMemo, useState } from 'react';
import {
  DEMO_TRAJECTORY_RECORDS,
  DEMO_TRAJECTORY_REQUESTS,
  recordForRequest,
  requestByNumber,
  type TrajectoryRecord,
  type TrajectoryTimeRange,
  type TrajectoryTimelineMode,
} from './trajectory-demo-data';
import { trajectoryTimelineFocusIndexes } from './trajectory-utils';
import { TrajectoryToolbar } from './TrajectoryToolbar';
import { TrajectoryTimeline } from './TrajectoryTimeline';
import { TrajectoryLedger } from './TrajectoryLedger';
import { TrajectoryDetailPanel } from './TrajectoryDetailPanel';

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

/** 中控台「轨迹」面板（演示数据，无后端）。 */
export function TrajectoryPanel() {
  const [mode, setMode] = useState<TrajectoryTimelineMode>('sequence');
  const [collapsedTurns, setCollapsedTurns] = useState<ReadonlySet<number>>(EMPTY_TURNS);
  const [collapsedAssistants, setCollapsedAssistants] = useState<ReadonlySet<string>>(EMPTY_ASSISTANTS);
  const [searchQuery, setSearchQuery] = useState('');
  const [range, setRange] = useState<TrajectoryTimeRange | null>(null);
  const [selectedRecordId, setSelectedRecordId] = useState<string | null>(null);
  const [selectedRequestNumber, setSelectedRequestNumber] = useState<number | null>(null);

  const searchActive = searchQuery.trim() !== '';

  const visibleRecords = useMemo(() => {
    if (!searchActive) return DEMO_TRAJECTORY_RECORDS;
    return DEMO_TRAJECTORY_RECORDS.filter((record) => matchesQuery(record, searchQuery.trim()));
  }, [searchActive, searchQuery]);

  const searchMatchIndexes = useMemo(() => {
    if (!searchActive) return null;
    return new Set(visibleRecords.map((record) => record.index));
  }, [searchActive, visibleRecords]);

  const focusIndexes = useMemo(
    () => range === null ? null : trajectoryTimelineFocusIndexes(DEMO_TRAJECTORY_RECORDS, range, mode),
    [range, mode],
  );

  const collapsibleTurns = useMemo(() => collapsibleTurnIds(DEMO_TRAJECTORY_RECORDS), []);
  const collapsibleAssistants = useMemo(() => collapsibleAssistantIds(DEMO_TRAJECTORY_RECORDS), []);

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

  const requestNumberByRecordId = useMemo(() => {
    const map = new Map<string, number>();
    for (const request of DEMO_TRAJECTORY_REQUESTS) {
      const record = recordForRequest(request.number);
      if (record !== undefined) map.set(record.id, request.number);
    }
    return map;
  }, []);

  const handleTimelineRecordSelect = (index: number) => {
    setRange(null);
    const record = DEMO_TRAJECTORY_RECORDS.find((candidate) => candidate.index === index);
    if (record === undefined) return;
    setSelectedRequestNumber(null);
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
    : DEMO_TRAJECTORY_RECORDS.find((record) => record.id === selectedRecordId) ?? null;
  const selectedRequest = selectedRequestNumber === null
    ? null
    : requestByNumber(selectedRequestNumber) ?? null;

  const requestTools = useMemo(() => {
    if (selectedRequest === null) return [];
    return DEMO_TRAJECTORY_RECORDS.filter((record) =>
      record.turn === selectedRequest.turn && record.group === selectedRequest.group
      && (record.kind === 'tool' || record.kind === 'subtool'));
  }, [selectedRequest]);
  const requestAssistant = useMemo(() => {
    if (selectedRequest === null) return null;
    return DEMO_TRAJECTORY_RECORDS.find((record) =>
      record.turn === selectedRequest.turn && record.group === selectedRequest.group
      && record.kind === 'message') ?? null;
  }, [selectedRequest]);

  return (
    <div className="flex h-full flex-col" data-trajectory-panel="">
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
        records={DEMO_TRAJECTORY_RECORDS}
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
            onClose={() => {
              setSelectedRecordId(null);
              setSelectedRequestNumber(null);
            }}
          />
        )}
      </div>
    </div>
  );
}