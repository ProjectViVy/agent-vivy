import { useEffect, useMemo, useState } from 'react';
import { Brain, Search } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { MasterDetail } from '@/components/layout/MasterDetail';
import { getDemoMemories } from '@/lib/demo-api';
import type { DemoMemoryItem } from '@/lib/types';
import { DemoLoadError } from './DemoBanner';

const CATEGORY_LABELS: Record<DemoMemoryItem['category'], string> = { preference: '偏好', project: '项目', decision: '决策' };

export function MemoryDemoView() {
  const [memories, setMemories] = useState<DemoMemoryItem[]>([]);
  const [query, setQuery] = useState('');
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const items = await getDemoMemories();
      setMemories(items);
      setSelectedId((current) => (current && items.some((item) => item.id === current) ? current : null));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { void load(); }, []);
  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return needle ? memories.filter((item) => `${item.title} ${item.content}`.toLowerCase().includes(needle)) : memories;
  }, [memories, query]);
  const selected = memories.find((item) => item.id === selectedId) ?? null;

  if (error) {
    return (
      <div className="h-full overflow-auto p-6">
        <div className="mx-auto max-w-xl">
          <DemoLoadError message={error} onRetry={() => void load()} />
        </div>
      </div>
    );
  }

  return (
    <MasterDetail
      selected={selectedId !== null}
      onBack={() => setSelectedId(null)}
      master={
        <section className="flex h-full min-h-0 flex-col border-b bg-sidebar md:border-b-0 md:border-r">
          <div className="border-b p-3">
            <div className="relative">
              <Search className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索记忆" className="pl-9" />
            </div>
          </div>
          <div className="min-h-0 flex-1 overflow-auto p-2">
            {loading ? (
              <div className="space-y-2 p-2">
                <div className="h-16 animate-pulse rounded-lg bg-muted" />
                <div className="h-16 animate-pulse rounded-lg bg-muted" />
              </div>
            ) : filtered.length ? filtered.map((item) => (
              <button
                key={item.id}
                type="button"
                onClick={() => setSelectedId(item.id)}
                className={`mb-1 w-full rounded-lg p-3 text-left ${selectedId === item.id ? 'bg-sidebar-accent' : 'hover:bg-sidebar-accent/50'}`}
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="truncate text-sm font-medium">{item.title}</span>
                  <Badge variant="outline">{CATEGORY_LABELS[item.category]}</Badge>
                </div>
                <p className="mt-1 truncate text-xs text-muted-foreground">{item.content}</p>
              </button>
            )) : (
              <p className="px-3 py-10 text-center text-sm text-muted-foreground">{query ? '没有匹配的记忆' : '暂无记忆'}</p>
            )}
          </div>
        </section>
      }
      detail={
        selected ? (
          <div className="p-4 sm:p-6">
            <Card className="mx-auto max-w-3xl">
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Brain className="h-5 w-5 shrink-0 text-primary" />
                  <span className="min-w-0">{selected.title}</span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <Badge>{CATEGORY_LABELS[selected.category]}</Badge>
                <p className="mt-4 leading-7 break-words">{selected.content}</p>
                <p className="mt-6 text-xs text-muted-foreground">更新于 {new Date(selected.updatedAt).toLocaleString()}</p>
              </CardContent>
            </Card>
          </div>
        ) : (
          <div className="flex h-full items-center justify-center p-6 text-muted-foreground">选择一条记忆查看详情</div>
        )
      }
    />
  );
}
