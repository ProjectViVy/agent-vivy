import { Check, Lightbulb, LightbulbOff } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import type { ThinkingMode } from '@/lib/api';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/i18n';

// Seven effort levels plus the auto/on/off aliases (VCP F1); level entries show
// their raw value because the server records exactly that string.
const THINKING_MODES: { value: ThinkingMode; icon: LucideIcon; label: string; filled?: boolean }[] = [
  { value: 'auto', icon: Lightbulb, label: 'chatInput.thinkingModeAuto' },
  { value: 'minimal', icon: Lightbulb, label: 'minimal' },
  { value: 'low', icon: Lightbulb, label: 'low' },
  { value: 'medium', icon: Lightbulb, label: 'medium' },
  { value: 'high', icon: Lightbulb, label: 'high' },
  { value: 'xhigh', icon: Lightbulb, label: 'xhigh' },
  { value: 'max', icon: Lightbulb, label: 'max', filled: true },
  { value: 'on', icon: Lightbulb, label: 'chatInput.thinkingModeOn', filled: true },
  { value: 'off', icon: LightbulbOff, label: 'chatInput.thinkingModeOff' },
];

/** Shown only when the active model reports thinking support (D9); the server
 * still has the final word on the preference carried by every send. */
export function ComposerThinkingMenu({ value, onChange }: { value: ThinkingMode; onChange: (mode: ThinkingMode) => void }) {
  const { t } = useTranslation();
  const current = THINKING_MODES.find((mode) => mode.value === value) ?? THINKING_MODES[0];
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button type="button" className="shrink-0 rounded-lg p-1.5 transition-colors hover:bg-hover" title={t('chatInput.thinkingMode')} aria-label={t('chatInput.thinkingMode')}>
          {current.filled
            ? <current.icon className="h-4 w-4" fill="currentColor" />
            : <current.icon className="h-4 w-4" />}
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="bottom" align="start" className="min-w-36 p-1.5">
        {THINKING_MODES.map((mode) => (
          <DropdownMenuItem
            key={mode.value}
            onSelect={() => onChange(mode.value)}
            className={cn('gap-2', value === mode.value && 'bg-selected text-fg')}
          >
            {mode.filled
              ? <mode.icon className="size-4 shrink-0" fill="currentColor" />
              : <mode.icon className="size-4 shrink-0" />}
            <span className="flex-1">{mode.label.startsWith('chatInput.') ? t(mode.label) : mode.label}</span>
            {value === mode.value ? <Check className="size-4 shrink-0 text-brand-ink" /> : null}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
