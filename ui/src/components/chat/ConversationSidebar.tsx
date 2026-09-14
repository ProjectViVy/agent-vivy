import { useEffect, useMemo, useRef, useState } from 'react';
import { Link, useRouterState } from '@tanstack/react-router';
import type { LucideIcon } from 'lucide-react';
import {
  Brain,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Clock,
  Dna,
  Folder,
  FolderOpen,
  FolderPlus,
  LayoutDashboard,
  MoreHorizontal,
  NotebookPen,
  Pencil,
  Pin,
  PinOff,
  Plug,
  Plus,
  Settings,
  ShieldCheck,
  Sparkles,
  Trash2,
  UserRound,
  VenetianMask,
  Wrench,
  X,
  Zap,
} from 'lucide-react';
import type { Session } from '@/lib/api';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/i18n';

type SidebarView = 'root' | 'toolbox' | 'vivy';

type NavItem = {
  to: string;
  icon: LucideIcon;
  labelKey: string;
  exact?: boolean;
};

const DASHBOARD_ITEM: NavItem = { to: '/dashboard', icon: LayoutDashboard, labelKey: 'nav.dashboard' };

const TOOLBOX_ITEMS: NavItem[] = [
  { to: '/cron-tasks', icon: Clock, labelKey: 'nav.cron' },
  { to: '/mcp', icon: Plug, labelKey: 'nav.mcp' },
  { to: '/skills', icon: Zap, labelKey: 'nav.skill' },
  { to: '/approvals', icon: ShieldCheck, labelKey: 'nav.approvals' },
];

const VIVY_ITEMS: NavItem[] = [
  { to: '/persona', icon: UserRound, labelKey: 'nav.persona' },
  { to: '/masks', icon: VenetianMask, labelKey: 'nav.masks' },
  { to: '/evolution', icon: Dna, labelKey: 'nav.evolution' },
  { to: '/memory', icon: Brain, labelKey: 'nav.memory' },
  { to: '/notebook', icon: NotebookPen, labelKey: 'nav.notebook' },
];

const EMPTY_FOLDERS_KEY = 'vivy.demo.emptyFolders';
const FOLDERS_KEY = 'vivy.demo.sessionFolders';
const FOLDER_OPEN_KEY = 'vivy.demo.sessionFoldersOpen';
const FOLDER_PINS_KEY = 'vivy.demo.sessionFolderPins';
const ORDER_KEY = 'vivy.demo.sessionOrder';
const DRAG_MIME = 'application/x-vivy-session';

function readJsonRecord(key: string): Record<string, string> {
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return {};
    const parsed: unknown = JSON.parse(raw);
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {};
    const out: Record<string, string> = {};
    for (const [k, v] of Object.entries(parsed as Record<string, unknown>)) {
      if (typeof v === 'string') out[k] = v;
    }
    return out;
  } catch {
    return {};
  }
}

function readOpenRecord(key: string): Record<string, boolean> {
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return {};
    const parsed: unknown = JSON.parse(raw);
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {};
    const out: Record<string, boolean> = {};
    for (const [k, v] of Object.entries(parsed as Record<string, unknown>)) {
      if (typeof v === 'boolean') out[k] = v;
    }
    return out;
  } catch {
    return {};
  }
}

function readStringArray(key: string): string[] {
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((item): item is string => typeof item === 'string');
  } catch {
    return [];
  }
}

function writeJson(key: string, value: unknown) {
  localStorage.setItem(key, JSON.stringify(value));
}

function viewForPath(pathname: string): SidebarView {
  if (pathname === '/' || pathname.startsWith('/sessions')) return 'root';
  if (TOOLBOX_ITEMS.some((item) => pathname === item.to || pathname.startsWith(`${item.to}/`))) return 'toolbox';
  if (VIVY_ITEMS.some((item) => pathname === item.to || pathname.startsWith(`${item.to}/`))) return 'vivy';
  return 'root';
}

