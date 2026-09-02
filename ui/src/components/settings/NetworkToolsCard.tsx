import { useEffect, useState } from 'react';
import { Check, Globe2 } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { settingsUpdateFrom } from '@/lib/api';
import { useVivyStore } from '@/lib/store';
import { useTranslation } from '@/i18n';

/**
 * 设置页的真实网络工具卡：展示 EINO 原生网络工具（network_search 各 provider
 * 的环境变量就绪状态），并允许选择首选 provider（保存到用户工作区
 * settings.yaml）。搜索服务密钥仍只做存在性提示（D-010）。http_request 的
 * 域名白名单与请求超时作为 settings.yaml http 覆盖层在此编辑。
 */
const NETWORK_PROVIDERS = ['bing', 'google', 'duckduckgo', 'searxng', 'wikipedia'] as const;

function parseHosts(value: string): string[] {
  return value.split(/[\n,]/).map((item) => item.trim()).filter(Boolean);
}

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

  const httpView = settings?.http;
  const [hostsText, setHostsText] = useState('');
  const [timeoutText, setTimeoutText] = useState('');
  const [httpSavedFlash, setHttpSavedFlash] = useState(false);

  // 进入分区时刷新，随后跟随 store 的最新 settings（含本次保存回显）。
  useEffect(() => {
    void load();
  }, [load]);
  useEffect(() => {
    if (view) setPreferred(view.provider);
  }, [view]);
  useEffect(() => {
    if (httpView) {
      setHostsText((httpView.allowed_hosts ?? []).join('\n'));
      setTimeoutText(String(httpView.timeout_seconds));
    }
  }, [httpView]);

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

  const nextHosts = parseHosts(hostsText);
  const nextTimeout = Number.parseInt(timeoutText, 10);
  const timeoutValid = Number.isInteger(nextTimeout) && nextTimeout >= 0 && nextTimeout <= 120;
  const httpDirty = !!httpView
    && (nextHosts.join('\n') !== (httpView.allowed_hosts ?? []).join('\n')
      || (timeoutValid ? nextTimeout : -1) !== httpView.timeout_seconds);

  const applyHttp = async () => {
    if (locked || !httpDirty || !timeoutValid) return;
    try {
      // settings/update 只改 http 覆盖层；显式空列表是拒绝全部的只读面。
      await save({
        ...settingsUpdateFrom(settings),
        http: { allowed_hosts: nextHosts, timeout_seconds: timeoutValid ? nextTimeout : 0 },
      });
      setHttpSavedFlash(true);
      window.setTimeout(() => setHttpSavedFlash(false), 2000);
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

        <div className="space-y-2 border-t pt-4">
          <div className="flex items-center gap-2">
            <p className="text-sm font-medium">{t('networkTools.httpSectionTitle')}</p>
            {httpView?.overlay_set ? <Badge variant="outline">{t('networkTools.httpOverlay')}</Badge> : null}
          </div>
          <div className="space-y-2">
            <Label htmlFor="http-allowed-hosts">{t('networkTools.httpHosts')}</Label>
            <Textarea
              id="http-allowed-hosts"
              rows={3}
              value={hostsText}
              disabled={locked}
              onChange={(event) => setHostsText(event.target.value)}
              placeholder={t('networkTools.httpHostsPlaceholder')}
            />
            <p className="text-xs text-muted-foreground">{t('networkTools.httpHostsHint')}</p>
          </div>
          <div className="max-w-48 space-y-2">
            <Label htmlFor="http-timeout">{t('networkTools.httpTimeout')}</Label>
            <Input
              id="http-timeout"
              type="number"
              min={0}
              max={120}
              value={timeoutText}
              disabled={locked}
              onChange={(event) => setTimeoutText(event.target.value)}
            />
            <p className="text-xs text-muted-foreground">
              {t('networkTools.httpTimeoutHint', { config: httpView?.config_timeout_seconds ?? 10 })}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <Button
              type="button"
              disabled={locked || !httpDirty || !timeoutValid}
              onClick={() => void applyHttp()}
            >
              {phase === 'processing' ? t('networkTools.saving') : t('networkTools.save')}
            </Button>
            {httpSavedFlash ? (
              <span className="inline-flex items-center gap-1.5 text-sm text-muted-foreground">
                <Check className="h-4 w-4 text-primary" aria-hidden="true" />
                {t('networkTools.saved')}
              </span>
            ) : null}
          </div>
        </div>

        {settings?.read_only ? <p className="text-xs text-muted-foreground">{t('networkTools.readOnly')}</p> : null}
      </CardContent>
    </Card>
  );
}