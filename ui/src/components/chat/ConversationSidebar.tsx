import { Link, useRouterState } from '@tanstack/react-router';
import { Brain, Clock, Dna, LayoutDashboard, MessageSquare, NotebookPen, Plug, Settings, ShieldCheck, UserRound, VenetianMask, Zap } from 'lucide-react';
import { ScrollArea } from '@/components/ui/scroll-area';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/i18n';

const NAV_ITEMS = [{ to: '/', icon: MessageSquare, labelKey: 'nav.chat', exact: true }, { to: '/dashboard', icon: LayoutDashboard, labelKey: 'nav.dashboard' }, { to: '/approvals', icon: ShieldCheck, labelKey: 'nav.approvals' }, { to: '/cron-tasks', icon: Clock, labelKey: 'nav.cron' }] as const;
const VIVY_ITEMS = [{ to: '/persona', icon: UserRound, labelKey: 'nav.persona' }, { to: '/masks', icon: VenetianMask, labelKey: 'nav.masks' }, { to: '/evolution', icon: Dna, labelKey: 'nav.evolution' }, { to: '/memory', icon: Brain, labelKey: 'nav.memory' }, { to: '/notebook', icon: NotebookPen, labelKey: 'nav.notebook' }] as const;
const TOOL_ITEMS = [{ to: '/mcp', icon: Plug, labelKey: 'nav.mcp' }, { to: '/skills', icon: Zap, labelKey: 'nav.skill' }] as const;

export function ConversationSidebar() {
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const { t } = useTranslation();
  const renderNavItem = (item: (typeof NAV_ITEMS)[number] | (typeof VIVY_ITEMS)[number] | (typeof TOOL_ITEMS)[number]) => {
    const Icon = item.icon;
    const label = t(item.labelKey);
    const active = 'exact' in item && item.exact ? pathname === item.to : pathname.startsWith(item.to);
    return <Link key={`${item.to}-${label}`} to={item.to}><div className={cn('flex cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition-colors', active ? 'bg-sidebar-accent font-medium text-sidebar-accent-foreground' : 'text-sidebar-foreground hover:bg-sidebar-accent/50')}><Icon className="h-4 w-4 shrink-0"/><span>{label}</span></div></Link>;
  };
  return <aside className="flex h-full flex-col bg-sidebar">
    <div className="flex items-center gap-2.5 px-4 py-4"><div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary"><span className="text-sm font-bold text-primary-foreground">V</span></div><span className="text-lg font-semibold text-foreground">Vivy</span></div>
    <ScrollArea className="flex-1 px-3"><nav className="space-y-0.5 py-2">{NAV_ITEMS.map(renderNavItem)}<div className="px-3 pb-1 pt-3"><span className="text-xs font-medium text-muted-foreground">Vivy</span></div>{VIVY_ITEMS.map(renderNavItem)}<div className="px-3 pb-1 pt-3"><span className="text-xs font-medium text-muted-foreground">{t('nav.toolsGroup')}</span></div>{TOOL_ITEMS.map(renderNavItem)}</nav></ScrollArea>
    <div className="border-t border-sidebar-border p-3"><Link to="/settings"><div className={cn('flex cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition-colors', pathname === '/settings' ? 'bg-sidebar-accent font-medium text-sidebar-accent-foreground' : 'text-sidebar-foreground hover:bg-sidebar-accent/50')}><Settings className="h-4 w-4 shrink-0"/><span>{t('nav.settings')}</span></div></Link></div>
  </aside>;
}
