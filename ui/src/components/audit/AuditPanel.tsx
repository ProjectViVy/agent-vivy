import { useState } from 'react';
import { FlaskConical } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ScrollArea } from '@/components/ui/scroll-area';
import { DIVA_AUDIT_EVENTS, type DivaAuditTab } from '@/components/settings/diva-preview-data';
import { useTranslation } from '@/i18n';

/** Audit panel embedded in the dashboard (中控台), moved from the top-bar drawer. */
export function AuditPanel() {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<DivaAuditTab>('structured');
  const [date, setDate] = useState('2026-08-24');
  const [feedback, setFeedback] = useState(t('audit.initialFeedback'));
  const events = DIVA_AUDIT_EVENTS[activeTab];

  return (
    <div className="flex h-full flex-col">
      <div className="space-y-3 border-b px-4 py-3">
        <div className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-xs text-amber-800 dark:text-amber-300">
          <FlaskConical className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <span>
            <strong>{t('audit.migrationNoticeTitle')}</strong>
            {' '}
            {t('audit.migrationNoticeBody')}
          </span>
        </div>
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div className="space-y-2">
            <Label htmlFor="audit-panel-date">{t('audit.date')}</Label>
            <Input
              id="audit-panel-date"
              type="date"
              value={date}
              onChange={(event) => {
                setDate(event.target.value);
                setFeedback(t('audit.dateUpdated'));
              }}
            />
          </div>
          <Button
            type="button"
            variant="outline"
            onClick={() => setFeedback(t('audit.refreshed', { date }))}
          >
            {t('audit.refreshPreview')}
          </Button>
        </div>
        <div className="flex flex-wrap gap-2" role="tablist" aria-label={t('audit.tablistLabel')}>
          {(['structured', 'gateway', 'gui'] as const).map((tab) => (
            <Button
              key={tab}
              type="button"
              size="sm"
              variant={activeTab === tab ? 'default' : 'outline'}
              role="tab"
              aria-selected={activeTab === tab}
              onClick={() => {
                setActiveTab(tab);
                setFeedback(t('audit.tabSwitched'));
              }}
            >
              {t(`audit.tabs.${tab}`)}
            </Button>
          ))}
        </div>
      </div>
      <ScrollArea className="min-h-0 flex-1 px-4 py-3">
        <div className="space-y-2">
          {events.map((event) => (
            <div
              key={`${event.at}-${event.source}-${event.message}`}
              className="grid gap-2 rounded-lg border p-3 text-sm sm:grid-cols-[5rem_5rem_1fr]"
            >
              <span className="font-mono text-xs text-muted-foreground">{event.at}</span>
              <span className={event.level === 'warn' ? 'text-amber-600' : 'text-emerald-600'}>{event.level}</span>
              <span>
                <strong className="font-medium">{event.source}</strong>
                {' '}
                ·
                {' '}
                {event.message}
              </span>
            </div>
          ))}
        </div>
        <p className="mt-4 text-xs text-muted-foreground" aria-live="polite">{feedback}</p>
      </ScrollArea>
    </div>
  );
}
