import { useEffect, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { Activity, ArrowRight, Cpu, GitBranch, Sparkles } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { settingsUpdateFrom } from '@/lib/api';
import { useVivyStore } from '@/lib/store';
import { RunInspector } from '@/components/chat/RunInspector';
import { openWelcome } from '@/hooks/use-welcome';
import { useTranslation } from '@/i18n';
import { DIVA_ADDITIONAL_SECTIONS, type DivaAdditionalSection, type DivaPreviewSection } from './diva-preview-data';
import { DivaSettingsPreview } from './DivaSettingsPreview';
import { ChannelsSettings } from './ChannelsSettings';
import { CompactionSettingsCard } from './CompactionSettingsCard';
import { GenerationParamsCard } from './GenerationParamsCard';
import { ModelSettingsCard } from './ModelSettingsCard';
import { ThemePicker } from './ThemePicker';
import { LanguagePicker } from './LanguagePicker';
import { NetworkToolsCard } from './NetworkToolsCard';
import { SandboxSettingsCard } from './SandboxSettingsCard';
import { ToolsSettingsCard } from './ToolsSettingsCard';

const SETTINGS_TAB_VALUES = ['general', 'model', 'tools', 'vivy', 'language', 'channels', 'network', 'sandbox', ...DIVA_ADDITIONAL_SECTIONS] as const;
export type SettingsTab = (typeof SETTINGS_TAB_VALUES)[number];

/** 路由 search 参数的白名单校验（?tab=…深链）。 */
export function isSettingsTab(value: unknown): value is SettingsTab {
  return typeof value === 'string' && (SETTINGS_TAB_VALUES as readonly string[]).includes(value);
}

export function SettingsView({ initialTab }: { initialTab?: SettingsTab }) {
  const connection = useVivyStore((state) => state.connection);
  const settings = useVivyStore((state) => state.settings);
  const phase = useVivyStore((state) => state.settingsPhase);
  const error = useVivyStore((state) => state.settingsError);
  const save = useVivyStore((state) => state.saveSettings);
  const { t } = useTranslation();
  const [form, setForm] = useState({ provider: '', default_model: '', base_url: '', execute_max_timeout: '' });
  const [formError, setFormError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<SettingsTab>(isSettingsTab(initialTab) ? initialTab : 'general');

  useEffect(() => { if (isSettingsTab(initialTab)) setActiveTab(initialTab); }, [initialTab]);

  // 执行超时表单跟随 settings 载入（settings/update 是整文档替换）。
  useEffect(() => {
    if (settings) setForm({ provider: settings.provider, default_model: settings.default_model, base_url: settings.base_url, execute_max_timeout: settings.execute_max_timeout_seconds ? String(settings.execute_max_timeout_seconds) : '' });
  }, [settings]);

  const locked = settings?.read_only || phase === 'processing';

  // settings/update 会整份覆盖：保存时带上网络搜索偏好与执行超时，避免清掉
  // 其他分区的设置（api_key 由 api.ts 归一为空串=清除覆盖层，与该接口的
  // wholesale 语义一致）。
  const submitSettings = async () => {
    const raw = form.execute_max_timeout.trim();
    const timeoutSeconds = raw === '' ? 0 : Number(raw);
    if (!Number.isInteger(timeoutSeconds) || timeoutSeconds < 0 || timeoutSeconds > 600) {
      setFormError(t('settings.executeTimeoutInvalid'));
      return;
    }
    setFormError(null);
    await save({ ...settingsUpdateFrom(settings), provider: form.provider, default_model: form.default_model, base_url: form.base_url, execute_max_timeout_seconds: timeoutSeconds });
  };

  return (
    <div className="h-full overflow-auto p-4 sm:p-6">
      <div className="mx-auto max-w-4xl">
        <h1 className="text-2xl font-bold">{t('settings.title')}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t('settings.subtitle')}</p>
        <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as SettingsTab)} className="mt-6">
          <TabsList className="w-full justify-start overflow-x-auto">
            <TabsTrigger value="general">{t('settings.tabs.general')}</TabsTrigger>
            <TabsTrigger value="model">{t('settings.tabs.model')}</TabsTrigger>
            <TabsTrigger value="tools">{t('settings.tabs.tools')}</TabsTrigger>
            <TabsTrigger value="vivy">{t('settings.tabs.vivy')}</TabsTrigger>
<TabsTrigger value="language">{t('settings.tabs.language')}</TabsTrigger>
            <TabsTrigger value="channels">{t('settings.tabs.channels')}</TabsTrigger>
            <TabsTrigger value="network">{t('settings.tabs.network')}</TabsTrigger>
            <TabsTrigger value="sandbox">{t('settings.tabs.sandbox')}</TabsTrigger>
            {DIVA_ADDITIONAL_SECTIONS.map((section) => (
              <TabsTrigger key={section} value={section}>
                {t(`settings.tabs.${section === 'self-evolution' ? 'selfEvolution' : section}`)}
                <span className="ml-1.5 rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium leading-none text-muted-foreground">{t('divaPreview.previewBadge')}</span>
              </TabsTrigger>
            ))}
          </TabsList>

          <TabsContent value="general" className="space-y-4">
            <Card>
              <CardHeader><CardTitle>{t('settings.executeTimeoutTitle')}</CardTitle><CardDescription>{t('settings.executeTimeoutDescription')}</CardDescription></CardHeader>
              <CardContent>
                <form className="space-y-4" onSubmit={(event) => { event.preventDefault(); void submitSettings(); }}>
                  <div className="space-y-2">
                    <Label htmlFor="execute-max-timeout">{t('settings.executeTimeoutLabel')}</Label>
                    <Input id="execute-max-timeout" type="number" min={0} max={600} step={1} value={form.execute_max_timeout} onChange={(event) => setForm({ ...form, execute_max_timeout: event.target.value })} placeholder={String(settings?.config_execute_max_timeout_seconds ?? 30)} disabled={locked} />
                    <p className="text-xs text-muted-foreground">{t('settings.executeTimeoutHint')}</p>
                  </div>
                  {settings?.read_only ? <p className="rounded bg-amber-500/10 p-3 text-sm text-amber-700">{t('settings.readOnlyNotice')}</p> : <Button type="submit" disabled={phase === 'processing'}>{phase === 'processing' ? t('settings.saving') : t('settings.saveGeneral')}</Button>}
                  {formError ? <p className="rounded bg-destructive/10 p-3 text-sm text-destructive">{formError}</p> : null}
                  {error ? <p className="rounded bg-destructive/10 p-3 text-sm text-destructive">{error}</p> : null}
                </form>
              </CardContent>
            </Card>
            <Card>
              <CardHeader><CardTitle>{t('settings.appInfoTitle')}</CardTitle><CardDescription>{t('settings.appInfoDescription')}</CardDescription></CardHeader>
              <CardContent className="grid gap-4 text-sm sm:grid-cols-2">
                <div><p className="text-muted-foreground">{t('settings.app')}</p><p className="mt-1 font-medium">Vivy</p></div>
                <div><p className="text-muted-foreground">{t('settings.version')}</p><p className="mt-1 font-medium">1.0.0</p></div>
                <div><p className="text-muted-foreground">{t('settings.connection')}</p><p className="mt-1 font-medium">{connection}</p></div>
                <div><p className="text-muted-foreground">{t('settings.demoContent')}</p><p className="mt-1 font-medium">{t('settings.demoContentValue')}</p></div>
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
            <CompactionSettingsCard />
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

          <TabsContent value="tools" className="space-y-4">
            <Card>
              <CardHeader>
                <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary"><Sparkles className="h-5 w-5" aria-hidden="true" /></div>
                <CardTitle>{t('settings.toolsTitle')}</CardTitle>
                <CardDescription>{t('settings.toolsDescription')}</CardDescription>
              </CardHeader>
              <ToolsSettingsCard />
            </Card>
          </TabsContent>

          <TabsContent value="vivy" className="space-y-4">
            <Card><CardHeader><div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary"><GitBranch className="h-5 w-5" aria-hidden="true" /></div><CardTitle>{t('settings.lifecycleTitle')}</CardTitle><CardDescription>{t('settings.lifecycleDescription')}</CardDescription></CardHeader><CardContent><Button asChild variant="outline"><Link to="/lifecycle">{t('settings.openLifecycle')}<ArrowRight className="ml-2 h-4 w-4" /></Link></Button></CardContent></Card>
            <Card><CardHeader><div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary"><Activity className="h-5 w-5" aria-hidden="true" /></div><CardTitle>{t('settings.runInspectorTitle')}</CardTitle><CardDescription>{t('settings.runInspectorDescription')}</CardDescription></CardHeader><CardContent className="h-[min(36rem,calc(100dvh-12rem))] overflow-hidden p-0"><RunInspector /></CardContent></Card>
          </TabsContent>

          <TabsContent value="language" className="space-y-4">
            <LanguagePicker />
          </TabsContent>

<TabsContent value="channels" className="space-y-4">
            <ChannelsSettings />
          </TabsContent>

          <TabsContent value="network" className="space-y-4">
            <NetworkToolsCard />
          </TabsContent>

          <TabsContent value="sandbox" className="space-y-4">
            <SandboxSettingsCard />
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
