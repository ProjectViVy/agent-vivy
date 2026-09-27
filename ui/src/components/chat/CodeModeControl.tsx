import { Code2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useTranslation } from '@/i18n';
import { useVivyStore } from '@/lib/store';

/**
 * Toggles the code Face independently from the legacy mask selector.
 * Availability comes from the control-plane initialize response, so an old
 * backend simply leaves this control out of the shell.
 */
export function CodeModeControl() {
  const { t } = useTranslation();
  const activeSessionId = useVivyStore((state) => state.activeSessionId);
  const available = useVivyStore((state) => state.codeModeAvailable);
  const enabled = useVivyStore((state) => state.codeMode);
  const setCodeMode = useVivyStore((state) => state.setCodeMode);

  if (!available) return null;

  const label = enabled ? t('codeMode.enabled') : t('codeMode.disabled');
  const hasSession = Boolean(activeSessionId);
  return (
    <Button
      type="button"
      data-code-mode-control=""
      variant={enabled ? 'secondary' : 'ghost'}
      size="sm"
      disabled={!hasSession}
      aria-pressed={enabled}
      aria-label={label}
      title={hasSession ? label : t('codeMode.noSession')}
      onClick={() => setCodeMode(!enabled)}
    >
      <Code2 className="h-4 w-4 shrink-0" aria-hidden="true" />
      <span className="hidden sm:inline">{label}</span>
    </Button>
  );
}
