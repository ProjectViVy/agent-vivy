import { useState, type ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import {
  Activity,
  Bot,
  FlaskConical,
  RadioTower,
  SlidersHorizontal,
  Sparkles,
} from 'lucide-react';
import { useTranslation } from '@/i18n';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import {
  DIVA_EVOLUTION_ACTIONS,
  type DivaEvolutionAction,
  type DivaPreviewSection,
} from './diva-preview-data';

type ChatPreviewPrefs = {
  cleanMode: boolean;
  autoExpandReasoning: boolean;
  autoExpandToolDetails: boolean;
  showRawMetaByDefault: boolean;
};

type EvolutionFrequency = 'daily' | 'weekly' | 'manual';

function usePreviewFeedback() {
  const { t } = useTranslation();
  const [feedback, setFeedback] = useState(() => t('divaPreview.defaultFeedback'));
  return { feedback, notify: setFeedback };
}

function PreviewNotice() {
  const { t } = useTranslation();
  return (
    <div className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-xs text-amber-800 dark:text-amber-300">
      <FlaskConical className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
      <span><strong>{t('divaPreview.noticeTitle')}</strong> · {t('divaPreview.noticeBody')}</span>
    </div>
  );
}

function PreviewHeader({ icon: Icon, title, description }: { icon: LucideIcon; title: string; description: string }) {
  return (
    <div className="flex items-center gap-3">
      <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
        <Icon className="h-5 w-5" aria-hidden="true" />
      </div>
      <div>
        <h2 className="text-lg font-semibold">{title}</h2>
        <p className="text-sm text-muted-foreground">{description}</p>
      </div>
    </div>
  );
}

function PreviewFrame({
  icon,
  title,
  description,
  feedback,
  children,
}: {
  icon: LucideIcon;
  title: string;
  description: string;
  feedback: string;
  children: ReactNode;
}) {
  return (
    <div className="space-y-4">
      <PreviewNotice />
      <PreviewHeader icon={icon} title={title} description={description} />
      {children}
      <p className="text-xs text-muted-foreground" aria-live="polite">{feedback}</p>
    </div>
  );
}

function PreviewCard({ title, description, children }: { title: string; description?: string; children: ReactNode }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{title}</CardTitle>
        {description ? <CardDescription>{description}</CardDescription> : null}
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  );
}

function ToggleRow({
  title,
  description,
  checked,
  disabled,
  onCheckedChange,
}: {
  title: string;
  description?: string;
  checked: boolean;
  disabled?: boolean;
  onCheckedChange: (checked: boolean) => void;
}) {
  return (
    <div className="flex items-center justify-between gap-4 rounded-lg border p-3">
      <div className="min-w-0">
        <p className="font-medium">{title}</p>
        {description ? <p className="mt-1 text-sm text-muted-foreground">{description}</p> : null}
      </div>
      <Switch checked={checked} disabled={disabled} onCheckedChange={onCheckedChange} aria-label={title} />
    </div>
  );
}

function GeneralPreview() {
  const { t } = useTranslation();
  const [prefs, setPrefs] = useState<ChatPreviewPrefs>({
    cleanMode: false,
    autoExpandReasoning: true,
    autoExpandToolDetails: false,
    showRawMetaByDefault: false,
  });
  const [cacheCleared, setCacheCleared] = useState(false);
  // 压缩配置已毕业为真实设置：CompactionSettingsCard（SettingsView → 通用）。
  const { feedback, notify } = usePreviewFeedback();

  const updatePref = (key: keyof ChatPreviewPrefs, value: boolean) => {
    setPrefs((current) => ({ ...current, [key]: value }));
    notify(t('divaPreview.chatDisplayUpdated'));
  };

  return (
    <PreviewFrame
      icon={SlidersHorizontal}
      title={t('divaPreview.generalTitle')}
      description={t('divaPreview.generalDescription')}
      feedback={cacheCleared ? t('divaPreview.cacheClearedFeedback') : feedback}
    >
      <PreviewCard title={t('divaPreview.chatDisplayTitle')} description={t('divaPreview.chatDisplayDescription')}>
        <div className="space-y-3">
          <ToggleRow
            title={t('divaPreview.cleanMode')}
            description={t('divaPreview.cleanModeDescription')}
            checked={prefs.cleanMode}
            onCheckedChange={(checked) => updatePref('cleanMode', checked)}
          />
          <ToggleRow
            title={t('divaPreview.autoExpandReasoning')}
            checked={prefs.autoExpandReasoning}
            disabled={prefs.cleanMode}
            onCheckedChange={(checked) => updatePref('autoExpandReasoning', checked)}
          />
          <ToggleRow
            title={t('divaPreview.autoExpandToolDetails')}
            checked={prefs.autoExpandToolDetails}
            disabled={prefs.cleanMode}
            onCheckedChange={(checked) => updatePref('autoExpandToolDetails', checked)}
          />
          <ToggleRow
            title={t('divaPreview.showRawMeta')}
            checked={prefs.showRawMetaByDefault}
            disabled={prefs.cleanMode}
            onCheckedChange={(checked) => updatePref('showRawMetaByDefault', checked)}
          />
        </div>
      </PreviewCard>

      <PreviewCard
        title={t('divaPreview.compactionGraduatedTitle')}
        description={t('divaPreview.compactionGraduatedDescription')}
      >
        <p className="text-sm text-muted-foreground">{t('divaPreview.compactionGraduatedBody')}</p>
      </PreviewCard>

      <PreviewCard title={t('divaPreview.cacheTitle')} description={t('divaPreview.cacheDescription')}>
        <div className="grid gap-3 sm:grid-cols-3">
          <div className="rounded-lg border p-3"><p className="text-xs text-muted-foreground">{t('divaPreview.controlPlane')}</p><p className="mt-1 font-medium text-emerald-600">{t('divaPreview.statusHealthy')}</p></div>
          <div className="rounded-lg border p-3"><p className="text-xs text-muted-foreground">{t('divaPreview.providersLabel')}</p><p className="mt-1 font-medium">2 / 3 {t('divaPreview.readySuffix')}</p></div>
          <div className="rounded-lg border p-3"><p className="text-xs text-muted-foreground">{t('divaPreview.channelsLabel')}</p><p className="mt-1 font-medium">1 / 3 {t('divaPreview.readySuffix')}</p></div>
        </div>
        <Button type="button" variant="outline" className="mt-4" onClick={() => { setCacheCleared(true); notify(t('divaPreview.cacheClearedShort')); }}>
          {cacheCleared ? t('divaPreview.clearCacheDone') : t('divaPreview.clearCache')}
        </Button>
      </PreviewCard>

      <PreviewCard title={t('divaPreview.aboutTitle')} description={t('divaPreview.aboutDescription')}>
        <dl className="grid gap-3 text-sm sm:grid-cols-3">
          <div><dt className="text-muted-foreground">{t('divaPreview.license')}</dt><dd className="mt-1 font-medium">MIT</dd></div>
          <div><dt className="text-muted-foreground">{t('divaPreview.maintainer')}</dt><dd className="mt-1 font-medium">projectViVY</dd></div>
          <div><dt className="text-muted-foreground">{t('divaPreview.uiSource')}</dt><dd className="mt-1 font-medium">{t('divaPreview.uiSourceValue')}</dd></div>
        </dl>
      </PreviewCard>
    </PreviewFrame>
  );
}

