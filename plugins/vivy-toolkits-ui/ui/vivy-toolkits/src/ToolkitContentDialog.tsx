import type { RefObject } from 'react';
import type { UITranslator } from '@vivy/ui-sdk';
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

export interface ComponentBody {
  readonly title: string;
  readonly content: string;
  readonly digest: string;
}

/**
 * Read-only component content, pinned to the inspected release digest.
 *
 * Focus returns to the control that opened it, and the dialog is the only modal
 * layer on this page: it is never opened from another dialog.
 */
export function ToolkitContentDialog({
  body,
  triggerRef,
  onClose,
  t,
}: {
  readonly body: ComponentBody | null;
  readonly triggerRef: RefObject<HTMLButtonElement | null>;
  readonly onClose: () => void;
  readonly t: UITranslator;
}) {
  return (
    <Dialog
      open={Boolean(body)}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent
        data-testid="component-content-dialog"
        className="max-w-3xl"
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          triggerRef.current?.focus();
        }}
      >
        <DialogHeader>
          <DialogTitle className="break-all">
            {t('plugin.vivy/toolkits-ui.componentContent')}: {body?.title}
          </DialogTitle>
          <DialogDescription className="break-all font-mono text-xs">
            {t('plugin.vivy/toolkits-ui.contentDigest')} {body?.digest}
          </DialogDescription>
        </DialogHeader>
        <DialogBody>
          <pre className="whitespace-pre-wrap break-words rounded-xl bg-muted/40 p-3 text-xs">{body?.content}</pre>
        </DialogBody>
      </DialogContent>
    </Dialog>
  );
}
