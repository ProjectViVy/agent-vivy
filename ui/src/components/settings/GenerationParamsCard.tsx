import { useEffect, useMemo, useState } from 'react';
import { Check, SlidersHorizontal } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Slider } from '@/components/ui/slider';
import { getDemoGenParams, saveDemoGenParams } from '@/lib/demo-api';
import type { DemoGenParams } from '@/lib/types';
import { useVivyStore } from '@/lib/store';
import { savedModelVendorLabel, useSavedModels, type SavedModelEntry } from './saved-models';
import { useTranslation } from '@/i18n';

/** 模型运行三元组键（provider/baseUrl/model），与顶栏/模型配置卡同口径。 */
function modelKey(entry: Pick<SavedModelEntry, 'provider' | 'baseUrl' | 'model'>): string {
  return `${entry.provider}/${entry.baseUrl}/${entry.model}`;
}

/**
 * 设置 → 通用 Tab 的「高级特性」卡：按模型独立保存的演示生成参数。
 *
 * - 模型下拉 = 已选模型快捷列表 ∪ 当前运行模型（即使未加入快捷列表也能编辑）；
 * - 仅对下拉选中的模型可编辑生成参数（温度 / 最大 Tokens），按模型三元组键
 *   独立落 `vivy.demo.gen-params`，互不覆盖、绝不传给真实 Provider。
 */
export function GenerationParamsCard() {
  const { t } = useTranslation();
  const settings = useVivyStore((state) => state.settings);
  const savedModels = useSavedModels();

  const options = useMemo(() => {
    const merged = new Map<string, SavedModelEntry>();
    const current: SavedModelEntry | null =
      settings?.provider && settings.default_model
        ? { provider: settings.provider, baseUrl: settings.base_url, model: settings.default_model }
        : null;
    if (current) merged.set(modelKey(current), current);
    for (const entry of savedModels) merged.set(modelKey(entry), entry);
    return [...merged.values()];
  }, [savedModels, settings]);

  const [selectedKey, setSelectedKey] = useState('');
  const [genParams, setGenParams] = useState<DemoGenParams | null>(null);
  const [genStatus, setGenStatus] = useState<'idle' | 'saving' | 'saved'>('idle');

  // 默认选中当前运行模型（settings 未就绪时等它载入）；否则选第一个可用模型。
  useEffect(() => {
    if (selectedKey || !options.length) return;
    const currentKey =
      settings?.provider && settings.default_model
        ? modelKey({ provider: settings.provider, baseUrl: settings.base_url, model: settings.default_model })
        : '';
    setSelectedKey(options.some((entry) => modelKey(entry) === currentKey) ? currentKey : modelKey(options[0]));
  }, [options, selectedKey, settings]);

  // 跟随所选模型载入其独立演示参数并复位保存反馈。
  useEffect(() => {
    if (!selectedKey) return;
    setGenParams(getDemoGenParams(selectedKey));
    setGenStatus('idle');
  }, [selectedKey]);

  const saveGenParams = () => {
    if (!selectedKey || !genParams) return;
    setGenStatus('saving');
    setGenParams(saveDemoGenParams(selectedKey, genParams));
    setGenStatus('saved');
  };

  const selectedEntry = options.find((entry) => modelKey(entry) === selectedKey);

  return (
    <Card>
      <CardHeader>
        <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <SlidersHorizontal className="h-5 w-5" aria-hidden="true" />
        </div>
        <CardTitle>{t('settings.advancedFeaturesTitle')}</CardTitle>
        <CardDescription>{t('settings.advancedFeaturesDescription')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {options.length ? (
          <>
            <div className="flex items-center gap-2">
              <p className="text-sm font-medium">{t('settings.generationParamsTitle')}</p>
              <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium leading-none text-muted-foreground">{t('settings.demoBadge')}</span>
            </div>
            <p className="text-xs text-muted-foreground">{t('settings.generationParamsDescription')}</p>
            <div className="space-y-1.5">
              <Label htmlFor="gen-params-model" className="text-xs">{t('settings.modelSelect')}</Label>
              <Select value={selectedKey} onValueChange={(value) => setSelectedKey(value)}>
                <SelectTrigger id="gen-params-model" className="w-full sm:max-w-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {options.map((entry) => (
                    <SelectItem key={modelKey(entry)} value={modelKey(entry)}>
                      <span className="font-medium">{savedModelVendorLabel(entry)}</span>
                      <span className="text-muted-foreground"> · </span>
                      <span className="font-mono text-xs">{entry.model}</span>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {selectedEntry ? (
                <p className="text-xs text-muted-foreground">{t('settings.genParamsModelHint', { model: selectedEntry.model })}</p>
              ) : null}
            </div>
            {genParams ? (
              <div className="space-y-4 rounded-lg border p-3">
                <div className="space-y-2">
                  <div className="flex items-center justify-between gap-2">
                    <Label className="text-xs">{t('settings.temperature')}</Label>
                    <span className="font-mono text-xs tabular-nums text-muted-foreground">{genParams.temperature.toFixed(1)}</span>
                  </div>
                  <Slider
                    value={[genParams.temperature]}
                    min={0}
                    max={2}
                    step={0.1}
                    onValueChange={([value]) => setGenParams({ ...genParams, temperature: value })}
                    aria-label={t('settings.temperature')}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="gen-params-max-tokens" className="text-xs">{t('settings.maxTokens')}</Label>
                  <Input
                    id="gen-params-max-tokens"
                    type="number"
                    min={1}
                    value={genParams.max_tokens}
                    onChange={(event) => {
                      const next = Number(event.target.value);
                      setGenParams({ ...genParams, max_tokens: Number.isFinite(next) ? Math.max(1, next) : 1 });
                    }}
                  />
                </div>
                <div className="flex items-center gap-3">
                  <Button type="button" size="sm" onClick={saveGenParams}>
                    {genStatus === 'saving' ? t('settings.saving') : t('settings.saveDemoParams')}
                  </Button>
                  {genStatus === 'saved' ? (
                    <span className="inline-flex items-center gap-1.5 text-sm text-muted-foreground">
                      <Check className="h-4 w-4 text-primary" aria-hidden="true" />
                      {t('settings.savedLocally')}
                    </span>
                  ) : null}
                </div>
              </div>
            ) : (
              <div className="h-32 animate-pulse rounded bg-muted" />
            )}
          </>
        ) : (
          <p className="rounded-md border border-dashed px-3 py-2 text-xs text-muted-foreground">{t('settings.noModelsForGenParams')}</p>
        )}
      </CardContent>
    </Card>
  );
}