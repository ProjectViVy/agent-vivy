import { useEffect, useState } from 'react';
import { Check, ShieldCheck } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { settingsUpdateFrom, type PermissionPreset } from '@/lib/api';
import { useVivyStore } from '@/lib/store';
import { useTranslation } from '@/i18n';

const PRESETS: Exclude<PermissionPreset, 'custom'>[] = ['cautious', 'smart', 'trusted'];

function parseDomains(value: string): string[] {
  return value.split(/[\n,]/).map((item) => item.trim()).filter(Boolean);
}

export function SandboxSettingsCard() {
  const { t } = useTranslation();
  const settings = useVivyStore((state) => state.settings);
  const phase = useVivyStore((state) => state.settingsPhase);
  const error = useVivyStore((state) => state.settingsError);
  const load = useVivyStore((state) => state.loadSettings);
  const save = useVivyStore((state) => state.saveSettings);

  const view = settings?.sandbox;
  const [preset, setPreset] = useState<Exclude<PermissionPreset, 'custom'>>('smart');
  const [denyPrivate, setDenyPrivate] = useState(true);
  const [domainsText, setDomainsText] = useState('');
  const [savedFlash, setSavedFlash] = useState(false);

  useEffect(() => {
    void load();
  }, [load]);
  useEffect(() => {
    if (!view) return;
    if (view.default_preset !== 'custom') setPreset(view.default_preset);
    setDenyPrivate(view.deny_private_ips);
    setDomainsText((view.allowed_domains ?? []).join('\n'));
  }, [view]);

  const locked = settings?.read_only || phase === 'processing';
  const currentDomains = view?.allowed_domains ?? [];
  const nextDomains = parseDomains(domainsText);
  const dirty = !view
    || preset !== view.default_preset
    || denyPrivate !== view.deny_private_ips
    || nextDomains.join('\n') !== currentDomains.join('\n');

  const apply = async () => {
    if (locked || !dirty) return;
    try {
      await save({
        ...settingsUpdateFrom(settings),
        sandbox: {
          default_preset: preset,
          deny_private_ips: denyPrivate,
          allowed_domains: nextDomains,
        },
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
          <ShieldCheck className="h-5 w-5" aria-hidden="true" />
        </div>
        <CardTitle>{t('sandboxSettings.title')}</CardTitle>
        <CardDescription>{t('sandboxSettings.description')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {error ? <p className="text-sm text-destructive" role="alert">{error}</p> : null}

        <div className="space-y-2">
          <Label htmlFor="sandbox-default-preset">{t('sandboxSettings.defaultPreset')}</Label>
          <Select value={preset} disabled={locked} onValueChange={(value: Exclude<PermissionPreset, 'custom'>) => setPreset(value)}>
            <SelectTrigger id="sandbox-default-preset" className="w-64"><SelectValue /></SelectTrigger>
            <SelectContent>
              {PRESETS.map((value) => (
                <SelectItem key={value} value={value}>{t(`sandboxSettings.presets.${value}`)}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <p className="text-xs text-muted-foreground">{t(`sandboxSettings.presetHints.${preset}`)}</p>
        </div>

        <div className="flex items-center justify-between rounded-lg border p-3">
          <div>
            <p className="font-medium">{t('sandboxSettings.denyPrivate')}</p>
            <p className="text-sm text-muted-foreground">{t('sandboxSettings.denyPrivateHint')}</p>
          </div>
          <Switch checked={denyPrivate} disabled={locked} onCheckedChange={setDenyPrivate} aria-label={t('sandboxSettings.denyPrivate')} />
        </div>

        <div className="space-y-2">
          <Label htmlFor="sandbox-allowed-domains">{t('sandboxSettings.allowedDomains')}</Label>
          <Textarea
            id="sandbox-allowed-domains"
            rows={3}
            value={domainsText}
            disabled={locked}
            onChange={(event) => setDomainsText(event.target.value)}
            placeholder={t('sandboxSettings.allowedDomainsPlaceholder')}
          />
          <p className="text-xs text-muted-foreground">{t('sandboxSettings.allowedDomainsHint')}</p>
        </div>

        <div className="space-y-2">
          <p className="text-sm font-medium">{t('sandboxSettings.workspaceRoot')}</p>
          <p className="break-all rounded-lg border bg-muted/40 px-3 py-2 font-mono text-xs">{view?.workspace_root || t('sandboxSettings.workspaceUnset')}</p>
        </div>

        <div className="space-y-2">
          <p className="text-sm font-medium">{t('sandboxSettings.commands')}</p>
          <div className="flex flex-wrap gap-2">
            {(view?.execute_allowed_commands ?? []).length
              ? (view?.execute_allowed_commands ?? []).map((command) => <Badge key={command} variant="outline">{command}</Badge>)
              : <p className="text-xs text-muted-foreground">{t('sandboxSettings.commandsEmpty')}</p>}
          </div>
          <p className="text-xs text-muted-foreground">{t('sandboxSettings.commandsHint')}</p>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <Button type="button" disabled={locked || !dirty} onClick={() => void apply()}>
            {phase === 'processing' ? t('sandboxSettings.saving') : t('sandboxSettings.save')}
          </Button>
          {savedFlash ? (
            <span className="inline-flex items-center gap-1.5 text-sm text-muted-foreground">
              <Check className="h-4 w-4 text-primary" aria-hidden="true" />
              {t('sandboxSettings.saved')}
            </span>
          ) : null}
        </div>
        {settings?.read_only ? <p className="text-xs text-muted-foreground">{t('sandboxSettings.readOnly')}</p> : null}
      </CardContent>
    </Card>
  );
}
