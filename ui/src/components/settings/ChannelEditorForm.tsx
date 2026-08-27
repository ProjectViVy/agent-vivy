import { useState } from 'react';
import { Eye, EyeOff } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import {
  CHANNEL_CREDENTIAL_FIELDS,
  coerceChannelFieldValue,
  fieldsByGroup,
  joinIdList,
  splitIdList,
  type WizardFormField,
} from './channel-schema';
import { useTranslation } from '@/i18n';

/**
 * 通道凭据内联编辑表单（移植自 Agent-Diva ChannelEditorForm.vue）。
 * 受控组件：值经 `onFieldChange` / `onExtraKeyChange` 上抛，父级持有配置副本。
 */

type ChannelEditorFormProps = {
  platform: string;
  config: Record<string, unknown>;
  onFieldChange: (field: WizardFormField, value: unknown) => void;
  /** 未知字段（schema 外）JSON 编辑回调；缺省时隐藏该区块。 */
  onExtraKeyChange?: (key: string, value: unknown) => void;
};

type RenderLabels = {
  showSecret: string;
  hideSecret: string;
};

function fieldValue(field: WizardFormField, config: Record<string, unknown>): unknown {
  return config[field.key] ?? coerceChannelFieldValue(field, undefined);
}

function BooleanSwitchRow({
  field,
  checked,
  onCheckedChange,
}: {
  field: WizardFormField;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
}) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-lg border p-3">
      <span className="text-sm font-medium">{field.label}</span>
      <Switch checked={checked} onCheckedChange={onCheckedChange} aria-label={field.label} />
    </div>
  );
}

function renderFieldControl(
  field: WizardFormField,
  value: unknown,
  revealed: boolean,
  labels: RenderLabels,
  onToggleReveal: () => void,
  onChange: (next: unknown) => void,
) {
  switch (field.type) {
    case 'select':
      return (
        <Select value={String(value ?? '')} onValueChange={onChange}>
          <SelectTrigger aria-label={field.label}>
            <SelectValue placeholder={field.placeholder} />
          </SelectTrigger>
          <SelectContent>
            {(field.options ?? []).map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      );
    case 'textarea':
      return (
        <Textarea
          rows={3}
          value={String(value ?? '')}
          placeholder={field.placeholder}
          onChange={(event) => onChange(event.target.value)}
        />
      );
    case 'string-list':
      return (
        <Textarea
          rows={3}
          value={joinIdList(value)}
          placeholder={field.placeholder}
          onChange={(event) => onChange(splitIdList(event.target.value))}
        />
      );
    case 'boolean':
      return (
        <BooleanSwitchRow
          field={field}
          checked={Boolean(value)}
          onCheckedChange={onChange}
        />
      );
    case 'password':
      return (
        <div className="relative">
          <Input
            type={revealed ? 'text' : 'password'}
            value={String(value ?? '')}
            placeholder={field.placeholder}
            autoComplete="off"
            onChange={(event) => onChange(event.target.value)}
            className="pr-10"
          />
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="absolute right-0 top-0 h-full px-3 text-muted-foreground hover:text-foreground"
            onClick={onToggleReveal}
            aria-label={revealed ? labels.hideSecret : labels.showSecret}
            title={revealed ? labels.hideSecret : labels.showSecret}
          >
            {revealed ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
          </Button>
        </div>
      );
    default:
      return (
        <Input
          type={field.type === 'number' ? 'number' : 'text'}
          value={value === null || value === undefined ? '' : (value as string | number)}
          placeholder={field.placeholder}
          onChange={(event) => {
            const raw = event.target.value;
            onChange(
              field.type === 'number'
                ? raw === ''
                  ? null
                  : Number(raw)
                : raw,
            );
          }}
        />
      );
  }
}

export function ChannelEditorForm({
  platform,
  config,
  onFieldChange,
  onExtraKeyChange,
}: ChannelEditorFormProps) {
  const { t } = useTranslation();
  const [revealedSecrets, setRevealedSecrets] = useState<Set<string>>(() => new Set());

  const basicFields = fieldsByGroup(platform, 'basic');
  const advancedFields = fieldsByGroup(platform, 'advanced');
  const hasSchema = (CHANNEL_CREDENTIAL_FIELDS[platform] || []).length > 0;

  const knownKeys = new Set([
    ...(CHANNEL_CREDENTIAL_FIELDS[platform] || []).map((field) => field.key),
    'enabled',
  ]);
  const extraKeys = Object.keys(config).filter((key) => !knownKeys.has(key));

  const toggleRevealed = (key: string) => {
    setRevealedSecrets((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const labels: RenderLabels = {
    showSecret: t('channels.showSecret'),
    hideSecret: t('channels.hideSecret'),
  };

  const renderFields = (fields: WizardFormField[]) =>
    fields.map((field) => (
      <div key={field.key} className="space-y-2">
        {field.type !== 'boolean' ? (
          <Label>
            {field.label}
            {field.required ? <span className="ml-0.5 text-destructive">*</span> : null}
          </Label>
        ) : null}
        {renderFieldControl(
          field,
          fieldValue(field, config),
          revealedSecrets.has(field.key),
          labels,
          () => toggleRevealed(field.key),
          (next) => onFieldChange(field, coerceChannelFieldValue(field, next)),
        )}
        {field.hint && field.type !== 'boolean' ? (
          <p className="text-xs text-muted-foreground">{field.hint}</p>
        ) : null}
      </div>
    ));

  return (
    <div className="space-y-4">
      {!hasSchema && extraKeys.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t('channels.noEditableFields')}</p>
      ) : null}

      {basicFields.length > 0 ? (
        <div className="space-y-4">{renderFields(basicFields)}</div>
      ) : null}

      {advancedFields.length > 0 ? (
        <details className="group rounded-lg border p-3">
          <summary className="cursor-pointer select-none text-sm font-medium">
            {t('channels.advancedSettings')}
          </summary>
          <div className="mt-3 space-y-4">{renderFields(advancedFields)}</div>
        </details>
      ) : null}

      {!hasSchema && extraKeys.length > 0 && onExtraKeyChange ? (
        <div className="space-y-4">
          {extraKeys.map((key) => (
            <div key={key} className="space-y-2">
              <Label>{key}</Label>
              <Textarea
                rows={3}
                className="font-mono text-xs"
                value={
                  typeof config[key] === 'string'
                    ? (config[key] as string)
                    : JSON.stringify(config[key] ?? '', null, 2)
                }
                onChange={(event) => {
                  const raw = event.target.value;
                  try {
                    onExtraKeyChange(key, JSON.parse(raw));
                  } catch {
                    onExtraKeyChange(key, raw);
                  }
                }}
              />
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}