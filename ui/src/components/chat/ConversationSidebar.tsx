import { useEffect, useMemo, useState } from 'react';
import { Link, useRouterState } from '@tanstack/react-router';
import type { LucideIcon } from 'lucide-react';
import {
  Brain, Check, ChevronDown, ChevronLeft, ChevronRight, Clock, Dna, Folder, FolderOpen,
  LayoutDashboard, NotebookPen, Pencil, Plug, Plus, Settings, ShieldCheck, Sparkles,
  Trash2, UserRound, VenetianMask, Wrench, X, Zap,
} from 'lucide-react';
import type { Session } from '@/lib/api';
import { ScrollArea } from '@/components/ui/scroll-area';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/i18n';
import { groupSessionsByWorkspace } from './session-workspaces';

type SidebarView = 'root' | 'toolbox' | 'vivy';
type NavItem = { to: string; icon: LucideIcon; labelKey: string; exact?: boolean };

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

function viewForPath(pathname: string): SidebarView {
  if (pathname === '/' || pathname.startsWith('/sessions')) return 'root';
  if (TOOLBOX_ITEMS.some((item) => pathname === item.to || pathname.startsWith(`${item.to}/`))) return 'toolbox';
  if (VIVY_ITEMS.some((item) => pathname === item.to || pathname.startsWith(`${item.to}/`))) return 'vivy';
  return 'root';
}

interface Props {
  sessions: Session[];
  activeSessionId: string | null;
  busyId: string | null;
  onSelectSession: (id: string) => void;
  onRenameSession: (id: string, title: string) => Promise<void>;
  onDeleteSession: (id: string) => Promise<void>;
  onCreateSession: (workspacePath?: string) => Promise<Session | null> | Session | null | void;
}

export function ConversationSidebar({
  sessions, activeSessionId, busyId, onSelectSession, onRenameSession, onDeleteSession, onCreateSession,
}: Props) {
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const { t } = useTranslation();
  const pathView = viewForPath(pathname);
  const [manualView, setManualView] = useState<SidebarView | null>(null);
  const [workspaceOpen, setWorkspaceOpen] = useState<Record<string, boolean>>({});
  const [editingSessionId, setEditingSessionId] = useState<string | null>(null);
  const [editingTitle, setEditingTitle] = useState('');
  const view = manualView ?? pathView;

  useEffect(() => setManualView(null), [pathView]);
  const groups = useMemo(() => groupSessionsByWorkspace(sessions, t('workspace.default')), [sessions, t]);
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
          <span className="min-w-0 flex-1 truncate">{t(item.labelKey)}</span>
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

  const renderSessions = () => (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex items-center justify-between px-3 pb-1 pt-3">
        <span className="text-xs font-medium text-muted-foreground">{t('layout.sessions')}</span>
        <button type="button" onClick={() => void onCreateSession('')} title={t('sidebar.newSession')} className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"><Plus className="h-3.5 w-3.5" /></button>
      </div>
      <ScrollArea className="min-h-0 flex-1 px-2 pb-2">
        {sessions.length === 0 ? <div className="px-3 py-8 text-center text-xs text-muted-foreground">{t('sessionDrawer.empty')}</div> : (
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
                  <button type="button" onClick={() => void onCreateSession(group.key)} title={t('sidebar.newSession')} className="pointer-events-none rounded p-1 opacity-0 hover:bg-sidebar-accent group-hover/workspace:pointer-events-auto group-hover/workspace:opacity-100"><Plus className="h-3.5 w-3.5" /></button>
                </div>
                {open ? <div className="space-y-0.5 pl-5">{group.sessions.map(renderSession)}</div> : null}
              </div>;
            })}
          </div>
        )}
      </ScrollArea>
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
    <nav className="space-y-0.5 px-2 py-1">{renderNavLink(DASHBOARD_ITEM)}{renderDrillButton('toolbox', Wrench, t('nav.toolbox'), TOOLBOX_ITEMS)}{renderDrillButton('vivy', Sparkles, t('nav.vivy'), VIVY_ITEMS)}</nav>
    {renderSessions()}
  </>;

  return <aside className="flex h-full flex-col bg-sidebar">
    <div className="flex items-center gap-2.5 px-4 py-4"><div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary"><span className="text-sm font-bold text-primary-foreground">V</span></div><span className="text-lg font-semibold text-foreground">Vivy</span></div>
    <div className="flex min-h-0 flex-1 flex-col">{view === 'toolbox' ? <>{renderDrillHeader(Wrench, t('nav.toolbox'))}<nav className="space-y-0.5 px-2 py-1">{TOOLBOX_ITEMS.map(renderNavLink)}</nav></> : view === 'vivy' ? <>{renderDrillHeader(Sparkles, t('nav.vivy'))}<nav className="space-y-0.5 px-2 py-1">{VIVY_ITEMS.map(renderNavLink)}</nav></> : rootBody}</div>
    <div className="border-t border-sidebar-border p-2">{renderNavLink({ to: '/settings', icon: Settings, labelKey: 'nav.settings' })}</div>
  </aside>;
}
