import { useEffect, useState } from 'react';
import { createFileRoute, Outlet, useNavigate, useRouterState } from '@tanstack/react-router';
import { ListTodo, Folder, Menu, MessageSquare, Wifi, WifiOff } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Sheet, SheetBody, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet';
import { ConversationSidebar } from '@/components/chat/ConversationSidebar';
import { SessionDrawer } from '@/components/chat/SessionDrawer';
import { SessionTodoPanel } from '@/components/planning/SessionTodoPanel';
import { ApprovalsView } from '@/components/approvals/ApprovalsView';
import { FilesPanel } from '@/components/files/FilesPanel';
import { WelcomeWizard } from '@/components/layout/WelcomeWizard';
import { useVivyStore } from '@/lib/store';
import { useIsMobile } from '@/hooks/use-mobile';
import { isWelcomeCompleted, openWelcome } from '@/hooks/use-welcome';
import { MaskAndModelSwitcher } from '@/components/chat/MaskAndModelSwitcher';
import { useTranslation } from '@/i18n';
import { cn } from '@/lib/utils';

export const Route = createFileRoute('/_layout')({ component: Layout });

function Layout() {
  const mobile = useIsMobile();
  const navigate = useNavigate();
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const { t } = useTranslation();
  const [desktopCollapsed, setDesktopCollapsed] = useState(false);
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const sessions = useVivyStore((state) => state.sessions);
  const activeSessionId = useVivyStore((state) => state.activeSessionId);
  const busyId = useVivyStore((state) => state.sessionBusyId);
  const deleteSession = useVivyStore((state) => state.deleteSession);
  const renameSession = useVivyStore((state) => state.renameSession);
  const selectSession = useVivyStore((state) => state.selectSession);
  const connection = useVivyStore((state) => state.connection);
  const run = useVivyStore((state) => state.currentRun);
  const reviewCenterOpen = useVivyStore((state) => state.reviewCenterOpen);
  const setReviewCenterOpen = useVivyStore((state) => state.setReviewCenterOpen);
  const filesPanelOpen = useVivyStore((state) => state.filesPanelOpen);
  const setFilesPanelOpen = useVivyStore((state) => state.setFilesPanelOpen);
  const sessionDrawerOpen = useVivyStore((state) => state.sessionDrawerOpen);
  const setSessionDrawerOpen = useVivyStore((state) => state.setSessionDrawerOpen);
  const todoPanelOpen = useVivyStore((state) => state.todoPanelOpen);
  const setTodoPanelOpen = useVivyStore((state) => state.setTodoPanelOpen);
  const reviewBusyId = useVivyStore((state) => state.reviewBusyId);
  const initialized = useVivyStore((state) => state.initialized);
  const initializationError = useVivyStore((state) => state.initializationError);

  useEffect(() => { setMobileNavOpen(false); }, [pathname]);
  // 首次使用（完成标记未写入）且后端初始化成功时，自动弹出欢迎向导。
  useEffect(() => {
    if (initialized && !initializationError && !isWelcomeCompleted()) openWelcome();
  }, [initialized, initializationError]);

  const connected = connection === 'connected';
  const navClosed = mobile ? !mobileNavOpen : desktopCollapsed;
  const selectAndOpen = async (id: string) => {
    await selectSession(id);
    setSessionDrawerOpen(false);
    setMobileNavOpen(false);
    await navigate({ to: '/' });
  };
  const toggleNav = () => {
    if (mobile) setMobileNavOpen((open) => !open);
    else setDesktopCollapsed((collapsed) => !collapsed);
  };
  const toggleTodos = () => setTodoPanelOpen(!todoPanelOpen);

  const sidebar = <ConversationSidebar />;

  return (
    <div className="flex h-dvh bg-background pb-[env(safe-area-inset-bottom)] pl-[env(safe-area-inset-left)] pr-[env(safe-area-inset-right)] pt-[env(safe-area-inset-top)]">
      <div className={cn('hidden h-full shrink-0 overflow-hidden transition-[width] duration-200 md:block', desktopCollapsed ? 'w-0' : 'w-72')}>
        {sidebar}
      </div>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 shrink-0 items-center gap-2 border-b bg-card px-2 sm:px-4">
          <div className="flex min-w-0 items-center gap-2">
            <Button
              variant="ghost"
              size="icon"
              aria-label={navClosed ? t('layout.openNav') : t('layout.closeNav')}
              title={navClosed ? t('layout.openNav') : t('layout.closeNav')}
              onClick={toggleNav}
            >
              <Menu className="h-5 w-5" />
            </Button>
            <div className="flex min-w-0 items-center gap-2">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-accent text-lg">😊</div>
              <div className="min-w-0">
                <div className="truncate text-sm font-semibold leading-tight">Vivy</div>
                <div className="hidden truncate text-xs leading-tight text-muted-foreground sm:block">{run ? run.status : t('layout.idleMood')}</div>
              </div>
              <span
                className={cn(
                  'ml-1 flex shrink-0 items-center gap-1 rounded-full px-2 py-0.5 text-xs',
                  connected ? 'bg-emerald-500/10 text-emerald-600' : 'bg-amber-500/10 text-amber-600',
                )}
                title={connected ? t('layout.online') : connection}
              >
                {connected ? <Wifi className="h-3 w-3" /> : <WifiOff className="h-3 w-3" />}
                <span className="hidden sm:inline">{connected ? t('layout.online') : connection}</span>
              </span>
            </div>
          </div>
          <div className="flex min-w-0 flex-1 justify-center">
            <MaskAndModelSwitcher />
          </div>
          <div className="flex shrink-0 items-center gap-1">
            <Sheet open={sessionDrawerOpen} onOpenChange={setSessionDrawerOpen}>
              <SheetTrigger asChild>
                <Button variant="ghost" size="icon" title={t('layout.sessions')} aria-label={t('layout.sessions')}><MessageSquare className="h-5 w-5" /></Button>
              </SheetTrigger>
              <SheetContent side="right" className="w-full sm:max-w-[360px]">
                <SheetHeader className="border-b">
                  <SheetTitle>{t('layout.sessions')}</SheetTitle>
                </SheetHeader>
                <SheetBody className="overflow-hidden">
                  <SessionDrawer
                    sessions={sessions}
                    activeSessionId={activeSessionId}
                    busyId={busyId}
                    onSelectSession={(id) => void selectAndOpen(id)}
                    onRenameSession={renameSession}
                    onDeleteSession={deleteSession}
                  />
                </SheetBody>
              </SheetContent>
            </Sheet>
            <Button
              variant="ghost"
              size="icon"
              className="hidden md:inline-flex"
              title={t('layout.todos')}
              aria-label={t('layout.todos')}
              aria-expanded={todoPanelOpen}
              aria-controls="session-todo-panel"
              onClick={toggleTodos}
            >
              <ListTodo className="h-5 w-5" />
            </Button>
            <Sheet open={mobile && todoPanelOpen} onOpenChange={setTodoPanelOpen}>
              <SheetTrigger asChild>
                <Button variant="ghost" size="icon" className="md:hidden" title={t('layout.todos')} aria-label={t('layout.todos')}><ListTodo className="h-5 w-5" /></Button>
              </SheetTrigger>
              <SheetContent side="right" className="w-full sm:max-w-[400px]">
                <SheetHeader className="sr-only">
                  <SheetTitle>{t('layout.todos')}</SheetTitle>
                </SheetHeader>
                <SheetBody className="overflow-hidden p-0">
                  <SessionTodoPanel />
                </SheetBody>
              </SheetContent>
            </Sheet>
            <Button
              variant="ghost"
              size="icon"
              title={t('layout.files')}
              aria-label={t('layout.files')}
              aria-expanded={filesPanelOpen}
              onClick={() => setFilesPanelOpen(!filesPanelOpen)}
            >
              <Folder className="h-5 w-5" />
            </Button>
          </div>
        </header>
        <main className="min-h-0 flex-1 overflow-hidden"><Outlet /></main>
      </div>
      {mobile ? (
        <Sheet open={mobileNavOpen} onOpenChange={setMobileNavOpen}>
          <SheetContent side="left" className="w-72 max-w-full [&>button]:hidden">
            {sidebar}
          </SheetContent>
        </Sheet>
      ) : null}
      <Sheet open={reviewCenterOpen} onOpenChange={(open) => { if (open || !reviewBusyId) setReviewCenterOpen(open); }}>
        <SheetContent
          side="right"
          className="w-full sm:max-w-[560px]"
          closeDisabled={!!reviewBusyId}
          onEscapeKeyDown={(event) => { if (reviewBusyId) event.preventDefault(); }}
          onPointerDownOutside={(event) => { if (reviewBusyId) event.preventDefault(); }}
        >
          <SheetHeader className="border-b">
            <SheetTitle>{t('layout.reviewCenter')}</SheetTitle>
          </SheetHeader>
          <SheetBody className="overflow-hidden">
            <ApprovalsView panel />
          </SheetBody>
        </SheetContent>
      </Sheet>
      <Sheet open={filesPanelOpen} onOpenChange={setFilesPanelOpen}>
        <SheetContent side="right" className="w-full sm:max-w-[560px]">
          <SheetHeader className="border-b">
            <SheetTitle>{t('layout.files')}</SheetTitle>
          </SheetHeader>
          <SheetBody className="overflow-hidden p-0">
            <FilesPanel />
          </SheetBody>
        </SheetContent>
      </Sheet>
      <WelcomeWizard />
    </div>
  );
}
