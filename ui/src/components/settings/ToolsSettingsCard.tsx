import { useTranslation } from '@/i18n';
import { useEffect, useState } from 'react';
import { Wrench } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Switch } from '@/components/ui/switch';
import { listTools, setActiveTools, type ToolsCatalogView } from '@/lib/api';
import { useVivyStore } from '@/lib/store';

type ToolsFeedback = { kind: 'saved'; count: number } | { kind: 'error'; message: string };

/**
 * 工具配置（真实）：tools/list 拉取内置注册表全量目录（激活 + 隐藏），
 * tools/set-active 以整表替换 settings.yaml 的 tools_enabled 覆盖层。保存后
 * 运行中的引擎在空闲时重建，下一次 run 绑定新的激活面。隐藏工具不进入模型
 * 请求、不占上下文；SKILL 声明的工具集可在 run 内动态挂载。
 */
export function ToolsSettingsCard() {
  const { t } = useTranslation();
  const settings = useVivyStore((state) => state.settings);
  const [view, setView] = useState<ToolsCatalogView | null>(null);
  const [active, setActive] = useState<string[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<ToolsFeedback | null>(null);
  const locked = Boolean(settings?.read_only || settings?.frozen);

  const load = async () => {
    setLoadError(null);
    try {
      const next = await listTools();
      setView(next);
      setActive(next.active);
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : String(error));
    }
  };

  useEffect(() => { void load(); }, []);

  const toggle = (name: string, checked: boolean) => {
    setActive((current) => (checked ? [...current, name] : current.filter((item) => item !== name)));
  };

  const dirty = Boolean(view) && (active.length !== (view?.active.length ?? 0) || active.some((name) => !(view?.active ?? []).includes(name)));

  const save = async (next?: string[]) => {
    setSaving(true);
    setFeedback(null);
    try {
      const result = await setActiveTools(next ?? active);
      setView(result);
      setActive(result.active);
      setFeedback({ kind: 'saved', count: result.active.length });
    } catch (error) {
      setFeedback({ kind: 'error', message: error instanceof Error ? error.message : String(error) });
    } finally {
      setSaving(false);
    }
  };

  // 恢复配置默认直接落盘：整表写回 config 默认并清掉覆盖层语义，
  // 不要求用户再点一次保存（否则按钮名与行为不符）。
  const reset = () => {
    if (view) void save(view.config_enabled);
  };

  if (loadError) {
    return (
      <div className="space-y-3">
        <p className="rounded bg-destructive/10 p-3 text-sm text-destructive">{loadError}</p>
        <Button type="button" variant="outline" onClick={() => void load()}>{t('common.retry')}</Button>
      </div>
    );
  }
  if (!view) {
    return <div className="h-56 animate-pulse rounded bg-muted" />;
  }

  return (
    <CardContent className="space-y-4">
      <p className="rounded-lg border p-3 text-xs text-muted-foreground">
        {t('toolsSettings.description', { count: view.config_enabled.length })}
        {' '}{view.overlay_written ? t('toolsSettings.overlay') : t('toolsSettings.noOverlay')}
      </p>
      <div className="space-y-2">
        {view.tools.map((tool) => {
          const checked = active.includes(tool.name);
          return (
            <div key={tool.name} className="flex items-center justify-between gap-3 rounded-lg border p-3">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <code className="text-sm font-medium">{tool.name}</code>
                  {tool.readonly ? <Badge variant="outline">{t('toolsSettings.readonly')}</Badge> : <Badge variant="secondary">{t('toolsSettings.approval')}</Badge>}
                </div>
                <p className="mt-1 truncate text-sm text-muted-foreground" title={tool.description}>{tool.description}</p>
              </div>
              <Switch checked={checked} disabled={locked || saving} onCheckedChange={(checked) => toggle(tool.name, checked)} aria-label={t('toolsSettings.activeLabel', { name: tool.name })} />
            </div>
          );
        })}
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <Button type="button" disabled={locked || saving || !dirty} onClick={() => void save()}>{saving ? t('toolsSettings.saving') : t('toolsSettings.save')}</Button>
        <Button type="button" variant="outline" disabled={locked || saving} onClick={reset}>{t('toolsSettings.reset')}</Button>
        {dirty ? <span className="text-sm text-amber-600 dark:text-amber-400">{t('toolsSettings.dirty')}</span> : null}
      </div>
      {feedback ? (
        <p className="text-xs text-muted-foreground" aria-live="polite">
          {feedback.kind === 'error' ? feedback.message : feedback.count === 0
            ? t('toolsSettings.savedEmpty')
            : t('toolsSettings.saved', { count: feedback.count })}
        </p>
      ) : null}
    </CardContent>
  );
}
