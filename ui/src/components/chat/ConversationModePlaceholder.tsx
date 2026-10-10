import { useState } from 'react';
import { Code2, Coffee } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useTranslation } from '@/i18n';

/** Visual placeholder only: this choice deliberately does not route a Run. */
export function ConversationModePlaceholder() {
  const { t } = useTranslation();
  const [code, setCode] = useState(true);
  const label = t(code ? 'codeMode.code' : 'codeMode.life');
  const Icon = code ? Code2 : Coffee;
  return (
    <Button
      type="button"
      data-conversation-mode-placeholder=""
      variant="ghost"
      size="sm"
      className="h-7 shrink-0 gap-1 rounded-lg px-1.5 text-xs text-muted-foreground"
      aria-label={t('codeMode.placeholderAria', { mode: label })}
      title={t('codeMode.placeholderHint')}
      onClick={() => setCode(!code)}
    >
      <Icon className="hidden h-3.5 w-3.5 sm:block" />
      <span>{label}</span>
    </Button>
  );
}
