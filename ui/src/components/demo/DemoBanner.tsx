import { AlertTriangle, FlaskConical } from 'lucide-react';
import { Button } from '@/components/ui/button';

export function DemoBanner() { return <div className="flex items-center gap-2 border-b border-amber-500/30 bg-amber-500/10 px-4 py-2 text-xs text-amber-800 dark:text-amber-300"><FlaskConical className="h-3.5 w-3.5"/><strong>演示 / 本地模拟</strong><span>此页面没有后端 API，数据仅保存在 vivy.demo.* localStorage 中。</span></div>; }

export function DemoLoadError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return <div className="rounded-xl border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive"><div className="flex items-center gap-2"><AlertTriangle className="h-4 w-4"/><strong>演示数据加载失败</strong></div><p className="mt-1">{message}</p><Button className="mt-3" size="sm" variant="outline" onClick={onRetry}>重试</Button></div>;
}
