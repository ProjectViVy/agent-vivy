import { useEffect, useState } from 'react';
import { Check, CircleHelp } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { Separator } from '@/components/ui/separator';
import { cn } from '@/lib/utils';
import { MaskIdentity } from './MaskIdentity';
import { MASK_OPTIONS, setActiveMaskId, useActiveMask } from './mask-catalog';

export function MaskManagementView() {
  const activeMask = useActiveMask();
  const [selectedId, setSelectedId] = useState(activeMask.id);
  const selectedMask = MASK_OPTIONS.find((option) => option.id === selectedId) ?? MASK_OPTIONS[0];

  useEffect(() => {
    setSelectedId(activeMask.id);
  }, [activeMask.id]);

  const activateSelected = () => setActiveMaskId(selectedMask.id);

  return (
    <div className="h-full overflow-auto">
      <div className="mx-auto max-w-6xl p-4 sm:p-6">
        <header className="mb-6 flex flex-wrap items-start justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold">面具</h1>
            <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
              为对话选择工作角色。面具只决定当前对话的工作方式，不会改写人格文档。
            </p>
          </div>
          <Badge variant="secondary" className="mt-1 gap-1.5 px-3 py-1">
            <Check className="h-3.5 w-3.5" />
            当前：{activeMask.name}
          </Badge>
        </header>

        <div className="grid gap-6 lg:grid-cols-[minmax(0,1.2fr)_minmax(20rem,0.8fr)]">
          <section aria-labelledby="mask-library-title">
            <div className="mb-3">
              <h2 id="mask-library-title" className="text-sm font-semibold">面具库</h2>
              <p className="mt-1 text-sm text-muted-foreground">选择一个面具查看它的工作范围，或直接设为当前面具。</p>
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              {MASK_OPTIONS.map((option) => {
                const isSelected = selectedMask.id === option.id;
                const isActive = activeMask.id === option.id;

                return (
                  <Card key={option.id} className={cn('overflow-hidden transition-colors', isSelected && 'border-primary/60')}>
                    <button
                      type="button"
                      aria-pressed={isSelected}
                      className={cn(
                        'flex w-full cursor-pointer items-start gap-3 p-4 text-left transition-colors hover:bg-accent/60',
                        isSelected && 'bg-accent/35',
                      )}
                      onClick={() => setSelectedId(option.id)}
                    >
                      <MaskIdentity option={option} />
                      <span className="min-w-0 flex-1">
                        <span className="flex items-center gap-2">
                          <span className="truncate font-medium">{option.name}</span>
                          {isActive ? <Badge variant="outline" className="shrink-0 text-[11px]">当前</Badge> : null}
                        </span>
                        <span className="mt-1 block text-sm text-muted-foreground">{option.description}</span>
                      </span>
                    </button>
                    <CardFooter className="border-t px-4 py-3">
                      <Button
                        size="sm"
                        variant={isActive ? 'secondary' : 'outline'}
                        disabled={isActive}
                        onClick={() => setActiveMaskId(option.id)}
                      >
                        {isActive ? <><Check className="mr-1.5 h-3.5 w-3.5" />当前使用</> : '设为当前'}
                      </Button>
                    </CardFooter>
                  </Card>
                );
              })}
            </div>
          </section>

          <div className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>面具详情</CardTitle>
                <CardDescription>当前选中的面具只影响对话工作方式。</CardDescription>
              </CardHeader>
              <CardContent className="space-y-5">
                <div className="flex items-center gap-3">
                  <MaskIdentity option={selectedMask} />
                  <div className="min-w-0">
                    <h2 className="truncate text-lg font-semibold">{selectedMask.name}</h2>
                    <p className="text-sm text-muted-foreground">{selectedMask.description}</p>
                  </div>
                </div>

                <div>
                  <p className="mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">适合用于</p>
                  <div className="flex flex-wrap gap-2">
                    {selectedMask.capabilities.map((capability) => <Badge key={capability} variant="secondary">{capability}</Badge>)}
                  </div>
                </div>

                <Separator />

                <div className="rounded-lg bg-muted/60 p-3 text-sm">
                  <p className="font-medium">与人格分开管理</p>
                  <p className="mt-1 text-muted-foreground">人格页面维护 IDENTITY.MD 等七份长期文档；面具是每次对话可切换的工作角色。</p>
                </div>

                <Button className="w-full" disabled={activeMask.id === selectedMask.id} onClick={activateSelected}>
                  {activeMask.id === selectedMask.id ? '当前正在使用' : `使用「${selectedMask.name}」`}
                </Button>
              </CardContent>
            </Card>

            <div className="flex gap-3 rounded-xl border bg-card p-4 text-sm">
              <CircleHelp className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
              <p className="text-muted-foreground">面具选择保存在本机的 UI 偏好中，聊天顶部的面具切换器会同步更新。</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
