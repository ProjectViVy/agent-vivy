import { createFileRoute } from '@tanstack/react-router';
import { Button } from '@/components/ui/button';
import { ChatView } from '@/components/chat/ChatView';
import { useVivyStore } from '@/lib/store';
import { useTranslation } from '@/i18n';

export const Route = createFileRoute('/_layout/')({ component: Index });
function Index() {
  const activeSessionId = useVivyStore((state) => state.activeSessionId);
  const sessionsPhase = useVivyStore((state) => state.sessionsPhase);
  const sessionsError = useVivyStore((state) => state.sessionsError);
  const sessionBusyId = useVivyStore((state) => state.sessionBusyId);
  const createSession = useVivyStore((state) => state.createSession);
  const { t } = useTranslation();
  return <div className="h-full overflow-hidden">{activeSessionId ? <ChatView sessionId={activeSessionId}/> : <div className="flex h-full items-center justify-center p-6 text-center text-muted-foreground"><div>{sessionsPhase === 'error' ? <><p className="text-lg text-foreground">{t('app.sessionCreateFailed')}</p><p className="mt-2 max-w-md text-sm">{sessionsError}</p><Button className="mt-4" disabled={sessionBusyId === 'create'} onClick={() => void createSession()}>{sessionBusyId === 'create' ? t('app.creating') : t('common.retry')}</Button></> : <><p className="text-lg">{t('app.creatingSession')}</p><p className="mt-1 text-sm">{t('app.almostReady')}</p></>}</div></div>}</div>;
}
