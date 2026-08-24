import { useEffect, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { Activity, ArrowRight, FlaskConical, GitBranch } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import { getConfig, getPersonaProfile, getToolsConfig, updateConfig, updatePersonaProfile, updateToolsConfig } from '@/lib/demo-api';
import type { PersonaProfile, RuntimeConfig, ToolsConfigShape } from '@/lib/types';
import { useVivyStore } from '@/lib/store';
import { DemoLoadError } from '@/components/demo/DemoBanner';
import { RunInspector } from '@/components/chat/RunInspector';
import { DIVA_ADDITIONAL_SECTIONS, type DivaAdditionalSection, type DivaPreviewSection } from './diva-preview-data';
import { DivaSettingsPreview } from './DivaSettingsPreview';
import { ThemePicker } from './ThemePicker';

type SettingsTab = 'general' | 'model' | 'persona' | 'tools' | 'vivy' | DivaPreviewSection;

const DIVA_TAB_LABELS: Record<DivaAdditionalSection, string> = {
  channels: '通道',
  network: '网络',
  language: '语言',
  compaction: '压缩',
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

export function SettingsView() {
  const settings = useVivyStore((state) => state.settings);
  const phase = useVivyStore((state) => state.settingsPhase);
  const error = useVivyStore((state) => state.settingsError);
  const connection = useVivyStore((state) => state.connection);
  const load = useVivyStore((state) => state.loadSettings);
  const save = useVivyStore((state) => state.saveSettings);
  const [form, setForm] = useState({ provider: '', default_model: '', base_url: '' });
  const [demoConfig, setDemoConfig] = useState<RuntimeConfig | null>(null);
  const [persona, setPersona] = useState<PersonaProfile | null>(null);
  const [tools, setTools] = useState<ToolsConfigShape | null>(null);
  const [demoBusy, setDemoBusy] = useState<'model' | 'persona' | 'tools' | null>(null);
  const [demoSaved, setDemoSaved] = useState<string | null>(null);
  const [demoError, setDemoError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<SettingsTab>('general');

  const loadDemos = async () => {
    setDemoError(null);
    const results = await Promise.allSettled([getConfig(), getPersonaProfile(), getToolsConfig()]);
    if (results[0].status === 'fulfilled') setDemoConfig(results[0].value);
    if (results[1].status === 'fulfilled') setPersona(results[1].value);
    if (results[2].status === 'fulfilled') setTools(results[2].value);
    const failures = results.filter((result): result is PromiseRejectedResult => result.status === 'rejected');
    if (failures.length) setDemoError(failures.map((result) => result.reason instanceof Error ? result.reason.message : String(result.reason)).join('；'));
  };

  useEffect(() => { void load(); }, [load]);
  useEffect(() => {
    if (activeTab === 'model' || activeTab === 'persona' || activeTab === 'tools') void loadDemos();
  }, [activeTab]);
  useEffect(() => {
    if (settings) setForm({ provider: settings.provider, default_model: settings.default_model, base_url: settings.base_url });
  }, [settings]);

  const locked = settings?.read_only || phase === 'processing';

  const persistDemo = async (kind: 'model' | 'persona' | 'tools') => {
    setDemoBusy(kind);
    setDemoSaved(null);
    setDemoError(null);
    try {
      if (kind === 'model' && demoConfig) setDemoConfig(await updateConfig(demoConfig));
      if (kind === 'persona' && persona) setPersona(await updatePersonaProfile(persona));
      if (kind === 'tools' && tools) setTools(await updateToolsConfig(tools));
      setDemoSaved(kind);
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
            <TabsTrigger value="persona">人格</TabsTrigger>
            <TabsTrigger value="tools">工具</TabsTrigger>
            <TabsTrigger value="vivy">Vivy 功能</TabsTrigger>
            {DIVA_ADDITIONAL_SECTIONS.map((section) => <TabsTrigger key={section} value={section}>{DIVA_TAB_LABELS[section]}</TabsTrigger>)}
          </TabsList>

          {demoError ? <div className="mt-4"><DemoLoadError message={demoError} onRetry={() => void loadDemos()} /></div> : null}

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
            <DemoNote />
            <DivaSettingsPreview section="general" />
          </TabsContent>

          <TabsContent value="model" className="space-y-4">
            <Card>
              <CardHeader><CardTitle>Vivy 模型配置</CardTitle><CardDescription>真实设置。密钥只由运行环境管理；保存后在下次启动时生效。</CardDescription></CardHeader>
              <CardContent>
                {phase === 'loading' && !settings ? <div className="space-y-3"><div className="h-10 animate-pulse rounded bg-muted" /><div className="h-10 animate-pulse rounded bg-muted" /><div className="h-10 animate-pulse rounded bg-muted" /></div> : (
                  <form className="space-y-4" onSubmit={async (event) => { event.preventDefault(); await save(form); }}>
                    <div className="space-y-2"><Label htmlFor="provider">Provider</Label><Input id="provider" value={form.provider} onChange={(event) => setForm({ ...form, provider: event.target.value })} placeholder={settings?.config_provider || '配置默认值'} disabled={locked} /></div>
                    <div className="space-y-2"><Label htmlFor="model">默认模型</Label><Input id="model" value={form.default_model} onChange={(event) => setForm({ ...form, default_model: event.target.value })} placeholder={settings?.config_model || 'Provider 默认值'} disabled={locked} /></div>
                    <div className="space-y-2"><Label htmlFor="base-url">Base URL</Label><Input id="base-url" type="url" value={form.base_url} onChange={(event) => setForm({ ...form, base_url: event.target.value })} placeholder="https://api.example.com/v1" disabled={locked} /></div>
                    {settings?.read_only ? <p className="rounded bg-amber-500/10 p-3 text-sm text-amber-700">此部署的设置为只读，请通过运行配置修改。</p> : <Button type="submit" disabled={phase === 'processing'}>{phase === 'processing' ? '保存中…' : '保存真实设置'}</Button>}
                    {error ? <p className="rounded bg-destructive/10 p-3 text-sm text-destructive">{error}</p> : null}
                  </form>
                )}
              </CardContent>
            </Card>
            <Card>
              <CardHeader><CardTitle>生成参数</CardTitle><CardDescription>恢复的示例参数，不会传给真实 Provider。</CardDescription></CardHeader>
              <CardContent>
                <DemoNote />
                {demoConfig ? <div className="grid gap-4 sm:grid-cols-2">
                  <div className="space-y-2"><Label htmlFor="temperature">Temperature</Label><Input id="temperature" type="number" min="0" max="2" step="0.1" value={demoConfig.temperature} disabled={demoBusy === 'model'} onChange={(event) => setDemoConfig({ ...demoConfig, temperature: Number(event.target.value) })} /></div>
                  <div className="space-y-2"><Label htmlFor="max-tokens">Max Tokens</Label><Input id="max-tokens" type="number" min="1" value={demoConfig.max_tokens} disabled={demoBusy === 'model'} onChange={(event) => setDemoConfig({ ...demoConfig, max_tokens: Number(event.target.value) })} /></div>
                  <div className="sm:col-span-2"><Button type="button" variant="outline" disabled={demoBusy === 'model'} onClick={() => void persistDemo('model')}>{demoBusy === 'model' ? '保存中…' : '保存演示参数'}</Button>{demoSaved === 'model' ? <span className="ml-3 text-sm text-muted-foreground">已保存到本地</span> : null}</div>
                </div> : <div className="h-24 animate-pulse rounded bg-muted" />}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="persona">
            <DemoNote />
            <Card>
              <CardHeader><CardTitle>人格配置</CardTitle><CardDescription>助手名称和系统提示词示例。</CardDescription></CardHeader>
              <CardContent>{persona ? <div className="space-y-4">
                <div className="space-y-2"><Label htmlFor="persona-name">名称</Label><Input id="persona-name" value={persona.name} disabled={demoBusy === 'persona'} onChange={(event) => setPersona({ ...persona, name: event.target.value })} /></div>
                <div className="space-y-2"><Label htmlFor="system-prompt">系统提示词</Label><Textarea id="system-prompt" rows={8} value={persona.system_prompt ?? ''} disabled={demoBusy === 'persona'} onChange={(event) => setPersona({ ...persona, system_prompt: event.target.value })} /></div>
                <Button type="button" disabled={demoBusy === 'persona'} onClick={() => void persistDemo('persona')}>{demoBusy === 'persona' ? '保存中…' : '保存人格演示'}</Button>{demoSaved === 'persona' ? <span className="ml-3 text-sm text-muted-foreground">已保存到本地</span> : null}
              </div> : <div className="h-56 animate-pulse rounded bg-muted" />}</CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="tools">
            <DemoNote />
            <Card>
              <CardHeader><CardTitle>工具配置</CardTitle><CardDescription>本地演示沙箱和命令审批规则。</CardDescription></CardHeader>
              <CardContent>{tools ? <div className="space-y-5">
                <div className="flex items-center justify-between rounded-lg border p-3"><div><p className="font-medium">沙箱模式</p><p className="text-sm text-muted-foreground">演示工具调用隔离状态</p></div><Switch checked={tools.sandbox_enabled} disabled={demoBusy === 'tools'} onCheckedChange={(checked) => setTools({ ...tools, sandbox_enabled: checked })} aria-label="演示沙箱模式" /></div>
                <div className="space-y-2"><Label>命令规则</Label>{(tools.command_rules ?? []).map((rule, index) => <div key={rule.command} className="grid gap-3 rounded-lg border p-3 sm:grid-cols-[1fr_auto_auto]"><code>{rule.command}</code><label className="flex items-center gap-2 text-sm"><Switch checked={rule.enabled} disabled={demoBusy === 'tools'} onCheckedChange={(checked) => setTools({ ...tools, command_rules: (tools.command_rules ?? []).map((item, itemIndex) => itemIndex === index ? { ...item, enabled: checked } : item) })} aria-label={`${rule.command} 启用状态`} />启用</label><label className="flex items-center gap-2 text-sm"><Switch checked={rule.approval_required} disabled={demoBusy === 'tools'} onCheckedChange={(checked) => setTools({ ...tools, command_rules: (tools.command_rules ?? []).map((item, itemIndex) => itemIndex === index ? { ...item, approval_required: checked } : item) })} aria-label={`${rule.command} 需要审批`} />需要审批</label></div>)}</div>
                <Button type="button" disabled={demoBusy === 'tools'} onClick={() => void persistDemo('tools')}>{demoBusy === 'tools' ? '保存中…' : '保存工具演示'}</Button>{demoSaved === 'tools' ? <span className="ml-3 text-sm text-muted-foreground">已保存到本地</span> : null}
              </div> : <div className="h-56 animate-pulse rounded bg-muted" />}</CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="vivy" className="space-y-4">
            <Card><CardHeader><div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary"><GitBranch className="h-5 w-5" aria-hidden="true" /></div><CardTitle>生命周期</CardTitle><CardDescription>查看 Species、Generation、评测与晋升。</CardDescription></CardHeader><CardContent><Button asChild variant="outline"><Link to="/lifecycle">打开生命周期<ArrowRight className="ml-2 h-4 w-4" /></Link></Button></CardContent></Card>
            <Card><CardHeader><div className="mb-2 flex items-center gap-2"><div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary"><Activity className="h-5 w-5" aria-hidden="true" /></div><div><CardTitle>Run Inspector</CardTitle><CardDescription>查看当前、后台与子 Run。</CardDescription></div></div></CardHeader><CardContent className="h-[min(36rem,calc(100dvh-12rem))] overflow-hidden p-0"><RunInspector /></CardContent></Card>
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
