import { useEffect, useMemo, useRef, useState, type ComponentType } from 'react';
import { Link, useRouterState } from '@tanstack/react-router';
import {
  Check, ChevronDown, ChevronLeft, ChevronRight, Clock, Folder, FolderOpen, FolderPlus,
  LayoutDashboard, Pencil, Plug, Plus, Search, Settings, ShieldCheck, SlidersHorizontal,
  Sparkles, Trash2, VenetianMask, Wrench, X, Zap,
} from 'lucide-react';
import type { Session } from '@/lib/api';
import { ScrollArea } from '@/components/ui/scroll-area';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/i18n';
import { SIDEBAR_VIVY_GROUP, useGroupedNavigation } from '@/plugins/grouped-navigation';
import { usePluginHost } from '@vivy/ui-sdk';
import { groupSessionsByWorkspace } from './session-workspaces';
import {
  filterSessions, readSessionListView, storeSessionListView, type SessionListView,
} from './session-list-view';
import { WorkspaceFolderDialog } from './WorkspaceFolderDialog';

type SidebarView = 'root' | 'toolbox' | 'vivy';
type NavIcon = ComponentType<{ className?: string }>;
type NavItem = { to: string; icon: NavIcon; labelKey: string; label?: string; exact?: boolean };
/** One VIVY entry with the order that places it among assembled entries. */
type OrderedNavItem = { readonly order: number; readonly item: NavItem };

const DASHBOARD_ITEM: NavItem = { to: '/dashboard', icon: LayoutDashboard, labelKey: 'nav.dashboard' };
const TOOLBOX_ITEMS: NavItem[] = [
  { to: '/cron-tasks', icon: Clock, labelKey: 'nav.cron' },
  { to: '/mcp', icon: Plug, labelKey: 'nav.mcp' },
  { to: '/skills', icon: Zap, labelKey: 'nav.skill' },
  { to: '/approvals', icon: ShieldCheck, labelKey: 'nav.approvals' },
];
/**
 * The VIVY group is assembled: 面具 is a core entry this shell always owns, and
 * 人格 / 进化 / 记忆 / 记事本 arrive as grouped navigation contributions from
 * the Modules the Recipe selected. Order 20 keeps 面具 second in the default
 * profile without pinning it ahead of a Module that claims an earlier slot.
 */
const CORE_VIVY_ITEMS: OrderedNavItem[] = [
  { order: 20, item: { to: '/masks', icon: VenetianMask, labelKey: 'nav.masks' } },
];
const VIEW_MODES: SessionListView[] = ['grouped', 'flat'];

function viewForPath(pathname: string, vivyItems: readonly NavItem[]): SidebarView {
  if (pathname === '/' || pathname.startsWith('/sessions')) return 'root';
  if (TOOLBOX_ITEMS.some((item) => pathname === item.to || pathname.startsWith(`${item.to}/`))) return 'toolbox';
  if (vivyItems.some((item) => pathname === item.to || pathname.startsWith(`${item.to}/`))) return 'vivy';
  return 'root';
}

/** Merges core and Module entries by order; ties keep core first, then Recipe order. */
function assembleVivyItems(core: readonly OrderedNavItem[], contributed: readonly OrderedNavItem[]): NavItem[] {
  return [...core, ...contributed]
    .map((entry, index) => ({ entry, index }))
    .sort((left, right) => left.entry.order - right.entry.order || left.index - right.index)
    .map((ranked) => ranked.entry.item);
}

interface Props {
  sessions: Session[];
  activeSessionId: string | null;
  busyId: string | null;
  onSelectSession: (id: string) => void;
  onRenameSession: (id: string, title: string) => Promise<void>;
  onDeleteSession: (id: string) => Promise<void>;
  onCreateSession: (workspacePath?: string) => Promise<Session | null> | Session | null | void;
  onChooseWorkspace: (workspacePath: string) => Promise<unknown> | unknown;
}

