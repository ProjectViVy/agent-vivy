import { useEffect, useState } from 'react';
import { Wrench } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Switch } from '@/components/ui/switch';
import { listTools, setActiveTools, type ToolsCatalogView } from '@/lib/api';
import { useVivyStore } from '@/lib/store';

/**
 * 工具配置（真实）：tools/list 拉取内置注册表全量目录（激活 + 隐藏），
 * tools/set-active 以整表替换 settings.yaml 的 tools_enabled 覆盖层。保存后
 * 运行中的引擎在空闲时重建，下一次 run 绑定新的激活面。隐藏工具不进入模型
 * 请求、不占上下文；SKILL 声明的工具集可在 run 内动态挂载。
 */
export function ToolsSettingsCard() {
  const settings = useVivyStore((state) => state.settings);
  const [view, setView] = useState<ToolsCatalogView | null>(null);
  const [active, setActive] = useState<string[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);
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

  const save = async () => {
    setSaving(true);
    setFeedback(null);
    try {
      const next = await setActiveTools(active);
      setView(next);
      setActive(next.active);
      setFeedback(next.active.length === 0 ? '已保存：纯对话模式（0 个工具）。' : `已保存：${next.active.length} 个工具激活，对下一次运行生效。`);
    } catch (error) {
      setFeedback(error instanceof Error ? error.message : String(error));
    } finally {
      setSaving(false);
    }
  };

  const reset = () => {
    if (view) setActive(view.config_enabled);
  };

  if (loadError) {
    return (
      <div className="space-y-3">
        <p className="rounded bg-destructive/10 p-3 text-sm text-destructive">{loadError}</p>
        <Button type="button" variant="outline" onClick={() => void load()}>重试</Button>
      </div>
    );
  }
  if (!view) {
    return <div className="h-56 animate-pulse rounded bg-muted" />;
  }

  return (
    <CardContent className="space-y-4">
      <p className="rounded-lg border p-3 text-xs text-muted-foreground">
        激活的工具每次请求都会绑定给模型；隐藏的工具不进入模型请求、不占上下文。
        隐藏工具仍可由声明它的 SKILL 在运行中动态挂载。配置默认值：{view.config_enabled.length} 个
        {view.overlay_written ? '（当前使用自定义覆盖层）' : '（当前未写覆盖层）'}。
      </p>
      <div className="space-y-2">
        {view.tools.map((tool) => {
          const checked = active.includes(tool.name);
          return (
            <div key={tool.name} className="flex items-center justify-between gap-3 rounded-lg border p-3">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <code className="text-sm font-medium">{tool.name}</code>
                  {tool.readonly ? <Badge variant="outline">只读</Badge> : <Badge variant="secondary">需审批</Badge>}
                </div>
                <p className="mt-1 truncate text-sm text-muted-foreground" title={tool.description}>{tool.description}</p>
              </div>
              <Switch checked={checked} disabled={locked || saving} onCheckedChange={(checked) => toggle(tool.name, checked)} aria-label={`${tool.name} 激活状态`} />
            </div>
          );
        })}
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <Button type="button" disabled={locked || saving || !dirty} onClick={() => void save()}>{saving ? '保存中…' : '保存工具配置'}</Button>
        <Button type="button" variant="outline" disabled={locked || saving} onClick={reset}>恢复配置默认</Button>
        {dirty ? <span className="text-sm text-amber-600 dark:text-amber-400">有未保存的改动</span> : null}
      </div>
      {feedback ? <p className="text-xs text-muted-foreground" aria-live="polite">{feedback}</p> : null}
    </CardContent>
  );
}