function SelfEvolutionPreview() {
  const { t } = useTranslation();
  const [enabled, setEnabled] = useState(true);
  const [frequency, setFrequency] = useState<EvolutionFrequency>('weekly');
  const [sessions, setSessions] = useState(5);
  const [messages, setMessages] = useState(100);
  const [confirmations, setConfirmations] = useState<Record<DivaEvolutionAction, boolean>>({ identity: true, relationship: true, commitment: true, sop: true, deprecation: true });
  const { feedback, notify } = usePreviewFeedback();

  const toggleConfirmation = (action: DivaEvolutionAction, checked: boolean) => {
    setConfirmations((current) => ({ ...current, [action]: checked }));
    notify(t('divaPreview.confirmUpdated', { label: t(`divaPreview.actions.${action}`) }));
  };

  return (
    <PreviewFrame icon={Sparkles} title={t('settings.tabs.selfEvolution')} description={t('divaPreview.selfEvolutionDescription')} feedback={feedback}>
      <PreviewCard title={t('divaPreview.autoTidyTitle')} description={t('divaPreview.autoTidyDescription')}>
        <div className="space-y-4">
          <ToggleRow title={t('divaPreview.enableAutoTidy')} checked={enabled} onCheckedChange={(checked) => { setEnabled(checked); notify(t('divaPreview.autoTidyUpdated')); }} />
          <div className="grid gap-4 sm:grid-cols-3">
            <div className="space-y-2"><Label htmlFor="preview-evolution-frequency">{t('divaPreview.frequencyLabel')}</Label><Select value={frequency} onValueChange={(value: EvolutionFrequency) => { setFrequency(value); notify(t('divaPreview.frequencyUpdated')); }}><SelectTrigger id="preview-evolution-frequency"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="daily">{t('divaPreview.frequencyDaily')}</SelectItem><SelectItem value="weekly">{t('divaPreview.frequencyWeekly')}</SelectItem><SelectItem value="manual">{t('divaPreview.frequencyManual')}</SelectItem></SelectContent></Select></div>
            <div className="space-y-2"><Label htmlFor="preview-evolution-sessions">{t('divaPreview.sessionsLabel')}</Label><Input id="preview-evolution-sessions" type="number" min={1} value={sessions} onChange={(event) => { setSessions(Math.max(1, Number(event.target.value) || 1)); notify(t('divaPreview.sessionsUpdated')); }} /></div>
            <div className="space-y-2"><Label htmlFor="preview-evolution-messages">{t('divaPreview.messagesLabel')}</Label><Input id="preview-evolution-messages" type="number" min={1} value={messages} onChange={(event) => { setMessages(Math.max(1, Number(event.target.value) || 1)); notify(t('divaPreview.messagesUpdated')); }} /></div>
          </div>
        </div>
      </PreviewCard>
      <PreviewCard title={t('divaPreview.confirmTitle')} description={t('divaPreview.confirmDescription')}>
        <div className="space-y-2">{DIVA_EVOLUTION_ACTIONS.map((action) => <ToggleRow key={action.id} title={t(`divaPreview.actions.${action.id}`)} checked={confirmations[action.id]} onCheckedChange={(checked) => toggleConfirmation(action.id, checked)} />)}</div>
      </PreviewCard>
    </PreviewFrame>
  );
}

export function DivaSettingsPreview({ section }: { section: DivaPreviewSection }) {
  switch (section) {
    case 'general':
      return <GeneralPreview />;
    case 'self-evolution': return <SelfEvolutionPreview />;
  }
}
