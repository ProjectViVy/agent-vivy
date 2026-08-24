import { useEffect, useState } from 'react';
import { Cat, Focus, Heart, Sparkles } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { getDemoPet, interactWithDemoPet } from '@/lib/demo-api';
import type { DemoPetState } from '@/lib/types';
import { DemoLoadError } from './DemoBanner';

const MOODS: Array<{ value: DemoPetState['mood']; label: string; icon: typeof Heart }> = [
  { value: 'calm', label: '安抚', icon: Heart },
  { value: 'focused', label: '专注', icon: Focus },
  { value: 'curious', label: '探索', icon: Sparkles },
];

export function PetDemoView() {
  const [pet, setPet] = useState<DemoPetState | null>(null);
  const [busy, setBusy] = useState<DemoPetState['mood'] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const load = async () => { setError(null); try { setPet(await getDemoPet()); } catch (cause) { setError(cause instanceof Error ? cause.message : String(cause)); } };
  useEffect(() => { void load(); }, []);
  const interact = async (mood: DemoPetState['mood']) => { setBusy(mood); setError(null); try { setPet(await interactWithDemoPet(mood)); } catch (cause) { setError(cause instanceof Error ? cause.message : String(cause)); } finally { setBusy(null); } };
  return <div className="h-full overflow-auto p-6"><div className="mx-auto grid max-w-5xl gap-6 md:grid-cols-[minmax(0,1.2fr)_minmax(280px,.8fr)]">
    <Card className="overflow-hidden"><CardContent className="flex min-h-96 flex-col items-center justify-center bg-primary/5 p-8 text-center"><div className="flex h-36 w-36 items-center justify-center rounded-full border border-primary/20 bg-card shadow-sm"><Cat className="h-20 w-20 text-primary"/></div><h1 className="mt-6 text-2xl font-bold">Vivy 宠物</h1><p className="mt-2 max-w-md text-sm text-muted-foreground">本地演示伙伴会记住最近一次互动和当前状态。</p></CardContent></Card>
    <Card><CardHeader><CardTitle className="flex items-center justify-between">当前状态{pet ? <Badge>{pet.mood}</Badge> : null}</CardTitle></CardHeader><CardContent className="space-y-5">{error ? <DemoLoadError message={error} onRetry={() => void load()}/> : pet ? <><div><div className="mb-2 flex justify-between text-sm"><span>活力</span><span>{pet.energy}%</span></div><Progress value={pet.energy}/></div><p className="rounded-lg bg-muted p-3 text-sm text-muted-foreground">{pet.lastInteraction}</p><div className="grid grid-cols-3 gap-2">{MOODS.map(({ value, label, icon: Icon }) => <Button key={value} variant={pet.mood === value ? 'default' : 'outline'} disabled={busy !== null} onClick={() => void interact(value)}><Icon className="mr-1 h-4 w-4"/>{busy === value ? '互动中…' : label}</Button>)}</div></> : <div className="space-y-3"><div className="h-5 animate-pulse rounded bg-muted"/><div className="h-20 animate-pulse rounded bg-muted"/></div>}</CardContent></Card>
  </div></div>;
}
