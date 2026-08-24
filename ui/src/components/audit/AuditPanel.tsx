import { useState } from 'react';
import { FlaskConical } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ScrollArea } from '@/components/ui/scroll-area';
import { DIVA_AUDIT_EVENTS, type DivaAuditTab } from '@/components/settings/diva-preview-data';

const TAB_LABELS: Record<DivaAuditTab, string> = {
  structured: '结构化事件',
  gateway: '网关日志',
  gui: '界面日志',
};

/** Audit panel embedded in the dashboard (中控台), moved from the top-bar drawer. */
export function AuditPanel() {
  const [activeTab, setActiveTab] = useState<DivaAuditTab>('structured');
  const [date, setDate] = useState('2026-08-24');
  const [feedback, setFeedback] = useState('控件只改变当前面板的临时预览，不会写入运行配置。');
  const events = DIVA_AUDIT_EVENTS[activeTab];

  return (
    <div className="flex h-full flex-col">
      <div className="space-y-3 border-b px-4 py-3">
        <div className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-xs text-amber-800 dark:text-amber-300">
          <FlaskConical className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <span>
            <strong>Agent-Diva 迁移预览</strong>
            {' '}
            · 日志为静态示例，不会保存或影响 Vivy 运行时。
          </span>
        </div>
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div className="space-y-2">
            <Label htmlFor="audit-panel-date">日期</Label>
            <Input
              id="audit-panel-date"
              type="date"
              value={date}
              onChange={(event) => {
                setDate(event.target.value);
                setFeedback('审计日期预览已更新。');
              }}
            />
          </div>
          <Button
            type="button"
            variant="outline"
            onClick={() => setFeedback(`已刷新 ${date} 的审计日志预览。`)}
          >
            刷新预览
          </Button>
        </div>
        <div className="flex flex-wrap gap-2" role="tablist" aria-label="审计日志类型">
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
                setFeedback('审计日志类型预览已切换。');
              }}
            >
              {TAB_LABELS[tab]}
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
