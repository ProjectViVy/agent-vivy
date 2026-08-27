import { useEffect, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { Activity, ArrowRight, Cpu, FlaskConical, GitBranch, Sparkles } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { getToolsConfig, updateToolsConfig } from '@/lib/demo-api';
import type { ToolsConfigShape } from '@/lib/types';
import { useVivyStore } from '@/lib/store';
import { DemoLoadError } from '@/components/demo/DemoBanner';
import { RunInspector } from '@/components/chat/RunInspector';
import { openWelcome } from '@/hooks/use-welcome';
import { useTranslation } from '@/i18n';
import { DIVA_ADDITIONAL_SECTIONS, type DivaAdditionalSection, type DivaPreviewSection } from './diva-preview-data';
import { DivaSettingsPreview } from './DivaSettingsPreview';
import { GenerationParamsCard } from './GenerationParamsCard';
import { ModelSettingsCard } from './ModelSettingsCard';
import { ThemePicker } from './ThemePicker';
import { LanguagePicker } from './LanguagePicker';

const SETTINGS_TAB_VALUES = ['general', 'model', 'tools', 'vivy', 'language', ...DIVA_ADDITIONAL_SECTIONS] as const;
export type SettingsTab = (typeof SETTINGS_TAB_VALUES)[number];

/** 路由 search 参数的白名单校验（?tab=…深链）。 */
export function isSettingsTab(value: unknown): value is SettingsTab {
  return typeof value === 'string' && (SETTINGS_TAB_VALUES as readonly string[]).includes(value);
}

const DIVA_TAB_LABELS: Record<DivaAdditionalSection, string> = {
  channels: '通道',
  network: '网络',
  'self-evolution': '自进化',
  sandbox: '沙箱',
};

function DemoNote() {
  return (
    <div className="mb-4 flex items-center gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-xs text-amber-800 dark:text-amber-300">
      <FlaskConical className="h-4 w-4" aria-hidden="true" />
      <strong>演示 / 本地模拟</strong>
      <span>修改只保存到 vivy.demo.* localStorage。</span>
    </div>
  );
}

export function SettingsView({ initialTab }: { initialTab?: SettingsTab }) {
  const connection = useVivyStore((state) => state.connection);
  const { t } = useTranslation();
  const [tools, setTools] = useState<ToolsConfigShape | null>(null);
  const [demoBusy, setDemoBusy] = useState<'tools' | null>(null);
  const [demoSaved, setDemoSaved] = useState<string | null>(null);
  const [demoError, setDemoError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<SettingsTab>(isSettingsTab(initialTab) ? initialTab : 'general');

  const loadTools = async () => {
    setDemoError(null);
    try {
      setTools(await getToolsConfig());
    } catch (cause) {
      setDemoError(cause instanceof Error ? cause.message : String(cause));
    }
  };

  useEffect(() => {
    if (activeTab === 'tools') void loadTools();
  }, [activeTab]);
  // 深链 ?tab=… 落地或欢迎向导完成跳转时切换到目标分区；非法值回落到「通用」。
  useEffect(() => { if (isSettingsTab(initialTab)) setActiveTab(initialTab); }, [initialTab]);

  const persistTools = async () => {
    setDemoBusy('tools');
    setDemoSaved(null);
    setDemoError(null);
    try {
      if (tools) setTools(await updateToolsConfig(tools));
      setDemoSaved('tools');
    } catch (cause) {
      setDemoError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setDemoBusy(null);
    }
  };

  return (
    <div className="h-full overflow-auto p-4 sm:p-6">
      <div className="mx-auto max-w-4xl">
        <h1 className="text-2xl font-bold">设置</h1>
        <p className="mt-1 text-sm text-muted-foreground">真实运行配置、Vivy 功能与 Agent-Diva 前端迁移预览分区展示。</p>
        <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as SettingsTab)} className="mt-6">
          <TabsList className="w-full justify-start overflow-x-auto">
            <TabsTrigger value="general">通用</TabsTrigger>
            <TabsTrigger value="model">模型</TabsTrigger>
            <TabsTrigger value="tools">工具</TabsTrigger>
            <TabsTrigger value="vivy">Vivy 功能</TabsTrigger>
            <TabsTrigger value="language">语言</TabsTrigger>
            {DIVA_ADDITIONAL_SECTIONS.map((section) => (
              <TabsTrigger key={section} value={section}>
                {DIVA_TAB_LABELS[section]}
                <span className="ml-1.5 rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium leading-none text-muted-foreground">预览</span>
              </TabsTrigger>
            ))}
          </TabsList>

          {demoError ? <div className="mt-4"><DemoLoadError message={demoError} onRetry={() => void loadTools()} /></div> : null}

          <TabsContent value="general" className="space-y-4">
            <Card>
              <CardHeader><CardTitle>应用信息</CardTitle><CardDescription>当前 Vivy 应用状态与演示内容范围。</CardDescription></CardHeader>
              <CardContent className="grid gap-4 text-sm sm:grid-cols-2">
                <div><p className="text-muted-foreground">应用</p><p className="mt-1 font-medium">Vivy</p></div>
                <div><p className="text-muted-foreground">版本</p><p className="mt-1 font-medium">1.0.0</p></div>
                <div><p className="text-muted-foreground">控制面连接</p><p className="mt-1 font-medium">{connection}</p></div>
                <div><p className="text-muted-foreground">演示内容</p><p className="mt-1 font-medium">仅保存在当前浏览器</p></div>
              </CardContent>
            </Card>
            <ThemePicker />
            <Card>
              <CardHeader>
                <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary"><Sparkles className="h-5 w-5" aria-hidden="true" /></div>
                <CardTitle>{t('welcome.rerunTitle')}</CardTitle>
                <CardDescription>{t('welcome.rerunDescription')}</CardDescription>
              </CardHeader>
              <CardContent><Button variant="outline" onClick={() => openWelcome()}>{t('welcome.rerunAction')}</Button></CardContent>
            </Card>
            <GenerationParamsCard />
            <DivaSettingsPreview section="general" />
          </TabsContent>

          <TabsContent value="model" className="space-y-4">
            <Card>
              <CardHeader>
                <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary"><Cpu className="h-5 w-5" aria-hidden="true" /></div>
                <CardTitle>{t('settings.modelConfigTitle')}</CardTitle>
                <CardDescription>{t('settings.modelConfigDescription')}</CardDescription>
              </CardHeader>
              <CardContent><ModelSettingsCard /></CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="tools">
            <DemoNote />
            <Card>
              <CardHeader><CardTitle>工具配置</CardTitle><CardDescription>本地演示沙箱和命令审批规则。</CardDescription></CardHeader>
              <CardContent>{tools ? <div className="space-y-5">
                <div className="flex items-center justify-between rounded-lg border p-3"><div><p className="font-medium">沙箱模式</p><p className="text-sm text-muted-foreground">演示工具调用隔离状态</p></div><Switch checked={tools.sandbox_enabled} disabled={demoBusy === 'tools'} onCheckedChange={(checked) => setTools({ ...tools, sandbox_enabled: checked })} aria-label="演示沙箱模式" /></div>
                <div className="space-y-2"><Label>命令规则</Label>{(tools.command_rules ?? []).map((rule, index) => <div key={rule.command} className="grid gap-3 rounded-lg border p-3 sm:grid-cols-[1fr_auto_auto]"><code>{rule.command}</code><label className="flex items-center gap-2 text-sm"><Switch checked={rule.enabled} disabled={demoBusy === 'tools'} onCheckedChange={(checked) => setTools({ ...tools, command_rules: (tools.command_rules ?? []).map((item, itemIndex) => itemIndex === index ? { ...item, enabled: checked } : item) })} aria-label={`${rule.command} 启用状态`} />启用</label><label className="flex items-center gap-2 text-sm"><Switch checked={rule.approval_required} disabled={demoBusy === 'tools'} onCheckedChange={(checked) => setTools({ ...tools, command_rules: (tools.command_rules ?? []).map((item, itemIndex) => itemIndex === index ? { ...item, approval_required: checked } : item) })} aria-label={`${rule.command} 需要审批`} />需要审批</label></div>)}</div>
                <Button type="button" disabled={demoBusy === 'tools'} onClick={() => void persistTools()}>{demoBusy === 'tools' ? '保存中…' : '保存工具演示'}</Button>{demoSaved === 'tools' ? <span className="ml-3 text-sm text-muted-foreground">已保存到本地</span> : null}
              </div> : <div className="h-56 animate-pulse rounded bg-muted" />}</CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="vivy" className="space-y-4">
            <Card><CardHeader><div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary"><GitBranch className="h-5 w-5" aria-hidden="true" /></div><CardTitle>生命周期</CardTitle><CardDescription>查看 Species、Generation、评测与晋升。</CardDescription></CardHeader><CardContent><Button asChild variant="outline"><Link to="/lifecycle">打开生命周期<ArrowRight className="ml-2 h-4 w-4" /></Link></Button></CardContent></Card>
            <Card><CardHeader><div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary"><Activity className="h-5 w-5" aria-hidden="true" /></div><CardTitle>Run Inspector</CardTitle><CardDescription>查看当前、后台与子 Run。</CardDescription></CardHeader><CardContent className="h-[min(36rem,calc(100dvh-12rem))] overflow-hidden p-0"><RunInspector /></CardContent></Card>
          </TabsContent>

          <TabsContent value="language" className="space-y-4">
            <LanguagePicker />
          </TabsContent>

          {DIVA_ADDITIONAL_SECTIONS.map((section) => (
            <TabsContent key={section} value={section} className="space-y-4">
              <DivaSettingsPreview section={section} />
            </TabsContent>
          ))}
        </Tabs>
      </div>
    </div>
  );
}
