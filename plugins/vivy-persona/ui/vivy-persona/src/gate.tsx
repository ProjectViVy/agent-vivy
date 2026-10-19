import { useEffect, useState } from 'react';
import { usePluginHost, usePluginTranslation } from '@vivy/ui-sdk';
import { useFaceState } from './face-state';
import { dismissPersonaOnboarding, isPersonaOnboardingDismissed } from './dismissal';
import { PersonaClient } from './persona-client';
import { PersonaOnboarding } from './onboarding';

/**
 * 首抵门禁：宿主在欢迎向导关闭后渲染 shell.onboarding 槽位，是否显示由本模块决定。
 *
 * 只有后端回答「未初始化」时才出现——人格状态永远由权威回答，本组件不做推断；
 * 已经初始化、没有会话或权威不可用时一律安静退出，不以坏掉的面板打扰用户。
 */
export function PersonaOnboardingGate() {
  const host = usePluginHost();
  const { t } = usePluginTranslation();
  const faceState = useFaceState(host);
  const sessionId = faceState.activeSessionId;
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    if (!host || !sessionId || isPersonaOnboardingDismissed()) {
      setVisible(false);
      return;
    }
    const client = PersonaClient.fromRPC(host.rpc);
    let active = true;
    setVisible(false);
    void (async () => {
      try {
        const outcome = await client.status(sessionId);
        if (active) setVisible(outcome.status === 'ok' && outcome.value?.persona.state === 'uninitialized');
      } catch {
        if (active) setVisible(false);
      }
    })();
    return () => { active = false; };
  }, [host, sessionId]);

  if (!visible || !host || !sessionId) return null;
  const client = PersonaClient.fromRPC(host.rpc);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={t('plugin.vivy/persona.onboarding.dialogLabel')}
      className="fixed inset-0 z-[190] flex items-center justify-center bg-background/80 p-4 backdrop-blur-sm animate-in fade-in-0 duration-200 motion-reduce:animate-none"
    >
      <PersonaOnboarding
        client={client}
        sessionId={sessionId}
        className="max-h-[88vh] w-full max-w-2xl shadow-lg"
        finishLabelKey="plugin.vivy/persona.onboarding.startChat"
        onFinish={() => setVisible(false)}
        secondaryLabelKey="plugin.vivy/persona.onboarding.openPersona"
        onSecondary={() => {
          setVisible(false);
          void host.router.navigate({ to: '/persona' });
        }}
        onSkip={() => {
          dismissPersonaOnboarding();
          setVisible(false);
        }}
      />
    </div>
  );
}
