import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { FailureNotice } from '@/components/feedback/FailureNotice';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import type { UITranslator } from '@vivy/ui-sdk';
import {
  MEMORY_REASONS,
  type MemoryClient,
  type MemoryEntry,
  type MemoryOutcome,
} from './memory-client';

interface ContentDialogProps {
  readonly mode: 'add' | 'edit';
  /** The record being edited; ignored for add. Remount on revision change to reset the draft. */
  readonly entry?: MemoryEntry;
  readonly client: MemoryClient;
  readonly t: UITranslator;
  readonly onClose: () => void;
  /** Called with the applied outcome; the parent reloads authoritative state. */
  readonly onApplied: (outcome: MemoryOutcome) => void;
  /** Returns the freshest entry after a CAS conflict so the parent can rebase the form. */
  readonly onRebase: (id: string) => void;
}

/**
 * Add/Edit dialog. Writes are revision-CAS guarded: a memory_revision_conflict
 * keeps the dialog open with a rebase affordance — it never silently
 * overwrites the newer record.
 */
export function MemoryContentDialog({ mode, entry, client, t, onClose, onApplied, onRebase }: ContentDialogProps) {
  const [content, setContent] = useState(entry?.content ?? '');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [conflict, setConflict] = useState(false);

  const submit = async () => {
    const trimmed = content.trim();
    if (!trimmed || busy) return;
    setBusy(true);
    setError(null);
    setConflict(false);
    try {
      const outcome = mode === 'add'
        ? await client.add({ kind: 'long_term', content: trimmed })
        : await client.update({ id: entry!.id, content: trimmed, base_revision: entry!.revision });
      if (outcome.status === 'applied') {
        onApplied(outcome);
        return;
      }
      if (outcome.reason === MEMORY_REASONS.revisionConflict) {
        setConflict(true);
      } else {
        setError(outcome.reason ?? outcome.status);
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t(mode === 'add' ? 'plugin.vivy/memory.addTitle' : 'plugin.vivy/memory.editTitle')}</DialogTitle>
        </DialogHeader>
        <DialogBody>
          <Textarea
            value={content}
            onChange={(event) => setContent(event.target.value)}
            rows={8}
            placeholder={t('plugin.vivy/memory.contentPlaceholder')}
            aria-label={t('plugin.vivy/memory.contentPlaceholder')}
          />
          {conflict ? (
            <div className="mt-3">
              <FailureNotice variant="inline" title={t('plugin.vivy/memory.conflict')} onRetry={() => entry && onRebase(entry.id)} retryLabel={t('common.refresh')} />
            </div>
          ) : error ? (
            <div className="mt-3">
              <FailureNotice variant="inline" title={t('errors.genericTitle')} error={error} />
            </div>
          ) : null}
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>{t('common.cancel')}</Button>
          <Button disabled={busy || !content.trim()} onClick={() => void submit()}>
            {mode === 'add' ? t('plugin.vivy/memory.addTitle') : t('common.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

interface DeleteDialogProps {
  readonly entry: MemoryEntry;
  readonly client: MemoryClient;
  readonly t: UITranslator;
  readonly onClose: () => void;
  readonly onApplied: (outcome: MemoryOutcome) => void;
  readonly onRebase: (id: string) => void;
}

/** Tombstone dialog: the backend requires a human reason plus base_revision. */
export function MemoryDeleteDialog({ entry, client, t, onClose, onApplied, onRebase }: DeleteDialogProps) {
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [conflict, setConflict] = useState(false);

  const submit = async () => {
    const trimmed = reason.trim();
    if (!trimmed || busy) return;
    setBusy(true);
    setError(null);
    setConflict(false);
    try {
      const outcome = await client.remove({ id: entry.id, reason: trimmed, base_revision: entry.revision });
      if (outcome.status === 'applied') {
        onApplied(outcome);
        return;
      }
      if (outcome.reason === MEMORY_REASONS.revisionConflict) {
        setConflict(true);
      } else {
        setError(outcome.reason ?? outcome.status);
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('plugin.vivy/memory.deleteTitle')}</DialogTitle>
        </DialogHeader>
        <DialogBody>
          <p className="text-ui text-fg-muted">{t('plugin.vivy/memory.deleteHint')}</p>
          <Input
            className="mt-3"
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            placeholder={t('plugin.vivy/memory.reasonPlaceholder')}
            aria-label={t('plugin.vivy/memory.reasonPlaceholder')}
          />
          {conflict ? (
            <div className="mt-3">
              <FailureNotice variant="inline" title={t('plugin.vivy/memory.conflict')} onRetry={() => onRebase(entry.id)} retryLabel={t('common.refresh')} />
            </div>
          ) : error ? (
            <div className="mt-3">
              <FailureNotice variant="inline" title={t('errors.genericTitle')} error={error} />
            </div>
          ) : null}
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>{t('common.cancel')}</Button>
          <Button variant="destructive" disabled={busy || !reason.trim()} onClick={() => void submit()}>
            {t('common.delete')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
