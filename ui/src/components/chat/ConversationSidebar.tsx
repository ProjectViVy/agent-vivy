import { useEffect, useMemo, useState } from 'react';
import { Link, useRouterState } from '@tanstack/react-router';
import {
  ChevronDown, ChevronLeft, ChevronRight, Folder, FolderOpen, FolderPlus,
  Plus, Search, SlidersHorizontal, X,
} from 'lucide-react';
import type { Session } from '@/lib/api';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/i18n';
import { SIDEBAR_VIVY_GROUP, useGroupedNavigation } from '@/plugins/grouped-navigation';
import { resolveHostIcon } from '@/plugins/host-icons';
import { usePluginHost, type HostIconName } from '@vivy/ui-sdk';
import { groupSessionsByWorkspace } from './session-workspaces';
import {
  filterSessions, readSessionListView, storeSessionListView, type SessionListView,
} from './session-list-view';
import { WorkspaceFolderDialog } from './WorkspaceFolderDialog';
import { SessionRow } from './session-row';

type SidebarView = 'root' | 'toolbox' | 'vivy';
/** Every entry names its left icon from the host icon set, core or Module. */
type NavItem = { to: string; icon: HostIconName; labelKey: string; label?: string; exact?: boolean };
/** One VIVY entry with the order that places it among assembled entries. */
type OrderedNavItem = { readonly order: number; readonly item: NavItem };

const DASHBOARD_ITEM: NavItem = { to: '/dashboard', icon: 'dashboard', labelKey: 'nav.dashboard' };
const TOOLBOX_ITEMS: NavItem[] = [
  { to: '/cron-tasks', icon: 'clock', labelKey: 'nav.cron' },
  { to: '/mcp', icon: 'plug', labelKey: 'nav.mcp' },
  { to: '/skills', icon: 'zap', labelKey: 'nav.skill' },
  { to: '/approvals', icon: 'shield-check', labelKey: 'nav.approvals' },
];
/**
 * The VIVY group is fully assembled from Module navigation contributions
 * (人格 / 进化 / 记忆 / 记事本 / 面具) — the shell owns no hard-coded entry
 * here; the Recipe decides which Modules exist.
 */
const VIEW_MODES: SessionListView[] = ['grouped', 'flat'];

function viewForPath(pathname: string, vivyItems: readonly NavItem[]): SidebarView {
  if (pathname === '/' || pathname.startsWith('/sessions')) return 'root';
  if (TOOLBOX_ITEMS.some((item) => pathname === item.to || pathname.startsWith(`${item.to}/`))) return 'toolbox';
  if (vivyItems.some((item) => pathname === item.to || pathname.startsWith(`${item.to}/`))) return 'vivy';
  return 'root';
}

