import type { ChatHeaderContext } from '@vivy/ui-sdk';
import { usePluginHost, usePluginTranslation } from '@vivy/ui-sdk';
import { useState } from 'react';
import {
  Check,
  ChevronDown,
  ChevronRight,
  CircleAlert,
  Loader2,
  Settings2,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { useIsMobile } from '@/hooks/use-mobile';
import { MaskIdentity } from './MaskIdentity';
import { MaskSession, maskErrorText, useMaskSession } from './mask-session';
import { defaultIdentity } from './mask-view';

export function MaskHeader({
  context,
  session: supplied,
  label,
}: {
  readonly context: ChatHeaderContext;
  readonly session?: MaskSession;
  readonly label?: string;
}) {
  const host = usePluginHost();
  const { t } = usePluginTranslation();
  const { session, state } = useMaskSession(host, supplied);
  const mobile = useIsMobile();
  const [open, setOpen] = useState(false);
  if (!host || !session) return null;
  const maskId = state.selection?.mask_id ?? '';
  const choices = [defaultIdentity(t), ...state.catalog];
  const active = choices.find((mask) => mask.id === maskId);
  const unknown = Boolean(state.sessionId && !state.selection);
  const name = unknown
    ? t('plugin.vivy/masks-ui.identityPending')
    : (active?.name ?? maskId);
  const disabled =
    !state.sessionId || !state.selection?.available || state.selectionPending;
  const toggle = (next: boolean) => {
    setOpen(next);
    if (next) {
      void session.refreshCatalog();
      void session.refreshSelection();
    }
  };
  const choose = async (id: string) => {
    if (await session.select(id)) setOpen(false);
  };
  const trigger = (
    <Button
      onClick={mobile ? () => toggle(true) : undefined}
      variant={label ? 'outline' : 'ghost'}
      className={`h-9 min-w-0 max-w-[160px] gap-1.5 rounded-xl px-1.5 text-sm ${label ? '' : 'bg-muted/40 sm:max-w-[200px] sm:px-3'}`}
      aria-label={t('plugin.vivy/masks-ui.selectorAria', { name })}
      disabled={!state.sessionId}
      title={
        !state.sessionId
          ? t('plugin.vivy/masks-ui.noActiveSession')
          : state.selection?.inactive_reason
            ? t('plugin.vivy/masks-ui.errors.unavailable')
            : context.running
              ? t('plugin.vivy/masks-ui.nextRun')
              : name
      }
    >
      {state.selectionPending ? (
        <Loader2 className="h-4 w-4 shrink-0 animate-spin" />
      ) : unknown ? (
        <CircleAlert className="h-4 w-4 shrink-0 text-amber-600" />
      ) : (
        <MaskIdentity id={maskId} compact />
      )}
      <span className="min-w-0 truncate font-medium">{label ?? name}</span>
      <ChevronDown className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
    </Button>
  );
  const warnings = (
    <>
      {state.selection?.inactive_reason ? (
        <p role="alert" className="px-2 py-2 text-xs text-destructive">
          {t('plugin.vivy/masks-ui.errors.unavailable')}
        </p>
      ) : null}
      {state.error ? (
        <div role="alert" className="px-2 py-2 text-xs text-destructive">
          {maskErrorText(t, state.error)}
          <button
            type="button"
            className="ml-2 underline"
            onClick={() => {
              void session.refreshCatalog();
              void session.refreshSelection();
            }}
          >
            {t('plugin.vivy/masks-ui.retry')}
          </button>
        </div>
      ) : null}
      {state.catalogPending ? (
        <p className="px-2 py-1 text-xs text-muted-foreground">
          {t('plugin.vivy/masks-ui.loading')}
        </p>
      ) : null}
    </>
  );
  const content = choices.map((mask) => {
    const row = (
      <>
        <MaskIdentity id={mask.id} />
        <span className="min-w-0 flex-1">
          <span className="block break-words text-sm font-medium">
            {mask.name}
          </span>
          <span className="mt-0.5 block break-words text-xs leading-relaxed text-muted-foreground">
            {mask.description}
          </span>
        </span>
        {maskId === mask.id && state.selection ? (
          <Check className="h-4 w-4 shrink-0 text-primary" />
        ) : null}
      </>
    );
    return mobile ? (
      <button
        key={mask.id}
        type="button"
        disabled={disabled}
        aria-pressed={Boolean(state.selection && maskId === mask.id)}
        className="flex w-full items-center gap-3 rounded-xl p-3 text-left hover:bg-accent/50 disabled:opacity-50"
        onClick={() => void choose(mask.id)}
      >
        {row}
      </button>
    ) : (
      <DropdownMenuItem
        key={mask.id}
        disabled={disabled}
        className="gap-3 rounded-xl p-2.5"
        onSelect={(event) => {
          event.preventDefault();
          void choose(mask.id);
        }}
      >
        {row}
      </DropdownMenuItem>
    );
  });
  const manage = () => {
    setOpen(false);
    void host.router.navigate({ to: '/masks' });
  };
  return (
    <div className="flex min-w-0" data-mask-header>
      {mobile ? (
        <>
          {trigger}
          <Sheet open={open} onOpenChange={toggle}>
            <SheetContent side="bottom" className="max-h-[85dvh] rounded-t-2xl">
              <SheetHeader className="text-left">
                <SheetTitle>{t('plugin.vivy/masks-ui.chooseMask')}</SheetTitle>
                <SheetDescription>
                  {t('plugin.vivy/masks-ui.currentSession')}
                </SheetDescription>
              </SheetHeader>
              <SheetBody className="px-3 pb-6">
                {warnings}
                {content}
                <Button
                  variant="ghost"
                  className="mt-3 w-full justify-start gap-2 border-t pt-3"
                  onClick={manage}
                >
                  <Settings2 className="h-4 w-4" />
                  {t('plugin.vivy/masks-ui.manageMasks')}
                  <ChevronRight className="ml-auto h-4 w-4" />
                </Button>
              </SheetBody>
            </SheetContent>
          </Sheet>
        </>
      ) : (
        <DropdownMenu open={open} onOpenChange={toggle}>
          <DropdownMenuTrigger asChild>{trigger}</DropdownMenuTrigger>
          <DropdownMenuContent
            align="start"
            className="max-h-[min(32rem,80dvh)] w-80 max-w-[calc(100vw-1rem)] overflow-auto rounded-2xl p-2"
          >
            <DropdownMenuLabel className="px-2 pb-2 text-xs font-normal text-muted-foreground">
              {context.running
                ? t('plugin.vivy/masks-ui.nextRun')
                : t('plugin.vivy/masks-ui.chooseMask')}
            </DropdownMenuLabel>
            {warnings}
            {content}
            <DropdownMenuSeparator className="my-2" />
            <DropdownMenuItem
              className="gap-2 rounded-xl px-2.5 py-2 text-muted-foreground"
              onSelect={manage}
            >
              <Settings2 className="h-4 w-4" />
              {t('plugin.vivy/masks-ui.manageMasks')}
              <ChevronRight className="ml-auto h-3.5 w-3.5" />
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  );
}
