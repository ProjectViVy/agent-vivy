import { Bot } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { FailureNotice } from '@/components/feedback/FailureNotice';
import { useTranslation } from '@/i18n';
import type { ProviderCatalogRow } from '@/components/settings/provider-catalog';
import type { WelcomeModelDraft, WelcomeVendorGroup } from '../welcome-model';

export type WizardStepModelProps = {
  form: WelcomeModelDraft;
  rows: ProviderCatalogRow[];
  vendors: WelcomeVendorGroup[];
  selectedRow: ProviderCatalogRow | null;
  vendorEndpoints: ProviderCatalogRow[];
  keyConfigured: boolean;
  apiKey: string;
  envKeyConfirmed: boolean;
  saving: boolean;
  readOnly: boolean;
  frozen: boolean;
  error: string | null;
  catalogLoading: boolean;
  catalogFailed: boolean;
  providersError: string | null;
  onChooseVendor: (vendor: string) => void;
  onChooseProtocol: (adapter: string) => void;
  onChooseModel: (model: string) => void;
  onApiKeyChange: (value: string) => void;
  onEnvKeyConfirmedChange: (checked: boolean) => void;
  onSubmit: () => void;
  onRetryCatalog: () => void;
};

export function WizardStepModel(props: WizardStepModelProps) {
  const { t } = useTranslation();
  const {
    form, rows, vendors, selectedRow, vendorEndpoints, keyConfigured,
    apiKey, envKeyConfirmed, saving, readOnly, frozen, error,
    catalogLoading, catalogFailed, providersError,
    onChooseVendor, onChooseProtocol, onChooseModel,
    onApiKeyChange, onEnvKeyConfirmedChange, onSubmit, onRetryCatalog,
  } = props;
  const locked = saving || readOnly;

  return (
    <form
      className="space-y-5"
      onSubmit={(event) => { event.preventDefault(); if (!saving) onSubmit(); }}
    >
      <div className="flex gap-4">
        <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-brand-soft text-brand-ink">
          <Bot className="h-6 w-6" aria-hidden="true" />
        </div>
        <div className="min-w-0">
          <h3 className="mb-1 text-title font-semibold">{t('welcome.modelTitle')}</h3>
          <p className="text-meta leading-relaxed text-fg-muted">{t('welcome.modelBody')}</p>
        </div>
      </div>
      {catalogLoading ? (
        <p
          data-testid="welcome-catalog-loading"
          aria-busy="true"
          className="rounded-lg bg-sunken p-3 text-meta leading-relaxed text-fg-muted"
        >
          {t('welcome.catalogLoading')}
        </p>
      ) : catalogFailed ? (
        <div className="space-y-2 rounded-lg bg-danger-soft p-3" role="alert">
          <p className="text-ui text-danger">
            {t('welcome.catalogFailed', { error: providersError ?? '' })}
          </p>
          <Button type="button" variant="outline" size="sm" onClick={onRetryCatalog}>
            {t('welcome.retry')}
          </Button>
        </div>
      ) : !rows.length ? (
        <p className="rounded-lg bg-sunken p-3 text-meta leading-relaxed text-fg-muted">
          {t('welcome.catalogEmpty')}
        </p>
      ) : (
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="welcome-provider">{t('welcome.provider')}</Label>
            <Select value={form.vendor} onValueChange={onChooseVendor} disabled={locked}>
              <SelectTrigger id="welcome-provider" data-testid="welcome-provider">
                <SelectValue placeholder={t('welcome.select')} />
              </SelectTrigger>
              <SelectContent className="z-[250]">
                {vendors.map((vendor) => (
                  <SelectItem key={vendor.vendor} value={vendor.vendor}>
                    {vendor.displayName}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="welcome-protocol">{t('welcome.protocol')}</Label>
            <Select value={form.adapter} onValueChange={onChooseProtocol} disabled={locked || !form.vendor}>
              <SelectTrigger id="welcome-protocol" data-testid="welcome-protocol">
                <SelectValue placeholder={t('welcome.select')} />
              </SelectTrigger>
              <SelectContent className="z-[250]">
                {vendorEndpoints.map((endpoint) => (
                  <SelectItem key={endpoint.adapter} value={endpoint.adapter}>
                    {endpoint.adapter}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="welcome-model">{t('welcome.model')}</Label>
            <Select value={form.model} onValueChange={onChooseModel} disabled={locked || !selectedRow}>
              <SelectTrigger id="welcome-model" data-testid="welcome-model">
                <SelectValue placeholder={t('welcome.select')} />
              </SelectTrigger>
              <SelectContent className="z-[250]">
                {(selectedRow?.models ?? []).map((model) => (
                  <SelectItem key={model} value={model}>
                    {model}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {selectedRow ? (
            <p className="text-meta text-fg-muted" data-testid="welcome-endpoint">
              {t('welcome.endpoint')}: <span className="break-all font-mono">{selectedRow.baseUrl}</span>
            </p>
          ) : null}

          <div className="space-y-2">
            <Label htmlFor="welcome-api-key">{t('welcome.apiKey')}</Label>
            <Input
              id="welcome-api-key"
              data-testid="welcome-api-key"
              type="password"
              autoComplete="off"
              spellCheck={false}
              value={apiKey}
              disabled={locked}
              placeholder={
                keyConfigured
                  ? t('welcome.apiKeyConfiguredPlaceholder')
                  : t('welcome.apiKeyPlaceholder')
              }
              onChange={(event) => onApiKeyChange(event.target.value)}
            />
            <p className="text-2xs leading-relaxed text-fg-muted">
              {keyConfigured ? t('welcome.apiKeyConfigured') : t('welcome.apiKeyHint')}
            </p>
          </div>

          {!readOnly && !keyConfigured && !apiKey.trim() ? (
            <div className="flex items-start gap-2 rounded-lg bg-warn-soft p-3">
              <Checkbox
                id="welcome-env-key"
                data-testid="welcome-env-key"
                checked={envKeyConfirmed}
                disabled={saving}
                onCheckedChange={(checked) => onEnvKeyConfirmedChange(checked === true)}
              />
              <div className="min-w-0 space-y-1">
                <Label htmlFor="welcome-env-key" className="cursor-pointer text-meta font-medium">
                  {t('welcome.envKey')}
                </Label>
                <p className="text-2xs leading-relaxed text-fg-muted">
                  {t('welcome.envKeyHint')}
                </p>
              </div>
            </div>
          ) : null}
        </div>
      )}
      {readOnly ? (
        <p className="rounded-lg bg-warn-soft p-3 text-meta leading-relaxed text-warn">
          {frozen ? t('welcome.frozenNotice') : t('welcome.readOnlyNotice')}
        </p>
      ) : null}
      {error ? <FailureNotice variant="compact" title={t('errors.genericTitle')} error={error} /> : null}
    </form>
  );
}