export function ConversationSidebar({
  sessions, activeSessionId, busyId, onSelectSession, onRenameSession, onDeleteSession,
  onCreateSession, onChooseWorkspace,
}: Props) {
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const { t } = useTranslation();
  const pluginHost = usePluginHost();
  // A Module entry names its own label, so the group surface resolves labels
  // through the host translator: it reads the selected Modules' sealed
  // `plugin.*` copy and still falls back to the core dictionary the shell owns.
  const navT = pluginHost?.t ?? t;
  const contributedVivy = useGroupedNavigation(SIDEBAR_VIVY_GROUP, pluginHost);
  const vivyItems = useMemo(() => assembleVivyItems(CORE_VIVY_ITEMS, contributedVivy.map((entry) => ({
    order: entry.order,
    item: {
      to: entry.to,
      icon: entry.icon ?? Sparkles,
      labelKey: entry.labelKey,
      label: entry.label,
      exact: entry.exact,
    },
  }))), [contributedVivy]);
  const pathView = viewForPath(pathname, vivyItems);
  const [manualView, setManualView] = useState<SidebarView | null>(null);
  const [workspaceOpen, setWorkspaceOpen] = useState<Record<string, boolean>>({});
  const [editingSessionId, setEditingSessionId] = useState<string | null>(null);
  const [editingTitle, setEditingTitle] = useState('');
  const [viewMode, setViewMode] = useState<SessionListView>(() => readSessionListView());
  const [viewMenuOpen, setViewMenuOpen] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [folderPickerOpen, setFolderPickerOpen] = useState(false);
  const viewMenuRef = useRef<HTMLDivElement>(null);
  const view = manualView ?? pathView;
  const searching = query.trim().length > 0;

  useEffect(() => setManualView(null), [pathView]);
  useEffect(() => {
    if (!viewMenuOpen) return;
    const onPointerDown = (event: PointerEvent) => {
      if (viewMenuRef.current?.contains(event.target as Node)) return;
      setViewMenuOpen(false);
    };
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') setViewMenuOpen(false); };
    document.addEventListener('pointerdown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('pointerdown', onPointerDown);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [viewMenuOpen]);

  const groups = useMemo(() => groupSessionsByWorkspace(sessions, t('workspace.default')), [sessions, t]);
  const matches = useMemo(() => filterSessions(sessions, query, {
    defaultWorkspace: t('workspace.default'),
    untitled: t('errors.newSessionDefault'),
  }), [sessions, query, t]);
  const isNavItemActive = (item: NavItem) =>
    item.exact ? pathname === item.to : pathname === item.to || pathname.startsWith(`${item.to}/`);

  const renderNavLink = (item: NavItem) => {
    const Icon = item.icon;
    const active = isNavItemActive(item);
    return (
      <Link key={item.to} to={item.to}>
        <div className={cn(
          'flex cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition-colors',
          active ? 'bg-sidebar-accent font-medium text-sidebar-accent-foreground' : 'text-sidebar-foreground hover:bg-sidebar-accent/50',
        )}>
          <Icon className="h-4 w-4 shrink-0" />
          <span className="min-w-0 flex-1 truncate">{item.label ?? navT(item.labelKey)}</span>
        </div>
      </Link>
    );
  };

  const renderDrillButton = (key: SidebarView, icon: typeof Wrench, label: string, childItems: NavItem[]) => {
    const Icon = icon;
    const active = view === key || childItems.some(isNavItemActive);
    return (
      <button type="button" onClick={() => setManualView(key)} className={cn(
        'flex w-full cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 text-left text-sm transition-colors',
        active ? 'bg-sidebar-accent font-medium text-sidebar-accent-foreground' : 'text-sidebar-foreground hover:bg-sidebar-accent/50',
      )}>
        <Icon className="h-4 w-4 shrink-0" />
        <span className="min-w-0 flex-1 truncate">{label}</span>
        <ChevronRight className="h-4 w-4 shrink-0 opacity-50" />
      </button>
    );
  };

  const saveRename = async () => {
    if (!editingSessionId || !editingTitle.trim()) return;
    await onRenameSession(editingSessionId, editingTitle.trim());
    setEditingSessionId(null);
  };

  const renderSession = (session: Session) => {
    const editing = editingSessionId === session.id;
    return (
      <div
        key={session.id}
        onClick={() => { if (!editing) onSelectSession(session.id); }}
        className={cn(
          'group relative flex min-h-8 cursor-pointer items-center rounded-lg px-2.5 py-1.5 text-sm transition-colors',
          activeSessionId === session.id ? 'bg-sidebar-accent text-sidebar-accent-foreground' : 'hover:bg-sidebar-accent/50',
          busyId === session.id && 'opacity-60',
        )}
      >
        {editing ? (
          <div className="flex w-full min-w-0 items-center gap-1">
            <input
              autoFocus value={editingTitle} onChange={(event) => setEditingTitle(event.target.value)}
              onKeyDown={(event) => { if (event.key === 'Enter') void saveRename(); if (event.key === 'Escape') setEditingSessionId(null); }}
              onClick={(event) => event.stopPropagation()}
              className="min-w-0 flex-1 rounded border bg-background px-2 py-0.5 text-sm outline-none focus:border-primary"
            />
            <button type="button" onClick={(event) => { event.stopPropagation(); void saveRename(); }} title={t('common.save')} className="rounded p-0.5 hover:bg-sidebar-accent"><Check className="h-3.5 w-3.5" /></button>
            <button type="button" onClick={(event) => { event.stopPropagation(); setEditingSessionId(null); }} title={t('common.cancel')} className="rounded p-0.5 hover:bg-sidebar-accent"><X className="h-3.5 w-3.5" /></button>
          </div>
        ) : (
          <>
            <span className={cn('min-w-0 flex-1 truncate group-hover:pr-12', !session.title && 'text-muted-foreground')}>
              {session.title || t('errors.newSessionDefault')}
            </span>
            <div className="pointer-events-none absolute inset-y-0 right-1 flex items-center gap-0.5 opacity-0 group-hover:pointer-events-auto group-hover:opacity-100 group-focus-within:pointer-events-auto group-focus-within:opacity-100">
              <button type="button" onClick={(event) => { event.stopPropagation(); setEditingSessionId(session.id); setEditingTitle(session.title); }} title={t('common.edit')} className="rounded p-1 hover:bg-sidebar-accent"><Pencil className="h-3.5 w-3.5" /></button>
              <button type="button" onClick={(event) => { event.stopPropagation(); void onDeleteSession(session.id); }} title={t('common.delete')} className="rounded p-1 text-destructive hover:bg-destructive/10"><Trash2 className="h-3.5 w-3.5" /></button>
            </div>
          </>
        )}
      </div>
    );
  };

  const closeSearch = () => { setSearchOpen(false); setQuery(''); };
  const selectViewMode = (mode: SessionListView) => {
    setViewMode(mode);
    storeSessionListView(mode);
    setViewMenuOpen(false);
  };

  // Section header actions, mirroring the harness sidebar: search the list,
  // choose how it is laid out, and open a folder to work in.
  const renderSessionsHeader = () => (
    <div className="flex min-h-7 items-center gap-0.5 px-3 pb-1 pt-3">
      {searchOpen ? (
        <>
          <Search className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
          <input
            autoFocus
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={(event) => { if (event.key === 'Escape') closeSearch(); }}
            placeholder={t('sessionDrawer.searchPlaceholder')}
            aria-label={t('sessionDrawer.searchPlaceholder')}
            className="min-w-0 flex-1 bg-transparent text-xs text-foreground outline-none placeholder:text-muted-foreground"
          />
          <button type="button" onClick={closeSearch} title={t('common.cancel')} aria-label={t('common.cancel')} className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"><X className="h-3.5 w-3.5" /></button>
        </>
      ) : (
        <>
          <span className="min-w-0 flex-1 truncate text-xs font-medium text-muted-foreground">{t('layout.sessions')}</span>
          <button type="button" onClick={() => setSearchOpen(true)} title={t('sidebar.search')} aria-label={t('sidebar.search')} className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"><Search className="h-3.5 w-3.5" /></button>
          <div ref={viewMenuRef} className="relative">
            <button type="button" onClick={() => setViewMenuOpen((open) => !open)} title={t('sidebar.view')} aria-label={t('sidebar.view')} aria-haspopup="menu" aria-expanded={viewMenuOpen} className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"><SlidersHorizontal className="h-3.5 w-3.5" /></button>
            {viewMenuOpen ? (
              <div role="menu" className="absolute right-0 top-7 z-20 w-44 rounded-lg border bg-popover p-1 text-popover-foreground shadow-md">
                {VIEW_MODES.map((mode) => (
                  <button
                    key={mode}
                    type="button"
                    role="menuitemradio"
                    aria-checked={viewMode === mode}
                    onClick={() => selectViewMode(mode)}
                    className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs transition-colors hover:bg-accent hover:text-accent-foreground"
                  >
                    <Check className={cn('h-3.5 w-3.5 shrink-0', viewMode !== mode && 'opacity-0')} />
                    <span className="min-w-0 flex-1 truncate">{t(mode === 'grouped' ? 'sidebar.viewGrouped' : 'sidebar.viewFlat')}</span>
                  </button>
                ))}
              </div>
            ) : null}
          </div>
          <button type="button" onClick={() => setFolderPickerOpen(true)} title={t('sidebar.openFolder')} aria-label={t('sidebar.openFolder')} className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"><FolderPlus className="h-3.5 w-3.5" /></button>
        </>
      )}
    </div>
  );

  const renderSessionList = () => {
    if (sessions.length === 0) {
      return <div className="px-3 py-8 text-center text-xs text-muted-foreground">{t('sessionDrawer.empty')}</div>;
    }
    if (searching) {
      return matches.length === 0
        ? <div className="px-3 py-8 text-center text-xs text-muted-foreground">{t('sessionDrawer.noMatch')}</div>
        : <div className="space-y-0.5">{matches.map(renderSession)}</div>;
    }
    if (viewMode === 'flat') return <div className="space-y-0.5">{sessions.map(renderSession)}</div>;
    return (
      <div className="space-y-0.5">
        {groups.map((group) => {
          const open = workspaceOpen[group.key] ?? true;
          return <div key={group.key || '__default__'}>
            <div className="group/workspace flex min-h-8 items-center gap-1 rounded-lg px-2.5 py-1.5 text-sm text-sidebar-foreground hover:bg-sidebar-accent/50" title={group.key || t('workspace.default')}>
              <button type="button" onClick={() => setWorkspaceOpen((current) => ({ ...current, [group.key]: !open }))} className="flex min-w-0 flex-1 items-center gap-2 text-left" aria-expanded={open}>
                {open ? <FolderOpen className="h-3.5 w-3.5 shrink-0 opacity-70" /> : <Folder className="h-3.5 w-3.5 shrink-0 opacity-70" />}
                <span className="min-w-0 flex-1 truncate font-medium">{group.label}</span>
                <span className="text-[10px] tabular-nums text-muted-foreground">{group.sessions.length}</span>
                <ChevronDown className={cn('h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform', !open && '-rotate-90')} />
              </button>
              <button type="button" onClick={() => void onCreateSession(group.key)} title={t('sidebar.newSession')} aria-label={t('sidebar.newSession')} className="pointer-events-none rounded p-1 opacity-0 hover:bg-sidebar-accent group-hover/workspace:pointer-events-auto group-hover/workspace:opacity-100"><Plus className="h-3.5 w-3.5" /></button>
            </div>
            {open ? <div className="space-y-0.5 pl-5">{group.sessions.map(renderSession)}</div> : null}
          </div>;
        })}
      </div>
    );
  };

  const renderSessions = () => (
    <div className="flex min-h-0 flex-1 flex-col">
      {renderSessionsHeader()}
      <ScrollArea className="min-h-0 flex-1 px-2 pb-2">{renderSessionList()}</ScrollArea>
    </div>
  );

  const renderDrillHeader = (icon: typeof Wrench, title: string) => {
    const Icon = icon;
    return <div className="flex items-center gap-1 px-2 pb-1 pt-3">
      <button type="button" onClick={() => setManualView('root')} aria-label={t('nav.back')} title={t('nav.back')} className="-ml-1 rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-sidebar-accent/50 hover:text-sidebar-foreground"><ChevronLeft className="h-4 w-4" /></button>
      <div className="flex min-w-0 items-center gap-2 text-sm font-semibold text-foreground"><Icon className="h-4 w-4 shrink-0" /><span className="truncate">{title}</span></div>
    </div>;
  };

  const rootBody = <>
    <div className="px-2 pt-2"><button type="button" onClick={() => void onCreateSession('')} className="flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-sm text-sidebar-foreground transition-colors hover:bg-sidebar-accent/50"><Plus className="h-4 w-4" /><span>{t('sidebar.newSession')}</span></button></div>
    <nav className="space-y-0.5 px-2 py-1">{renderNavLink(DASHBOARD_ITEM)}{renderDrillButton('toolbox', Wrench, t('nav.toolbox'), TOOLBOX_ITEMS)}{renderDrillButton('vivy', Sparkles, t('nav.vivy'), vivyItems)}</nav>
    {renderSessions()}
  </>;

  return <aside className="flex h-full flex-col bg-sidebar">
    <div className="flex items-center gap-2.5 px-4 py-4"><div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary"><span className="text-sm font-bold text-primary-foreground">V</span></div><span className="text-lg font-semibold text-foreground">Vivy</span></div>
    <div className="flex min-h-0 flex-1 flex-col">{view === 'toolbox' ? <>{renderDrillHeader(Wrench, t('nav.toolbox'))}<nav className="space-y-0.5 px-2 py-1">{TOOLBOX_ITEMS.map(renderNavLink)}</nav></> : view === 'vivy' ? <>{renderDrillHeader(Sparkles, t('nav.vivy'))}<nav className="space-y-0.5 px-2 py-1">{vivyItems.map(renderNavLink)}</nav></> : rootBody}</div>
    <div className="border-t border-sidebar-border p-2">{renderNavLink({ to: '/settings', icon: Settings, labelKey: 'nav.settings' })}</div>
    <WorkspaceFolderDialog
      open={folderPickerOpen}
      onOpenChange={setFolderPickerOpen}
      startPath=""
      title={t('workspace.enterTitle')}
      description={t('workspace.enterDescription')}
      onPick={onChooseWorkspace}
    />
  </aside>;
}