/** Orders Module entries; ties keep Recipe registration order. */
function assembleVivyItems(contributed: readonly OrderedNavItem[]): NavItem[] {
  return contributed
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
  const vivyItems = useMemo(() => assembleVivyItems(contributedVivy.map((entry) => ({
    order: entry.order,
    item: {
      to: entry.to,
      icon: entry.icon ?? 'sparkles',
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
  const view = manualView ?? pathView;
  const searching = query.trim().length > 0;

  useEffect(() => setManualView(null), [pathView]);

  const groups = useMemo(() => groupSessionsByWorkspace(sessions, t('workspace.default')), [sessions, t]);
  const matches = useMemo(() => filterSessions(sessions, query, {
    defaultWorkspace: t('workspace.default'),
    untitled: t('errors.newSessionDefault'),
  }), [sessions, query, t]);
  const isNavItemActive = (item: NavItem) =>
    item.exact ? pathname === item.to : pathname === item.to || pathname.startsWith(`${item.to}/`);

  const renderNavLink = (item: NavItem) => {
    const Icon = resolveHostIcon(item.icon);
    const active = isNavItemActive(item);
    return (
      <Link key={item.to} to={item.to}>
        <div className={cn(
          'flex cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 text-ui transition-colors',
          active ? 'bg-sidebar-accent font-medium text-sidebar-accent-foreground' : 'text-sidebar-foreground hover:bg-sidebar-accent/50',
        )} title={item.label ?? navT(item.labelKey)}>
          <Icon className="h-4 w-4 shrink-0" />
          <span className="min-w-0 flex-1 line-clamp-2 break-words">{item.label ?? navT(item.labelKey)}</span>
        </div>
      </Link>
    );
  };

  const renderDrillButton = (key: SidebarView, icon: HostIconName, label: string, childItems: NavItem[]) => {
    const Icon = resolveHostIcon(icon);
    const active = view === key || childItems.some(isNavItemActive);
    return (
      <button type="button" onClick={() => setManualView(key)} title={label} className={cn(
        'flex w-full cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 text-left text-ui transition-colors',
        active ? 'bg-sidebar-accent font-medium text-sidebar-accent-foreground' : 'text-sidebar-foreground hover:bg-sidebar-accent/50',
      )}>
        <Icon className="h-4 w-4 shrink-0" />
        <span className="min-w-0 flex-1 line-clamp-2 break-words">{label}</span>
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
      <SessionRow
        key={session.id}
        title={session.title || t('errors.newSessionDefault')}
        untitled={!session.title}
        active={activeSessionId === session.id}
        busy={busyId === session.id}
        onSelect={() => onSelectSession(session.id)}
        rename={{
          editing,
          draft: editingTitle,
          onDraftChange: setEditingTitle,
          onStart: () => { setEditingSessionId(session.id); setEditingTitle(session.title); },
          onSave: () => void saveRename(),
          onCancel: () => setEditingSessionId(null),
        }}
        onDelete={() => void onDeleteSession(session.id)}
      />
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
          <Search className="h-3.5 w-3.5 shrink-0 text-fg-muted" aria-hidden="true" />
          <input
            autoFocus
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={(event) => { if (event.key === 'Escape') closeSearch(); }}
            placeholder={t('sessionDrawer.searchPlaceholder')}
            aria-label={t('sessionDrawer.searchPlaceholder')}
            className="min-w-0 flex-1 bg-transparent text-ui text-fg outline-none placeholder:text-fg-muted"
          />
          <button type="button" onClick={closeSearch} title={t('common.cancel')} aria-label={t('common.cancel')} className="rounded-md p-1 text-fg-muted transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"><X className="h-3.5 w-3.5" /></button>
        </>
      ) : (
        <>
          <span className="min-w-0 flex-1 truncate text-meta font-medium text-fg-muted">{t('layout.sessions')}</span>
          <button type="button" onClick={() => setSearchOpen(true)} title={t('sidebar.search')} aria-label={t('sidebar.search')} className="rounded-md p-1 text-fg-muted transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"><Search className="h-3.5 w-3.5" /></button>
          <DropdownMenu open={viewMenuOpen} onOpenChange={setViewMenuOpen}>
            <DropdownMenuTrigger asChild>
              <button type="button" title={t('sidebar.view')} aria-label={t('sidebar.view')} className="rounded-md p-1 text-fg-muted transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"><SlidersHorizontal className="h-3.5 w-3.5" /></button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-44">
              <DropdownMenuRadioGroup value={viewMode} onValueChange={(value) => selectViewMode(value as SessionListView)}>
                {VIEW_MODES.map((mode) => (
                  <DropdownMenuRadioItem key={mode} value={mode} className="text-ui">
                    {t(mode === 'grouped' ? 'sidebar.viewGrouped' : 'sidebar.viewFlat')}
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
            </DropdownMenuContent>
          </DropdownMenu>
          <button type="button" onClick={() => setFolderPickerOpen(true)} title={t('sidebar.openFolder')} aria-label={t('sidebar.openFolder')} className="rounded-md p-1 text-fg-muted transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"><FolderPlus className="h-3.5 w-3.5" /></button>
        </>
      )}
    </div>
  );

  const renderSessionList = () => {
    if (sessions.length === 0) {
      return <div className="px-3 py-8 text-center text-meta text-fg-muted">{t('sessionDrawer.empty')}</div>;
    }
    if (searching) {
      return matches.length === 0
        ? <div className="px-3 py-8 text-center text-meta text-fg-muted">{t('sessionDrawer.noMatch')}</div>
        : <div className="space-y-0.5">{matches.map(renderSession)}</div>;
    }
    if (viewMode === 'flat') return <div className="space-y-0.5">{sessions.map(renderSession)}</div>;
    return (
      <div className="space-y-0.5">
        {groups.map((group) => {
          const open = workspaceOpen[group.key] ?? true;
          return <div key={group.key || '__default__'}>
            <div className="group/workspace flex min-h-8 items-center gap-1 rounded-lg px-2.5 py-1.5 text-ui text-sidebar-foreground hover:bg-sidebar-accent/50" title={group.key || t('workspace.default')}>
              <button type="button" onClick={() => setWorkspaceOpen((current) => ({ ...current, [group.key]: !open }))} className="flex min-w-0 flex-1 items-center gap-2 text-left" aria-expanded={open}>
                {open ? <FolderOpen className="h-3.5 w-3.5 shrink-0 opacity-70" /> : <Folder className="h-3.5 w-3.5 shrink-0 opacity-70" />}
                <span className="min-w-0 flex-1 line-clamp-2 break-words font-medium">{group.label}</span>
                <span className="text-2xs tabular-nums text-fg-muted">{group.sessions.length}</span>
                <ChevronDown className={cn('h-3.5 w-3.5 shrink-0 text-fg-muted transition-transform', !open && '-rotate-90')} />
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

  const renderDrillHeader = (icon: HostIconName, title: string) => {
    const Icon = resolveHostIcon(icon);
    return <div className="flex items-center gap-1 px-2 pb-1 pt-3">
      <button type="button" onClick={() => setManualView('root')} aria-label={t('nav.back')} title={t('nav.back')} className="-ml-1 rounded-md p-1.5 text-fg-muted transition-colors hover:bg-sidebar-accent/50 hover:text-sidebar-foreground"><ChevronLeft className="h-4 w-4" /></button>
      <div className="flex min-w-0 items-center gap-2 text-ui font-semibold text-fg"><Icon className="h-4 w-4 shrink-0" /><span className="line-clamp-2 break-words" title={title}>{title}</span></div>
    </div>;
  };

  const rootBody = <>
    <div className="px-2 pt-2"><button type="button" onClick={() => void onCreateSession('')} className="flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-ui text-sidebar-foreground transition-colors hover:bg-sidebar-accent/50"><Plus className="h-4 w-4" /><span>{t('sidebar.newSession')}</span></button></div>
    <nav className="space-y-0.5 px-2 py-1">{renderNavLink(DASHBOARD_ITEM)}{renderDrillButton('toolbox', 'wrench', t('nav.toolbox'), TOOLBOX_ITEMS)}{renderDrillButton('vivy', 'sparkles', t('nav.vivy'), vivyItems)}</nav>
    {renderSessions()}
  </>;

  return <aside className="flex h-full flex-col bg-sidebar">
    <div className="flex items-center gap-2.5 px-4 py-4"><div className="flex h-8 w-8 items-center justify-center rounded-lg bg-brand"><span className="text-ui font-bold text-brand-fg">V</span></div><span className="text-title font-semibold text-fg">Vivy</span></div>
    <div className="flex min-h-0 flex-1 flex-col">{view === 'toolbox' ? <>{renderDrillHeader('wrench', t('nav.toolbox'))}<nav className="space-y-0.5 px-2 py-1">{TOOLBOX_ITEMS.map(renderNavLink)}</nav></> : view === 'vivy' ? <>{renderDrillHeader('sparkles', t('nav.vivy'))}<nav className="space-y-0.5 px-2 py-1">{vivyItems.map(renderNavLink)}</nav></> : rootBody}</div>
    <div className="border-t border-sidebar-border p-2">{renderNavLink({ to: '/settings', icon: 'settings', labelKey: 'nav.settings' })}</div>
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
