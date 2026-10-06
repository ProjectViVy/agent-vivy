import type { ChatHeaderContext } from '@vivy/ui-sdk';
import { usePluginHost, usePluginTranslation } from '@vivy/ui-sdk';
import { useState } from 'react';
import { Check, ChevronDown, ChevronRight, Loader2, Settings2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { MaskIdentity } from './MaskIdentity';
import { MaskSession, maskErrorText, useMaskSession } from './mask-session';

export function MaskHeader({ context, session: supplied }: { readonly context: ChatHeaderContext; readonly session?: MaskSession }) {
  const host = usePluginHost();
  const { t } = usePluginTranslation();
  const { session, state } = useMaskSession(host, supplied);
  const [open, setOpen] = useState(false);
  if (!host || !session) return null;
  const maskId = state.selection?.mask_id ?? '';
  const active = state.catalog.find((mask) => mask.id === maskId);
  const name = state.selectionPending ? t('plugin.vivy/masks-ui.loading') : active?.name ?? (maskId || t('plugin.vivy/masks-ui.unmasked'));
  const disabled = !state.sessionId || !state.selection?.available || state.selectionPending;
  const choose = async (id: string) => { if (await session.select(id)) setOpen(false); };
  return <div className="flex min-w-0 items-center gap-1" data-mask-header>
    <DropdownMenu open={open} onOpenChange={(next) => { setOpen(next); if (next) { void session.refreshCatalog(); void session.refreshSelection(); } }}>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" className="h-9 max-w-[180px] gap-2 px-1.5 font-normal sm:px-2.5" aria-label={t('plugin.vivy/masks-ui.selectorLabel')} disabled={!state.sessionId} title={!state.sessionId ? t('plugin.vivy/masks-ui.noActiveSession') : state.selection?.inactive_reason ? t('plugin.vivy/masks-ui.errors.unavailable') : context.running ? t('plugin.vivy/masks-ui.nextRun') : name}>
          {state.selectionPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <MaskIdentity id={maskId} compact />}
          <span className="hidden truncate text-sm sm:inline">{name}</span>
          <ChevronDown className="hidden h-3.5 w-3.5 shrink-0 text-muted-foreground sm:block" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-80 max-w-[calc(100vw-1rem)] p-2">
        <DropdownMenuLabel className="px-2 pb-2 text-xs font-normal text-muted-foreground">{context.running ? t('plugin.vivy/masks-ui.nextRun') : t('plugin.vivy/masks-ui.chooseMask')}</DropdownMenuLabel>
        {state.selection?.inactive_reason ? <p role="alert" className="px-2 py-2 text-xs text-destructive">{t('plugin.vivy/masks-ui.errors.unavailable')}</p> : null}
        {state.error ? <div role="alert" className="px-2 py-2 text-xs text-destructive">{maskErrorText(t, state.error)}<button type="button" className="ml-2 underline" onClick={() => void session.refreshSelection()}>{t('plugin.vivy/masks-ui.retry')}</button></div> : null}
        {[{ id: '', name: t('plugin.vivy/masks-ui.unmasked'), description: t('plugin.vivy/masks-ui.unmaskedHint') }, ...state.catalog].map((mask) => <DropdownMenuItem key={mask.id} disabled={disabled} className="gap-3 rounded-lg p-2.5" onSelect={(event) => { event.preventDefault(); void choose(mask.id); }}>
          <MaskIdentity id={mask.id} /><span className="min-w-0 flex-1"><span className="block truncate text-sm font-medium">{mask.name}</span><span className="block truncate text-xs text-muted-foreground">{mask.description}</span></span>
          {maskId === mask.id && state.selection ? <Check className="h-4 w-4 shrink-0 text-primary" /> : null}
        </DropdownMenuItem>)}
        <DropdownMenuSeparator className="my-2" />
        <DropdownMenuItem className="gap-2 rounded-lg px-2.5 py-2 text-muted-foreground" onSelect={() => void host.router.navigate({ to: '/masks' })}><Settings2 className="h-4 w-4" />{t('plugin.vivy/masks-ui.manageMasks')}<ChevronRight className="ml-auto h-3.5 w-3.5" /></DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
    <span aria-hidden="true" className="h-5 w-px shrink-0 bg-border" />
  </div>;
}
