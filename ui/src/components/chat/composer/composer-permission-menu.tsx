import { useState } from 'react';
import { Check, CheckCircle, ChevronDown, Shield, Sparkles } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import type { PermissionPreset } from '@/lib/api';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/i18n';

export type PermissionMode = 'cautious' | 'smart' | 'trusted';

// Permission presets mirror Agent-DIVA's permissionOptions.
const PERMISSION_MODES: { value: PermissionMode; icon: LucideIcon; label: string; desc: string }[] = [
  { value: 'cautious', icon: Shield, label: 'chatInput.permissionCautious', desc: 'chatInput.permissionCautiousDesc' },
  { value: 'smart', icon: Sparkles, label: 'chatInput.permissionSmart', desc: 'chatInput.permissionSmartDesc' },
  { value: 'trusted', icon: CheckCircle, label: 'chatInput.permissionTrusted', desc: 'chatInput.permissionTrustedDesc' },
];

interface ComposerPermissionMenuProps {
  preset: PermissionPreset;
  locked: boolean;
  running: boolean;
  onApply: (preset: PermissionMode) => Promise<void> | void;
}

/** Trusted asks once before it is applied; the other presets apply directly. */
export function ComposerPermissionMenu({ preset, locked, running, onApply }: ComposerPermissionMenuProps) {
  const { t } = useTranslation();
  const [confirmTrusted, setConfirmTrusted] = useState(false);
  const current = PERMISSION_MODES.find((mode) => mode.value === preset) ?? PERMISSION_MODES[1];

  const choose = (next: PermissionMode) => {
    if (locked || next === preset) return;
    if (next === 'trusted') { setConfirmTrusted(true); return; }
    void onApply(next);
  };

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button type="button" disabled={locked} title={running ? t('chatInput.permissionLocked') : t(current.desc)} className="flex shrink-0 items-center gap-1 rounded-lg px-2 py-1 text-ui transition-colors hover:bg-hover disabled:opacity-50">
            <current.icon className="h-3.5 w-3.5" />
            <span>{preset === 'custom' ? t('chatInput.permissionCustom') : t(current.label)}</span>
            <ChevronDown className="h-3 w-3" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent side="top" align="start" className="w-64 p-1.5">
          {PERMISSION_MODES.map((mode) => (
            <DropdownMenuItem
              key={mode.value}
              disabled={locked}
              onSelect={() => choose(mode.value)}
              className={cn('gap-2.5 py-2', preset === mode.value && 'bg-selected text-fg')}
            >
              <mode.icon className="size-4 shrink-0" />
              <span className="min-w-0 flex-1">
                <span className="block text-ui font-semibold">{t(mode.label)}</span>
                <span className="block text-meta text-fg-muted">{t(mode.desc)}</span>
              </span>
              {preset === mode.value ? <Check className="size-4 shrink-0 text-brand" /> : null}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
      <AlertDialog open={confirmTrusted} onOpenChange={setConfirmTrusted}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('chatInput.trustedConfirmTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('chatInput.trustedConfirmDescription')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={() => { setConfirmTrusted(false); void onApply('trusted'); }}>{t('chatInput.trustedConfirm')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
