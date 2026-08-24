import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useVivyStore } from '@/lib/store';

export function RunInspector() {
  const run = useVivyStore((state) => state.currentRun);
  const events = useVivyStore((state) => state.runEvents);
  const background = useVivyStore((state) => state.backgroundRuns);
  const children = useVivyStore((state) => state.children);
  const selectedChild = useVivyStore((state) => state.selectedChild);
  const error = useVivyStore((state) => state.backgroundError ?? state.childrenError);
  const busyId = useVivyStore((state) => state.childBusyId ?? state.backgroundBusyId);
  const attach = useVivyStore((state) => state.attachBackgroundRun);
  const startChild = useVivyStore((state) => state.startChild);
  const openChild = useVivyStore((state) => state.openChild);
  const waitChild = useVivyStore((state) => state.waitChild);
  const cancelChild = useVivyStore((state) => state.cancelChild);
  const [childText, setChildText] = useState('');
  const [policy, setPolicy] = useState('');
  const [tools, setTools] = useState('');
  const active = (status: string) => !['completed', 'failed', 'cancelled'].includes(status);
  return <Tabs defaultValue="run" className="flex h-full flex-col"><TabsList className="mx-4 mt-4 grid grid-cols-3"><TabsTrigger value="run">当前 Run</TabsTrigger><TabsTrigger value="background">后台 {background.length}</TabsTrigger><TabsTrigger value="children">子 Run {children.length}</TabsTrigger></TabsList>
    <div className="flex-1 overflow-auto p-4"><TabsContent value="run" className="mt-0 space-y-4">{run ? <><div className="rounded-lg border p-3 text-sm"><div className="flex items-center justify-between"><code>{run.id}</code><Badge>{run.status}</Badge></div><div className="mt-2 text-muted-foreground">Session {run.session_id}</div></div><div><h3 className="mb-2 text-sm font-medium">事件</h3><div className="space-y-1">{events.slice().reverse().map((event) => <button key={event.seq} className="flex w-full items-center gap-2 rounded p-2 text-left text-xs hover:bg-muted" title={JSON.stringify(event.payload)}><span className="w-10 text-muted-foreground">#{event.seq}</span><span>{event.type}</span></button>)}</div></div></> : <p className="py-10 text-center text-sm text-muted-foreground">当前会话没有 Run</p>}</TabsContent>
    <TabsContent value="background" className="mt-0 space-y-2">{background.length ? background.map((item) => <div key={item.id} className="rounded-lg border p-3 text-sm"><div className="flex items-center justify-between"><div className="min-w-0"><div className="truncate font-medium">{item.id}</div><div className="text-xs text-muted-foreground">{item.status}</div></div><Button size="sm" variant="outline" disabled={busyId === item.id} onClick={() => void attach(item.id)}>打开</Button></div></div>) : <p className="py-10 text-center text-sm text-muted-foreground">没有后台 Run</p>}</TabsContent>
    <TabsContent value="children" className="mt-0 space-y-4">{run ? <form className="space-y-2 rounded-lg border p-3" onSubmit={async (event) => { event.preventDefault(); if (!childText.trim()) return; await startChild(childText.trim(), policy.trim(), tools.split(',').map((value) => value.trim()).filter(Boolean)); setChildText(''); }}><div className="text-sm font-medium">启动子 Run</div><Input value={childText} onChange={(event) => setChildText(event.target.value)} placeholder="委派任务"/><Input value={policy} onChange={(event) => setPolicy(event.target.value)} placeholder="Policy profile（可选）"/><Input value={tools} onChange={(event) => setTools(event.target.value)} placeholder="工具名，逗号分隔（可选）"/><Button type="submit" size="sm" disabled={!childText.trim() || busyId === 'create'}>启动</Button></form> : null}
    {children.map((child) => <div key={child.id} className="rounded-lg border p-3 text-sm"><button className="flex w-full items-center justify-between text-left" onClick={() => void openChild(child.id)}><span className="truncate">{'—'.repeat(Math.max(0, child.depth - 1))} {child.id}</span><Badge variant="outline">{child.status}</Badge></button>{selectedChild?.id === child.id ? <div className="mt-3 border-t pt-3"><p className="whitespace-pre-wrap text-xs text-muted-foreground">{selectedChild.result || selectedChild.error || '尚无结果'}</p>{active(child.status) ? <div className="mt-2 flex gap-2"><Button size="sm" variant="outline" disabled={busyId === child.id} onClick={() => void waitChild(child.id)}>等待完成</Button><Button size="sm" variant="destructive" disabled={busyId === child.id} onClick={() => void cancelChild(child.id)}>取消</Button></div> : null}</div> : null}</div>)}{!children.length ? <p className="py-8 text-center text-sm text-muted-foreground">没有子 Run</p> : null}</TabsContent>
    {error ? <p className="mt-3 rounded bg-destructive/10 p-2 text-sm text-destructive">{error}</p> : null}</div></Tabs>;
}
