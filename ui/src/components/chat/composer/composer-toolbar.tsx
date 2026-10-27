import type { ReactNode } from 'react';
import { Paperclip } from 'lucide-react';
import { useTranslation } from '@/i18n';
import { cn } from '@/lib/utils';

interface ComposerToolbarProps {
  history: ReactNode;
  thinking: ReactNode;
  permission: ReactNode;
  approval: ReactNode;
  onFiles: (files: Iterable<File>) => void;
  disabled: boolean;
}

/**
 * One row of composer controls. It wraps instead of scrolling horizontally:
 * at 390px the controls still fit on one line, and an overview of every
 * control beats a scroll gutter that hides the approval badge.
 */
export function ComposerToolbar({ history, thinking, permission, approval, onFiles, disabled }: ComposerToolbarProps) {
  return (
    <div className="flex flex-wrap items-center gap-1 px-3 pb-1 pt-2.5 text-fg-muted">
      {history}
      <ComposerAttachButton disabled={disabled} onFiles={onFiles} />
      {thinking}
      {permission}
      <div className="ml-auto flex shrink-0 items-center gap-1">{approval}</div>
    </div>
  );
}

/** Image attachments share the server's gate (png/jpeg/gif/webp, 5 MB, 4 max). */
export function ComposerAttachButton({ disabled, onFiles }: { disabled: boolean; onFiles: (files: Iterable<File>) => void }) {
  const { t } = useTranslation();
  return (
    <label
      className={cn('shrink-0 rounded-lg p-1.5 transition-colors hover:bg-hover', disabled ? 'pointer-events-none opacity-50' : 'cursor-pointer')}
      title={t('chatInput.attachment')}
      aria-label={t('chatInput.attachment')}
    >
      <Paperclip className="h-4 w-4" />
      <input
        type="file"
        accept="image/png,image/jpeg,image/gif,image/webp"
        multiple
        className="hidden"
        disabled={disabled}
        onChange={(event) => { onFiles(event.target.files ?? []); event.target.value = ''; }}
      />
    </label>
  );
}