function sortByIdOrder(sessions: Session[], order: string[]): Session[] {
  const index = new Map(order.map((id, i) => [id, i]));
  return [...sessions].sort((a, b) => {
    const ia = index.get(a.id);
    const ib = index.get(b.id);
    if (ia != null && ib != null) return ia - ib;
    if (ia != null) return -1;
    if (ib != null) return 1;
    return (b.created_at ?? 0) - (a.created_at ?? 0);
  });
}

type DropTarget =
  | { kind: 'folder'; name: string }
  | { kind: 'session'; id: string; before: boolean };

interface Props {
  sessions: Session[];
  activeSessionId: string | null;
  busyId: string | null;
  onSelectSession: (id: string) => void;
  onRenameSession: (id: string, title: string) => Promise<void>;
  onDeleteSession: (id: string) => Promise<void>;
  onCreateSession: () => Promise<Session | null> | Session | null | void;
}

export function ConversationSidebar({
  sessions,
  activeSessionId,
  busyId,
  onSelectSession,
  onRenameSession,
  onDeleteSession,
  onCreateSession,
}: Props) {
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const { t } = useTranslation();
  const pathView = viewForPath(pathname);
  const [manualView, setManualView] = useState<SidebarView | null>(null);
  const view = manualView ?? pathView;
  const [folderMap, setFolderMap] = useState<Record<string, string>>(() => readJsonRecord(FOLDERS_KEY));
  const [folderOpen, setFolderOpen] = useState<Record<string, boolean>>(() => readOpenRecord(FOLDER_OPEN_KEY));
  const [sessionOrder, setSessionOrder] = useState<string[]>(() => readStringArray(ORDER_KEY));
  const [emptyFolderList, setEmptyFolderList] = useState<string[]>(() => readStringArray(EMPTY_FOLDERS_KEY));
  const [pinnedFolders, setPinnedFolders] = useState<string[]>(() => readStringArray(FOLDER_PINS_KEY));
  const [editingSessionId, setEditingSessionId] = useState<string | null>(null);
  const [editingTitle, setEditingTitle] = useState('');
  const [editingFolder, setEditingFolder] = useState<string | null>(null);
  const [editingFolderName, setEditingFolderName] = useState('');
  const [removeFolderTarget, setRemoveFolderTarget] = useState<string | null>(null);
  const [dragId, setDragId] = useState<string | null>(null);
  const [dropTarget, setDropTarget] = useState<DropTarget | null>(null);
  const rowRefs = useRef(new Map<string, HTMLDivElement>());

  useEffect(() => {
    setManualView(null);
  }, [pathView]);

  const labelOf = (session: Session) => session.title || t('errors.newSessionDefault');
  const uncategorized = t('sidebar.uncategorized');

  const persistFolders = (next: Record<string, string>) => {
    setFolderMap(next);
    writeJson(FOLDERS_KEY, next);
  };

  const persistOpen = (next: Record<string, boolean>) => {
    setFolderOpen(next);
    writeJson(FOLDER_OPEN_KEY, next);
  };

  const persistOrder = (ids: string[]) => {
    setSessionOrder(ids);
    writeJson(ORDER_KEY, ids);
  };

  const toggleFolder = (name: string) => {
    persistOpen({ ...folderOpen, [name]: !(folderOpen[name] ?? true) });
  };

  const isFolderOpen = (name: string) => folderOpen[name] ?? true;

  const orderedSessions = useMemo(() => sortByIdOrder(sessions, sessionOrder), [sessions, sessionOrder]);

  const uncategorizedSessions = useMemo(
    () => orderedSessions.filter((session) => !folderMap[session.id]),
    [orderedSessions, folderMap],
  );

  const saveRename = async () => {
    if (!editingSessionId || !editingTitle.trim()) return;
    await onRenameSession(editingSessionId, editingTitle.trim());
    setEditingSessionId(null);
  };

  const startRename = (session: Session) => {
    setEditingSessionId(session.id);
    setEditingTitle(session.title);
  };

  const createFolder = () => {
    const base = t('sidebar.newFolder');
    let name = base;
    let n = 2;
    const existing = new Set([...Object.values(folderMap), ...emptyFolderList]);
    while (existing.has(name)) {
      name = `${base} ${n}`;
      n += 1;
    }
    const nextEmpty = [...emptyFolderList, name];
    setEmptyFolderList(nextEmpty);
    writeJson(EMPTY_FOLDERS_KEY, nextEmpty);
    persistOpen({ ...folderOpen, [name]: true });
  };

  const persistPins = (next: string[]) => {
    setPinnedFolders(next);
    writeJson(FOLDER_PINS_KEY, next);
  };

  const togglePinFolder = (name: string) => {
    const next = pinnedFolders.includes(name)
      ? pinnedFolders.filter((item) => item !== name)
      : [...pinnedFolders, name];
    persistPins(next);
  };

  const startRenameFolder = (name: string) => {
    setEditingFolder(name);
    setEditingFolderName(name);
  };

  const saveRenameFolder = () => {
    if (!editingFolder) return;
    const nextName = editingFolderName.trim();
    if (!nextName || nextName === uncategorized || nextName === editingFolder) {
      setEditingFolder(null);
      return;
    }
    const taken = new Set([...Object.values(folderMap), ...emptyFolderList]);
    taken.delete(editingFolder);
    if (taken.has(nextName)) {
      setEditingFolder(null);
      return;
    }
    const nextMap = { ...folderMap };
    for (const [id, folder] of Object.entries(nextMap)) {
      if (folder === editingFolder) nextMap[id] = nextName;
    }
    const nextEmpty = emptyFolderList.includes(editingFolder)
      ? emptyFolderList.map((item) => (item === editingFolder ? nextName : item))
      : emptyFolderList;
    const nextOpen = { ...folderOpen };
    if (editingFolder in nextOpen) {
      nextOpen[nextName] = nextOpen[editingFolder];
      delete nextOpen[editingFolder];
    }
    const nextPins = pinnedFolders.map((item) => (item === editingFolder ? nextName : item));
    persistFolders(nextMap);
    setEmptyFolderList(nextEmpty);
    writeJson(EMPTY_FOLDERS_KEY, nextEmpty);
    persistOpen(nextOpen);
    persistPins(nextPins);
    setEditingFolder(null);
  };

  const confirmRemoveFolder = () => {
    const name = removeFolderTarget;
    if (!name) return;
    const nextMap = { ...folderMap };
    for (const [id, folder] of Object.entries(nextMap)) {
      if (folder === name) delete nextMap[id];
    }
    const nextEmpty = emptyFolderList.filter((item) => item !== name);
    const nextOpen = { ...folderOpen };
    delete nextOpen[name];
    persistFolders(nextMap);
    setEmptyFolderList(nextEmpty);
    writeJson(EMPTY_FOLDERS_KEY, nextEmpty);
    persistOpen(nextOpen);
    persistPins(pinnedFolders.filter((item) => item !== name));
    setRemoveFolderTarget(null);
  };

  const createSessionInFolder = async (folder?: string) => {
    const created = await onCreateSession();
    if (folder && created && typeof created === 'object' && 'id' in created) {
      persistFolders({ ...folderMap, [created.id]: folder });
    }
  };

  const namedFolderEntries = useMemo(() => {
    const fromMap = new Map<string, Session[]>();
    for (const session of orderedSessions) {
      const folder = folderMap[session.id];
      if (!folder) continue;
      const list = fromMap.get(folder) ?? [];
      list.push(session);
      fromMap.set(folder, list);
    }
    const allNames = new Set([...fromMap.keys(), ...emptyFolderList, ...Object.values(folderMap)]);
    return [...allNames]
      .map((name) => [name, fromMap.get(name) ?? []] as [string, Session[]])
      .sort((a, b) => {
        const pa = pinnedFolders.includes(a[0]) ? 0 : 1;
        const pb = pinnedFolders.includes(b[0]) ? 0 : 1;
        if (pa !== pb) return pa - pb;
        return a[0].localeCompare(b[0], 'zh-CN');
      });
  }, [orderedSessions, folderMap, emptyFolderList, pinnedFolders]);

  const visualIds = useMemo(() => {
    const ids: string[] = [];
    for (const [, items] of namedFolderEntries) ids.push(...items.map((s) => s.id));
    ids.push(...uncategorizedSessions.map((s) => s.id));
    return ids;
  }, [namedFolderEntries, uncategorizedSessions]);

  const applyDrop = (dragSessionId: string, target: DropTarget) => {
    if (target.kind === 'session' && dragSessionId === target.id) return;
    const nextFolders = { ...folderMap };
    const rest = visualIds.filter((id) => id !== dragSessionId);

    if (target.kind === 'folder') {
      if (target.name === uncategorized) delete nextFolders[dragSessionId];
      else nextFolders[dragSessionId] = target.name;
      const lastInFolder = rest.filter((id) => {
        const folder = nextFolders[id];
        return target.name === uncategorized ? !folder : folder === target.name;
      }).pop();
      const insertAt = lastInFolder ? rest.indexOf(lastInFolder) + 1 : rest.length;
      rest.splice(insertAt, 0, dragSessionId);
    } else {
      const targetFolder = folderMap[target.id];
      if (targetFolder) nextFolders[dragSessionId] = targetFolder;
      else delete nextFolders[dragSessionId];
      const targetIdx = rest.indexOf(target.id);
      const insertAt = targetIdx < 0 ? rest.length : target.before ? targetIdx : targetIdx + 1;
      rest.splice(insertAt, 0, dragSessionId);
    }

    const stillEmpty = emptyFolderList.filter((name) => !Object.values(nextFolders).includes(name));
    setEmptyFolderList(stillEmpty);
    writeJson(EMPTY_FOLDERS_KEY, stillEmpty);

    persistFolders(nextFolders);
    persistOrder(rest);
  };

  const clearDragState = () => {
    setDragId(null);
    setDropTarget(null);
  };

  const onSessionDragOver = (event: React.DragEvent, session: Session) => {
    if (!dragId || dragId === session.id) return;
    event.preventDefault();
    event.stopPropagation();
    const rect = event.currentTarget.getBoundingClientRect();
    const before = event.clientY < rect.top + rect.height / 2;
    setDropTarget({ kind: 'session', id: session.id, before });
  };

  const onFolderDragOver = (event: React.DragEvent, name: string) => {
    if (!dragId) return;
    event.preventDefault();
    event.stopPropagation();
    setDropTarget({ kind: 'folder', name });
  };

  const onDropOn = (event: React.DragEvent, target: DropTarget) => {
    event.preventDefault();
    event.stopPropagation();
    const id = event.dataTransfer.getData(DRAG_MIME) || dragId;
    if (id) applyDrop(id, target);
    clearDragState();
  };

  const isNavItemActive = (item: NavItem) =>
    item.exact ? pathname === item.to : pathname === item.to || pathname.startsWith(`${item.to}/`);

  const renderNavLink = (item: NavItem, className?: string) => {
    const Icon = item.icon;
    const active = isNavItemActive(item);
    return (
      <Link key={item.to} to={item.to} className={className}>
        <div
          className={cn(
            'flex cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition-colors',
            active
              ? 'bg-sidebar-accent font-medium text-sidebar-accent-foreground'
              : 'text-sidebar-foreground hover:bg-sidebar-accent/50',
          )}
        >
          <Icon className="h-4 w-4 shrink-0" />
          <span className="min-w-0 flex-1 truncate">{t(item.labelKey)}</span>
        </div>
      </Link>
    );
  };

  const renderDrillButton = (
    key: SidebarView,
    icon: typeof Wrench,
    label: string,
    childItems: NavItem[],
  ) => {
    const Icon = icon;
    const active = view === key || childItems.some(isNavItemActive);
    return (
      <button
        type="button"
        onClick={() => setManualView(key)}
        className={cn(
          'flex w-full cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 text-left text-sm transition-colors',
          active
            ? 'bg-sidebar-accent font-medium text-sidebar-accent-foreground'
            : 'text-sidebar-foreground hover:bg-sidebar-accent/50',
        )}
      >
        <Icon className="h-4 w-4 shrink-0" />
        <span className="min-w-0 flex-1 truncate">{label}</span>
        <ChevronRight className="h-4 w-4 shrink-0 opacity-50" />
      </button>
    );
  };

  const renderSessionRow = (session: Session) => {
    const active = activeSessionId === session.id;
    const editing = editingSessionId === session.id;
    const label = labelOf(session);
    const isDragging = dragId === session.id;
    const isDropHere = dropTarget?.kind === 'session' && dropTarget.id === session.id;
    return (
      <div
        key={session.id}
        ref={(node) => {
          if (node) rowRefs.current.set(session.id, node);
          else rowRefs.current.delete(session.id);
        }}
        draggable={!editing}
        onDragStart={(event) => {
          event.dataTransfer.setData(DRAG_MIME, session.id);
          event.dataTransfer.effectAllowed = 'move';
          setDragId(session.id);
        }}
        onDragEnd={clearDragState}
        onDragOver={(event) => onSessionDragOver(event, session)}
        onDrop={(event) =>
          onDropOn(event, {
            kind: 'session',
            id: session.id,
            before: dropTarget?.kind === 'session' ? dropTarget.before : true,
          })
        }
        onClick={() => {
          if (!editing) onSelectSession(session.id);
        }}
        className={cn(
          'group relative flex min-h-8 cursor-grab items-center rounded-lg px-2.5 py-1.5 text-sm transition-colors active:cursor-grabbing',
          active ? 'bg-sidebar-accent text-sidebar-accent-foreground' : 'hover:bg-sidebar-accent/50',
          busyId === session.id && 'opacity-60',
          isDragging && 'opacity-40',
        )}
      >
        {isDropHere ? (
          <span
            className={cn(
              'pointer-events-none absolute inset-x-2 h-0.5 rounded bg-primary',
              dropTarget?.before ? '-top-0.5' : '-bottom-0.5',
            )}
          />
        ) : null}
        {editing ? (
          <div className="flex w-full min-w-0 items-center gap-1">
            <input
              autoFocus
              value={editingTitle}
              onChange={(event) => setEditingTitle(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') void saveRename();
                if (event.key === 'Escape') setEditingSessionId(null);
              }}
              onClick={(event) => event.stopPropagation()}
              className="min-w-0 flex-1 rounded border bg-background px-2 py-0.5 text-sm outline-none focus:border-primary"
            />
            <button
              type="button"
              onClick={(event) => {
                event.stopPropagation();
                void saveRename();
              }}
              title={t('common.save')}
              className="shrink-0 rounded p-0.5 hover:bg-sidebar-accent"
            >
              <Check className="h-3.5 w-3.5" />
            </button>
            <button
              type="button"
              onClick={(event) => {
                event.stopPropagation();
                setEditingSessionId(null);
              }}
              title={t('common.cancel')}
              className="shrink-0 rounded p-0.5 hover:bg-sidebar-accent"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        ) : (
          <>
            <span className={cn(
              'min-w-0 flex-1 truncate pr-0 group-hover:pr-12 group-focus-within:pr-12',
              !session.title && 'text-muted-foreground',
            )}>
              {label}
            </span>
            {/* Actions overlay on hover — do not reserve permanent width. */}
            <div className="pointer-events-none absolute inset-y-0 right-1 flex items-center gap-0.5 opacity-0 group-hover:pointer-events-auto group-hover:opacity-100 group-focus-within:pointer-events-auto group-focus-within:opacity-100">
              <button
                type="button"
                onClick={(event) => {
                  event.stopPropagation();
                  startRename(session);
                }}
                title={t('common.edit')}
                className="rounded p-1 hover:bg-sidebar-accent"
              >
                <Pencil className="h-3.5 w-3.5" />
              </button>
              <button
                type="button"
                onClick={(event) => {
                  event.stopPropagation();
                  void onDeleteSession(session.id);
                }}
                title={t('common.delete')}
                className="rounded p-1 text-destructive hover:bg-destructive/10"
              >
                <Trash2 className="h-3.5 w-3.5" />
              </button>
            </div>
          </>
        )}
      </div>
    );
  };

  const renderFolderBlock = (name: string, items: Session[]) => {
    const open = isFolderOpen(name);
    const isDrop = dropTarget?.kind === 'folder' && dropTarget.name === name;
    const isVirtual = name === uncategorized;
    const isPinned = pinnedFolders.includes(name);
    const editing = editingFolder === name;

    return (
      <div key={name}>
        <div
          onDragOver={(event) => onFolderDragOver(event, name)}
          onDrop={(event) => onDropOn(event, { kind: 'folder', name })}
          className={cn(
            'group/folder relative flex min-h-8 items-center gap-1 rounded-lg px-2.5 py-1.5 text-sm text-sidebar-foreground transition-colors hover:bg-sidebar-accent/50',
            isDrop && 'bg-sidebar-accent',
          )}
        >
          <button
            type="button"
            onClick={() => toggleFolder(name)}
            className="flex min-w-0 flex-1 items-center gap-2 text-left"
          >
            {open ? (
              <FolderOpen className="h-3.5 w-3.5 shrink-0 opacity-70" />
            ) : (
              <Folder className="h-3.5 w-3.5 shrink-0 opacity-70" />
            )}
            {editing ? (
              <input
                autoFocus
                value={editingFolderName}
                onChange={(event) => setEditingFolderName(event.target.value)}
                onClick={(event) => event.stopPropagation()}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') saveRenameFolder();
                  if (event.key === 'Escape') setEditingFolder(null);
                }}
                className="min-w-0 flex-1 rounded border bg-background px-1.5 py-0.5 text-sm outline-none focus:border-primary"
              />
            ) : (
              <span className="min-w-0 flex-1 truncate font-medium">
                {name}
                {isPinned ? <Pin className="ml-1 inline h-3 w-3 align-[-2px] opacity-60" /> : null}
              </span>
            )}
          </button>

          {!editing && (
            <div className="pointer-events-none flex shrink-0 items-center opacity-0 group-hover/folder:pointer-events-auto group-hover/folder:opacity-100 group-focus-within/folder:pointer-events-auto group-focus-within/folder:opacity-100">
              <button
                type="button"
                onClick={(event) => {
                  event.stopPropagation();
                  void createSessionInFolder(isVirtual ? undefined : name);
                }}
                title={t('sidebar.newSession')}
                className="rounded p-1 hover:bg-sidebar-accent"
              >
                <Plus className="h-3.5 w-3.5" />
              </button>
              {!isVirtual ? (
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <button
                      type="button"
                      onClick={(event) => event.stopPropagation()}
                      title={t('sidebar.more')}
                      aria-label={t('sidebar.more')}
                      className="rounded p-1 hover:bg-sidebar-accent"
                    >
                      <MoreHorizontal className="h-3.5 w-3.5" />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-48">
                    <DropdownMenuItem onClick={() => startRenameFolder(name)}>
                      <Pencil className="h-3.5 w-3.5" />
                      {t('sidebar.renameFolder')}
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => togglePinFolder(name)}>
                      {isPinned ? <PinOff className="h-3.5 w-3.5" /> : <Pin className="h-3.5 w-3.5" />}
                      {isPinned ? t('sidebar.unpinFolder') : t('sidebar.pinFolder')}
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                      className="text-destructive focus:text-destructive"
                      onClick={() => setRemoveFolderTarget(name)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                      {t('sidebar.removeFolder')}
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              ) : null}
              <button
                type="button"
                onClick={(event) => {
                  event.stopPropagation();
                  toggleFolder(name);
                }}
                title={open ? t('common.close') : t('common.open')}
                aria-expanded={open}
                className="rounded p-1 hover:bg-sidebar-accent"
              >
                <ChevronDown
                  className={cn(
                    'h-3.5 w-3.5 text-muted-foreground',
                    !open && '-rotate-90',
                  )}
                />
              </button>
            </div>
          )}
        </div>
        {open && items.length > 0 ? (
          <div className="space-y-0.5 pl-5">
            {items.map((session) => renderSessionRow(session))}
          </div>
        ) : null}
      </div>
    );
  };

  const allFolders: Array<[string, Session[]]> = [
    ...namedFolderEntries,
    [uncategorized, uncategorizedSessions] as [string, Session[]],
  ];

  const renderSessions = () => (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex items-center justify-between px-3 pb-1 pt-3">
        <span className="text-xs font-medium text-muted-foreground">{t('layout.sessions')}</span>
        <div className="flex items-center gap-0.5">
          <button
            type="button"
            onClick={createFolder}
            title={t('sidebar.newFolder')}
            className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
          >
            <FolderPlus className="h-3.5 w-3.5" />
          </button>
          <button
            type="button"
            onClick={() => void createSessionInFolder()}
            title={t('sidebar.newSession')}
            className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
          >
            <Plus className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>
      <ScrollArea className="min-h-0 flex-1 px-2 pb-2">
        {sessions.length === 0 ? (
          <div className="px-3 py-8 text-center text-xs text-muted-foreground">{t('sessionDrawer.empty')}</div>
        ) : (
          <div
            className="space-y-0.5"
            onDragOver={(event) => {
              if (dragId) event.preventDefault();
            }}
          >
            {allFolders.map(([name, items]) => renderFolderBlock(name, items))}
          </div>
        )}
      </ScrollArea>
    </div>
  );

  const rootBody = (
    <>
      <div className="px-2 pt-2">
        <button
          type="button"
          onClick={() => void createSessionInFolder()}
          className="flex w-full cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 text-sm text-sidebar-foreground transition-colors hover:bg-sidebar-accent/50"
        >
          <Plus className="h-4 w-4 shrink-0" />
          <span>{t('sidebar.newSession')}</span>
        </button>
      </div>
      <nav className="space-y-0.5 px-2 py-1">
        {renderNavLink(DASHBOARD_ITEM)}
        {renderDrillButton('toolbox', Wrench, t('nav.toolbox'), TOOLBOX_ITEMS)}
        {renderDrillButton('vivy', Sparkles, t('nav.vivy'), VIVY_ITEMS)}
      </nav>
      {renderSessions()}
    </>
  );

  const renderDrillHeader = (icon: typeof Wrench, title: string) => {
    const Icon = icon;
    return (
      <div className="flex items-center gap-1 px-2 pb-1 pt-3">
        <button
          type="button"
          onClick={() => setManualView('root')}
          aria-label={t('nav.back')}
          title={t('nav.back')}
          className="-ml-1 rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-sidebar-accent/50 hover:text-sidebar-foreground"
        >
          <ChevronLeft className="h-4 w-4 shrink-0" />
        </button>
        <div className="flex min-w-0 items-center gap-2 text-sm font-semibold text-foreground">
          <Icon className="h-4 w-4 shrink-0" />
          <span className="truncate">{title}</span>
        </div>
      </div>
    );
  };

  const toolboxBody = (
    <>
      {renderDrillHeader(Wrench, t('nav.toolbox'))}
      <nav className="space-y-0.5 px-2 py-1">{TOOLBOX_ITEMS.map((item) => renderNavLink(item))}</nav>
    </>
  );

  const vivyBody = (
    <>
      {renderDrillHeader(Sparkles, t('nav.vivy'))}
      <nav className="space-y-0.5 px-2 py-1">{VIVY_ITEMS.map((item) => renderNavLink(item))}</nav>
    </>
  );

  return (
    <aside className="flex h-full flex-col bg-sidebar">
      <div className="flex items-center gap-2.5 px-4 py-4">
        <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary">
          <span className="text-sm font-bold text-primary-foreground">V</span>
        </div>
        <span className="text-lg font-semibold text-foreground">Vivy</span>
      </div>
      <div className="flex min-h-0 flex-1 flex-col">
        {view === 'toolbox' ? toolboxBody : view === 'vivy' ? vivyBody : rootBody}
      </div>
      <div className="border-t border-sidebar-border p-2">
        {renderNavLink(
          { to: '/settings', icon: Settings, labelKey: 'nav.settings' },
        )}
      </div>
      <AlertDialog open={removeFolderTarget != null} onOpenChange={(open) => { if (!open) setRemoveFolderTarget(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('sidebar.removeFolder')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('sidebar.removeFolderConfirm', { name: removeFolderTarget ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmRemoveFolder}>{t('common.delete')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </aside>
  );
}
