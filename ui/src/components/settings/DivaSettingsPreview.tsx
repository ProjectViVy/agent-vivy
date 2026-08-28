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
import { Badge } from '@/components/ui/badge';
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
  const [feedback, setFeedback] = useState('控件只改变当前页面的临时预览，不会写入运行配置。');
  return { feedback, notify: setFeedback };
}

function PreviewNotice() {
  return (
    <div className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-xs text-amber-800 dark:text-amber-300">
      <FlaskConical className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
      <span><strong>Agent-Diva 迁移预览</strong> · 当前内容使用假数据，仅用于展示界面效果，不会保存或影响 Vivy 运行时。</span>
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
  const [prefs, setPrefs] = useState<ChatPreviewPrefs>({
    cleanMode: false,
    autoExpandReasoning: true,
    autoExpandToolDetails: false,
    showRawMetaByDefault: false,
  });
  const [cacheCleared, setCacheCleared] = useState(false);
  // 压缩配置随「压缩」分区移除，并入通用分区（DivaSettingsPreview 的 GeneralPreview）。
  const [maxTokens, setMaxTokens] = useState(8192);
  const [thresholdPercent, setThresholdPercent] = useState(80);
  const [keepRecent, setKeepRecent] = useState(12);
  const [historyTokens, setHistoryTokens] = useState(6340);
  const pressure = Math.min(100, Math.round((historyTokens / Math.max(1, maxTokens)) * 100));
  const shouldCompact = pressure >= thresholdPercent;
  const { feedback, notify } = usePreviewFeedback();

  const updatePref = (key: keyof ChatPreviewPrefs, value: boolean) => {
    setPrefs((current) => ({ ...current, [key]: value }));
    notify('聊天显示预览已更新。');
  };

  return (
    <PreviewFrame
      icon={SlidersHorizontal}
      title="通用与关于"
      description="迁移聊天显示偏好、上下文压缩、缓存状态和项目归属信息。"
      feedback={cacheCleared ? '界面缓存清理已模拟完成，真实浏览器数据未被删除。' : feedback}
    >
      <PreviewCard title="聊天显示" description="这些开关只模拟 Agent-Diva 的消息展示偏好。">
        <div className="space-y-3">
          <ToggleRow
            title="简洁模式"
            description="隐藏推理、工具细节和原始元数据的展开内容。"
            checked={prefs.cleanMode}
            onCheckedChange={(checked) => updatePref('cleanMode', checked)}
          />
          <ToggleRow
            title="自动展开推理"
            checked={prefs.autoExpandReasoning}
            disabled={prefs.cleanMode}
            onCheckedChange={(checked) => updatePref('autoExpandReasoning', checked)}
          />
          <ToggleRow
            title="自动展开工具详情"
            checked={prefs.autoExpandToolDetails}
            disabled={prefs.cleanMode}
            onCheckedChange={(checked) => updatePref('autoExpandToolDetails', checked)}
          />
          <ToggleRow
            title="默认显示原始元数据"
            checked={prefs.showRawMetaByDefault}
            disabled={prefs.cleanMode}
            onCheckedChange={(checked) => updatePref('showRawMetaByDefault', checked)}
          />
        </div>
      </PreviewCard>

      <PreviewCard title="上下文压缩" description="预览历史消息预算、阈值和手动压缩入口，调整只影响本页预览，不写入运行配置。">
        <div className="flex items-center justify-between text-sm"><span>历史消息占用</span><span className="font-medium">{historyTokens.toLocaleString()} / {maxTokens.toLocaleString()} tokens</span></div>
        <div className="mt-3 h-2 overflow-hidden rounded-full bg-muted"><div className={`h-full rounded-full transition-all ${shouldCompact ? 'bg-amber-500' : 'bg-primary'}`} style={{ width: `${pressure}%` }} /></div>
        <div className="mt-3 flex flex-wrap gap-2 text-xs text-muted-foreground"><Badge variant={shouldCompact ? 'secondary' : 'outline'}>{pressure}% 压力</Badge><span>{shouldCompact ? '达到压缩阈值' : '暂不需要压缩'}</span></div>
        <div className="mt-4 grid gap-4 sm:grid-cols-3">
          <div className="space-y-2"><Label htmlFor="preview-compaction-max">最大 tokens</Label><Input id="preview-compaction-max" type="number" min={1} value={maxTokens} onChange={(event) => { setMaxTokens(Math.max(1, Number(event.target.value) || 1)); notify('最大 tokens 预览已更新。'); }} /></div>
          <div className="space-y-2"><Label htmlFor="preview-compaction-threshold">压缩阈值 (%)</Label><Input id="preview-compaction-threshold" type="number" min={10} max={100} value={thresholdPercent} onChange={(event) => { setThresholdPercent(Math.min(100, Math.max(10, Number(event.target.value) || 10))); notify('压缩阈值预览已更新。'); }} /></div>
          <div className="space-y-2"><Label htmlFor="preview-compaction-recent">保留最近消息</Label><Input id="preview-compaction-recent" type="number" min={1} value={keepRecent} onChange={(event) => { setKeepRecent(Math.max(1, Number(event.target.value) || 1)); notify('保留消息数预览已更新。'); }} /></div>
        </div>
        <div className="mt-4 flex flex-wrap items-center gap-3"><Button type="button" onClick={() => { setHistoryTokens(keepRecent * 240); notify('已模拟执行一次上下文压缩预览。'); }}>执行压缩预览</Button><Button type="button" variant="outline" onClick={() => { setMaxTokens(8192); setThresholdPercent(80); setKeepRecent(12); setHistoryTokens(6340); notify('压缩配置已恢复为预览默认值。'); }}>恢复预览默认值</Button></div>
      </PreviewCard>

      <PreviewCard title="缓存与运行状态" description="用静态状态展示 DIVA 通用设置中的运行摘要。">
        <div className="grid gap-3 sm:grid-cols-3">
          <div className="rounded-lg border p-3"><p className="text-xs text-muted-foreground">控制面</p><p className="mt-1 font-medium text-emerald-600">健康</p></div>
          <div className="rounded-lg border p-3"><p className="text-xs text-muted-foreground">Provider</p><p className="mt-1 font-medium">2 / 3 就绪</p></div>
          <div className="rounded-lg border p-3"><p className="text-xs text-muted-foreground">通道</p><p className="mt-1 font-medium">1 / 3 就绪</p></div>
        </div>
        <Button type="button" variant="outline" className="mt-4" onClick={() => { setCacheCleared(true); notify('界面缓存清理已模拟完成。'); }}>
          {cacheCleared ? '已模拟清理缓存' : '清理界面缓存（预览）'}
        </Button>
      </PreviewCard>

      <PreviewCard title="关于 Vivy" description="DIVA About 设置合并到通用页，避免重复的应用信息入口。">
        <dl className="grid gap-3 text-sm sm:grid-cols-3">
          <div><dt className="text-muted-foreground">许可证</dt><dd className="mt-1 font-medium">MIT</dd></div>
          <div><dt className="text-muted-foreground">维护方</dt><dd className="mt-1 font-medium">projectViVY</dd></div>
          <div><dt className="text-muted-foreground">界面来源</dt><dd className="mt-1 font-medium">Vivy 前端</dd></div>
        </dl>
      </PreviewCard>
    </PreviewFrame>
  );
}

function SelfEvolutionPreview() {
  const [enabled, setEnabled] = useState(true);
  const [frequency, setFrequency] = useState<EvolutionFrequency>('weekly');
  const [sessions, setSessions] = useState(5);
  const [messages, setMessages] = useState(100);
  const [confirmations, setConfirmations] = useState<Record<DivaEvolutionAction, boolean>>({ identity: true, relationship: true, commitment: true, sop: true, deprecation: true });
  const { feedback, notify } = usePreviewFeedback();

  const toggleConfirmation = (action: DivaEvolutionAction, checked: boolean) => {
    setConfirmations((current) => ({ ...current, [action]: checked }));
    notify(`${DIVA_EVOLUTION_ACTIONS.find((item) => item.id === action)?.label ?? '动作'}的确认策略已更新。`);
  };

  return (
    <PreviewFrame icon={Sparkles} title="自进化" description="预览 AutoDream 触发条件与人工确认策略。" feedback={feedback}>
      <PreviewCard title="自动整理" description="执行与合并能力尚未接入，所有数值均为假数据。">
        <div className="space-y-4">
          <ToggleRow title="启用自动整理" checked={enabled} onCheckedChange={(checked) => { setEnabled(checked); notify('自动整理预览已更新。'); }} />
          <div className="grid gap-4 sm:grid-cols-3">
            <div className="space-y-2"><Label htmlFor="preview-evolution-frequency">运行频率</Label><Select value={frequency} onValueChange={(value: EvolutionFrequency) => { setFrequency(value); notify('自动整理频率预览已更新。'); }}><SelectTrigger id="preview-evolution-frequency"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="daily">每天</SelectItem><SelectItem value="weekly">每周</SelectItem><SelectItem value="manual">手动</SelectItem></SelectContent></Select></div>
            <div className="space-y-2"><Label htmlFor="preview-evolution-sessions">会话阈值</Label><Input id="preview-evolution-sessions" type="number" min={1} value={sessions} onChange={(event) => { setSessions(Math.max(1, Number(event.target.value) || 1)); notify('会话阈值预览已更新。'); }} /></div>
            <div className="space-y-2"><Label htmlFor="preview-evolution-messages">消息阈值</Label><Input id="preview-evolution-messages" type="number" min={1} value={messages} onChange={(event) => { setMessages(Math.max(1, Number(event.target.value) || 1)); notify('消息阈值预览已更新。'); }} /></div>
          </div>
        </div>
      </PreviewCard>
      <PreviewCard title="人工确认策略" description="预览阶段不允许自动合并，所有变更都要求人工确认。">
        <div className="space-y-2">{DIVA_EVOLUTION_ACTIONS.map((action) => <ToggleRow key={action.id} title={action.label} checked={confirmations[action.id]} onCheckedChange={(checked) => toggleConfirmation(action.id, checked)} />)}</div>
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
