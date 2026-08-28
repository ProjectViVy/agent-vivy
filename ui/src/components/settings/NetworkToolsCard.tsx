import { useEffect, useState } from 'react';
import { Check, Globe2 } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { settingsUpdateFrom } from '@/lib/api';
import { useVivyStore } from '@/lib/store';
import { useTranslation } from '@/i18n';

/**
 * 设置页的真实网络工具卡：展示 EINO 原生网络工具（network_search 各 provider
 * 的环境变量就绪状态），并允许选择首选 provider（保存到用户工作区
 * settings.yaml）。搜索服务密钥仍只做存在性提示（D-010）。
 */
const NETWORK_PROVIDERS = ['bing', 'google', 'duckduckgo', 'searxng', 'wikipedia'] as const;

export function NetworkToolsCard() {
  const { t } = useTranslation();
  const settings = useVivyStore((state) => state.settings);
  const phase = useVivyStore((state) => state.settingsPhase);
  const error = useVivyStore((state) => state.settingsError);
  const load = useVivyStore((state) => state.loadSettings);
  const save = useVivyStore((state) => state.saveSettings);

  const view = settings?.network_search;
  const roster = view?.providers ?? [];
  const [preferred, setPreferred] = useState<string>('');
  const [savedFlash, setSavedFlash] = useState(false);

  // 进入分区时刷新，随后跟随 store 的最新 settings（含本次保存回显）。
  useEffect(() => {
    void load();
  }, [load]);
  useEffect(() => {
    if (view) setPreferred(view.provider);
  }, [view]);

  const locked = settings?.read_only || phase === 'processing';

  const applyPreferred = async (provider: string) => {
    if (locked) return;
    try {
      // settings/update 只改 active 选择与 network_search 偏好；密钥覆盖层由
      // 后端按注册表解析（settings/update 不发送、不回传密钥）。
      await save({
        ...settingsUpdateFrom(settings),
        network_search: { provider },
      });
      setSavedFlash(true);
      window.setTimeout(() => setSavedFlash(false), 2000);
    } catch {
      // settingsError 已由 store 记录并渲染。
    }
  };

  return (
    <Card>
      <CardHeader>
        <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <Globe2 className="h-5 w-5" aria-hidden="true" />
        </div>
        <CardTitle>{t('networkTools.title')}</CardTitle>
        <CardDescription>{t('networkTools.description')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {error ? <p className="text-sm text-destructive" role="alert">{error}</p> : null}

        <div className="space-y-2">
          <Label htmlFor="network-tools-provider">{t('networkTools.preferredProvider')}</Label>
          <div className="flex flex-wrap items-center gap-3">
            <Select value={preferred} disabled={locked} onValueChange={(value) => setPreferred(value)}>
              <SelectTrigger id="network-tools-provider" className="w-64"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="">{t('networkTools.auto')}</SelectItem>
                {NETWORK_PROVIDERS.map((name) => (
                  <SelectItem key={name} value={name}>{t(`networkTools.providerNames.${name}`)}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button type="button" disabled={locked || preferred === (view?.provider ?? '')} onClick={() => void applyPreferred(preferred)}>
              {phase === 'processing' ? t('networkTools.saving') : t('networkTools.save')}
            </Button>
            {savedFlash ? (
              <span className="inline-flex items-center gap-1.5 text-sm text-muted-foreground">
                <Check className="h-4 w-4 text-primary" aria-hidden="true" />
                {t('networkTools.saved')}
              </span>
            ) : null}
          </div>
          <p className="text-xs text-muted-foreground">{t('networkTools.providerHint')}</p>
        </div>

        <div className="space-y-2">
          <p className="text-sm font-medium">{t('networkTools.rosterTitle')}</p>
          <div className="rounded-lg border">
            {roster.map((provider, index) => (
              <div key={provider.name} className={`flex flex-wrap items-center justify-between gap-2 p-3 ${index > 0 ? 'border-t' : ''}`}>
                <div className="min-w-0">
                  <p className="font-medium">{t(`networkTools.providerNames.${provider.name}`)}</p>
                  <p className="mt-0.5 text-xs text-muted-foreground">
                    {provider.keyless
                      ? t('networkTools.keyless')
                      : provider.env_key
                        ? t('networkTools.needsEnv', { env: provider.env_key })
                        : t('networkTools.noEnv')}
                  </p>
                </div>
                <Badge variant={provider.configured ? 'default' : 'secondary'}>
                  {provider.configured ? t('networkTools.configured') : t('networkTools.pending')}
                </Badge>
              </div>
            ))}
          </div>
          <p className="text-xs text-muted-foreground">{t('networkTools.secretNote')}</p>
        </div>

        {settings?.read_only ? <p className="text-xs text-muted-foreground">{t('networkTools.readOnly')}</p> : null}
      </CardContent>
    </Card>
  );
}