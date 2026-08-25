import { useEffect, useRef, useState } from 'react';
import { Link, useRouterState } from '@tanstack/react-router';
import { Brain, Clock, Dna, LayoutDashboard, MessageSquare, NotebookPen, Plug, Plus, Settings, UserRound, VenetianMask, Zap } from 'lucide-react';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/i18n';

const NAV_ITEMS = [{ to: '/', icon: MessageSquare, labelKey: 'nav.chat', exact: true }, { to: '/dashboard', icon: LayoutDashboard, labelKey: 'nav.dashboard' }, { to: '/cron-tasks', icon: Clock, labelKey: 'nav.cron' }] as const;
const VIVY_ITEMS = [{ to: '/persona', icon: UserRound, labelKey: 'nav.persona' }, { to: '/masks', icon: VenetianMask, labelKey: 'nav.masks' }, { to: '/evolution', icon: Dna, labelKey: 'nav.evolution', pending: true }, { to: '/memory', icon: Brain, labelKey: 'nav.memory' }, { to: '/notebook', icon: NotebookPen, labelKey: 'nav.notebook' }] as const;
const TOOL_ITEMS = [{ to: '/mcp', icon: Plug, labelKey: 'nav.mcp' }, { to: '/skills', icon: Zap, labelKey: 'nav.skill' }] as const;

interface ConversationSidebarProps { onCreateSession: () => void; creating?: boolean; createError?: string | null }

export function ConversationSidebar({ onCreateSession, creating, createError }: ConversationSidebarProps) {
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const { t } = useTranslation();
  const [notice, setNotice] = useState<string | null>(null);
  const noticeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => () => { if (noticeTimer.current) clearTimeout(noticeTimer.current); }, []);
  const showNotice = (message: string) => {
    setNotice(message);
    if (noticeTimer.current) clearTimeout(noticeTimer.current);
    noticeTimer.current = setTimeout(() => setNotice(null), 1800);
  };
  const renderNavItem = (item: (typeof NAV_ITEMS)[number] | (typeof VIVY_ITEMS)[number] | (typeof TOOL_ITEMS)[number]) => {
    const Icon = item.icon;
    const label = t(item.labelKey);
    if ('pending' in item && item.pending) {
      return (
        <button key={`${item.to}-${label}`} type="button" onClick={() => showNotice(t('nav.evolutionUnavailable'))} className="flex w-full cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 text-sm text-sidebar-foreground transition-colors hover:bg-sidebar-accent/50">
          <Icon className="h-4 w-4 shrink-0"/>
          <span className="min-w-0 flex-1 truncate text-left">{label}</span>
          <Badge variant="outline" className="shrink-0 border-sidebar-border px-1.5 py-0 text-[10px] font-normal text-muted-foreground">{t('nav.evolutionPending')}</Badge>
        </button>
      );
    }
    const active = 'exact' in item && item.exact ? pathname === item.to : pathname.startsWith(item.to);
    return <Link key={`${item.to}-${label}`} to={item.to}><div className={cn('flex cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition-colors', active ? 'bg-sidebar-accent font-medium text-sidebar-accent-foreground' : 'text-sidebar-foreground hover:bg-sidebar-accent/50')}><Icon className="h-4 w-4 shrink-0"/><span>{label}</span></div></Link>;
  };
  return <aside className="flex h-full flex-col bg-sidebar">
    <div className="flex items-center gap-2.5 px-4 py-4"><div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary"><span className="text-sm font-bold text-primary-foreground">V</span></div><span className="text-lg font-semibold text-foreground">Vivy</span></div>
    <div className="px-3 pb-2"><button type="button" aria-label={t('nav.newSession')} title={t('nav.newSession')} onClick={onCreateSession} disabled={creating} className="flex w-full items-center justify-center rounded-xl border border-border bg-card py-2.5 text-sm text-muted-foreground transition-colors hover:bg-accent disabled:opacity-50"><Plus className="h-4 w-4"/></button>{createError ? <p className="mt-2 rounded-lg bg-destructive/10 px-3 py-2 text-xs text-destructive">{createError}</p> : null}</div>
    <ScrollArea className="flex-1 px-3"><nav className="space-y-0.5 py-2">{NAV_ITEMS.map(renderNavItem)}<div className="px-3 pb-1 pt-3"><span className="text-xs font-medium text-muted-foreground">Vivy</span></div>{VIVY_ITEMS.map(renderNavItem)}<div className="px-3 pb-1 pt-3"><span className="text-xs font-medium text-muted-foreground">{t('nav.toolsGroup')}</span></div>{TOOL_ITEMS.map(renderNavItem)}</nav></ScrollArea>
    {notice ? <div className="border-t border-sidebar-border px-3 py-2 text-xs text-muted-foreground" aria-live="polite">{notice}</div> : null}
    <div className="border-t border-sidebar-border p-3"><Link to="/settings"><div className={cn('flex cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition-colors', pathname === '/settings' ? 'bg-sidebar-accent font-medium text-sidebar-accent-foreground' : 'text-sidebar-foreground hover:bg-sidebar-accent/50')}><Settings className="h-4 w-4 shrink-0"/><span>{t('nav.settings')}</span></div></Link></div>
  </aside>;
}
