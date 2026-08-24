import { Link, useRouterState } from '@tanstack/react-router';
import { Brain, Cat, Clock, Dna, LayoutDashboard, MessageSquare, NotebookPen, Plug, Plus, Settings, VenetianMask, Zap } from 'lucide-react';
import { ScrollArea } from '@/components/ui/scroll-area';
import { cn } from '@/lib/utils';

const NAV_ITEMS = [{ to: '/', icon: MessageSquare, label: '聊天', exact: true }, { to: '/pet', icon: Cat, label: '宠物' }, { to: '/dashboard', icon: LayoutDashboard, label: '中控台' }, { to: '/cron-tasks', icon: Clock, label: '定时任务' }] as const;
const VIVY_ITEMS = [{ to: '/persona', icon: VenetianMask, label: '面具' }, { to: '/skills', icon: Dna, label: '进化' }, { to: '/memory', icon: Brain, label: '记忆' }, { to: '/notebook', icon: NotebookPen, label: '记事本' }] as const;
const TOOL_ITEMS = [{ to: '/mcp', icon: Plug, label: 'MCP' }, { to: '/skills', icon: Zap, label: 'Skill' }] as const;

interface ConversationSidebarProps { onCreateSession: () => void; creating?: boolean; createError?: string | null }

export function ConversationSidebar({ onCreateSession, creating, createError }: ConversationSidebarProps) {
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const renderNavItem = (item: (typeof NAV_ITEMS)[number] | (typeof VIVY_ITEMS)[number] | (typeof TOOL_ITEMS)[number]) => {
    const active = 'exact' in item && item.exact ? pathname === item.to : pathname.startsWith(item.to);
    const Icon = item.icon;
    return <Link key={`${item.to}-${item.label}`} to={item.to}><div className={cn('flex cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition-colors', active ? 'bg-sidebar-accent font-medium text-sidebar-accent-foreground' : 'text-sidebar-foreground hover:bg-sidebar-accent/50')}><Icon className="h-4 w-4 shrink-0"/><span>{item.label}</span></div></Link>;
  };
  return <aside className="flex h-full flex-col bg-sidebar">
    <div className="flex items-center gap-2.5 px-4 py-4"><div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary"><span className="text-sm font-bold text-primary-foreground">V</span></div><span className="text-lg font-semibold text-foreground">Vivy</span></div>
    <div className="px-3 pb-2"><button type="button" aria-label="新会话" title="新会话" onClick={onCreateSession} disabled={creating} className="flex w-full items-center justify-center rounded-xl border border-border bg-card py-2.5 text-sm text-muted-foreground transition-colors hover:bg-accent disabled:opacity-50"><Plus className="h-4 w-4"/></button>{createError ? <p className="mt-2 rounded-lg bg-destructive/10 px-3 py-2 text-xs text-destructive">{createError}</p> : null}</div>
    <ScrollArea className="flex-1 px-3"><nav className="space-y-0.5 py-2">{NAV_ITEMS.map(renderNavItem)}<div className="px-3 pb-1 pt-3"><span className="text-xs font-medium text-muted-foreground">Vivy</span></div>{VIVY_ITEMS.map(renderNavItem)}<div className="px-3 pb-1 pt-3"><span className="text-xs font-medium text-muted-foreground">工具管理</span></div>{TOOL_ITEMS.map(renderNavItem)}</nav></ScrollArea>
    <div className="border-t border-sidebar-border p-3"><Link to="/settings"><div className={cn('flex cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition-colors', pathname === '/settings' ? 'bg-sidebar-accent font-medium text-sidebar-accent-foreground' : 'text-sidebar-foreground hover:bg-sidebar-accent/50')}><Settings className="h-4 w-4 shrink-0"/><span>设置</span></div></Link></div>
  </aside>;
}
