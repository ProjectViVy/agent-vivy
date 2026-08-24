import { useEffect, useState } from 'react';
import { createFileRoute, Outlet, useNavigate, useRouterState } from '@tanstack/react-router';
import { ListTodo, Menu, MessageSquare, Wifi, WifiOff } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Sheet, SheetBody, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet';
import { ConversationSidebar } from '@/components/chat/ConversationSidebar';
import { SessionDrawer } from '@/components/chat/SessionDrawer';
import { PlanSidebarPanel } from '@/components/planning/PlanSidebarPanel';
import { ApprovalsView } from '@/components/approvals/ApprovalsView';
import { getPlanSidebarData } from '@/lib/demo-api';
import type { PlanSidebarData } from '@/lib/types';
import { useVivyStore } from '@/lib/store';
import { useIsMobile } from '@/hooks/use-mobile';
import { MaskAndModelSwitcher } from '@/components/chat/MaskAndModelSwitcher';

export const Route = createFileRoute('/_layout')({ component: Layout });

function Layout() {
  const mobile = useIsMobile();
  const navigate = useNavigate();
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const [collapsed, setCollapsed] = useState(false);
  const [sessionOpen, setSessionOpen] = useState(false);
  const [planData, setPlanData] = useState<PlanSidebarData | null>(null);
  const sessions = useVivyStore((state) => state.sessions);
  const activeSessionId = useVivyStore((state) => state.activeSessionId);
  const busyId = useVivyStore((state) => state.sessionBusyId);
  const sessionsError = useVivyStore((state) => state.sessionsError);
  const createSession = useVivyStore((state) => state.createSession);
  const deleteSession = useVivyStore((state) => state.deleteSession);
  const renameSession = useVivyStore((state) => state.renameSession);
  const selectSession = useVivyStore((state) => state.selectSession);
  const connection = useVivyStore((state) => state.connection);
  const run = useVivyStore((state) => state.currentRun);
  const reviewCenterOpen = useVivyStore((state) => state.reviewCenterOpen);
  const setReviewCenterOpen = useVivyStore((state) => state.setReviewCenterOpen);
  const reviewBusyId = useVivyStore((state) => state.reviewBusyId);

  useEffect(() => { getPlanSidebarData().then(setPlanData); }, []);
  useEffect(() => { if (mobile) setCollapsed(true); }, [mobile, pathname]);
  const connected = connection === 'connected';
  const createAndOpen = async () => { try { await createSession(); setSessionOpen(false); await navigate({ to: '/' }); if (mobile) setCollapsed(true); } catch { /* store exposes the error beside the action */ } };
  const selectAndOpen = async (id: string) => { await selectSession(id); setSessionOpen(false); await navigate({ to: '/' }); };

  return <div className="flex h-screen bg-background">
    {mobile && !collapsed ? <button aria-label="关闭导航" className="fixed inset-0 z-30 bg-black/30" onClick={() => setCollapsed(true)}/> : null}
    <div className={`transition-[width] duration-200 ${collapsed ? 'w-0 overflow-hidden' : mobile ? 'fixed inset-y-0 left-0 z-40 w-72 shadow-xl' : 'w-72'}`}><ConversationSidebar onCreateSession={() => void createAndOpen()} creating={busyId === 'create'} createError={sessionsError}/></div>
    <div className="flex min-w-0 flex-1 flex-col">
      <header className="flex h-14 items-center justify-between border-b bg-card px-4">
        <div className="flex items-center gap-3"><Button variant="ghost" size="icon" aria-label={collapsed ? '打开导航' : '收起导航'} title={collapsed ? '打开导航' : '收起导航'} onClick={() => setCollapsed(!collapsed)}><Menu className="h-5 w-5"/></Button><div className="flex items-center gap-2"><div className="flex h-9 w-9 items-center justify-center rounded-full bg-accent text-lg">😊</div><div><div className="text-sm font-semibold leading-tight">Vivy</div><div className="text-xs leading-tight text-muted-foreground">{run ? run.status : '开心'}</div></div><span className={`ml-1 flex items-center gap-1 rounded-full px-2 py-0.5 text-xs ${connected ? 'bg-emerald-500/10 text-emerald-600' : 'bg-amber-500/10 text-amber-600'}`}>{connected ? <Wifi className="h-3 w-3"/> : <WifiOff className="h-3 w-3"/>}{connected ? '在线' : connection}</span></div></div>
        <div className="hidden min-w-0 flex-1 justify-center px-2 md:flex"><MaskAndModelSwitcher/></div>
        <div className="flex items-center gap-1">
          <Sheet open={sessionOpen} onOpenChange={setSessionOpen}><SheetTrigger asChild><Button variant="ghost" size="icon" title="会话"><MessageSquare className="h-5 w-5"/></Button></SheetTrigger><SheetContent side="right" className="w-[360px] p-0"><SheetHeader className="shrink-0 border-b px-4 py-3"><SheetTitle>会话</SheetTitle></SheetHeader><SheetBody><SessionDrawer sessions={sessions} activeSessionId={activeSessionId} busyId={busyId} onSelectSession={(id) => void selectAndOpen(id)} onCreateSession={() => void createAndOpen()} onRenameSession={renameSession} onDeleteSession={deleteSession}/></SheetBody></SheetContent></Sheet>
          <Sheet><SheetTrigger asChild><Button variant="ghost" size="icon" title="待办事项"><ListTodo className="h-5 w-5"/></Button></SheetTrigger><SheetContent side="right" className="w-[min(400px,100vw)] p-0"><SheetHeader className="shrink-0 border-b px-4 py-3"><SheetTitle>待办事项</SheetTitle></SheetHeader><SheetBody className="overflow-auto p-4"><PlanSidebarPanel plan={planData?.plan ?? null} todos={planData?.todos ?? []} validationIssues={planData?.validation_issues}/></SheetBody></SheetContent></Sheet>
        </div>
      </header>
      <main className="min-h-0 flex-1 overflow-hidden"><Outlet/></main>
    </div>
    <Sheet open={reviewCenterOpen} onOpenChange={(open) => { if (open || !reviewBusyId) setReviewCenterOpen(open); }}><SheetContent side="right" className="w-full p-0 sm:max-w-[560px]" closeDisabled={!!reviewBusyId} onEscapeKeyDown={(event) => { if (reviewBusyId) event.preventDefault(); }} onPointerDownOutside={(event) => { if (reviewBusyId) event.preventDefault(); }}><SheetHeader className="shrink-0 border-b px-4 py-3"><SheetTitle>审批中心</SheetTitle></SheetHeader><SheetBody><ApprovalsView panel/></SheetBody></SheetContent></Sheet>
  </div>;
}
