import { useEffect, useRef, useState } from 'react';
import { Check, Copy, Pencil, X } from 'lucide-react';
import type { Message } from '@/lib/api';
import { useTranslation } from '@/i18n';
import { copyTextToClipboard } from '@/lib/clipboard';
import { Textarea } from '@/components/ui/textarea';
import { MarkdownBody } from './markdown-block';
import { MessageAction } from './message-actions';

export function MessageUserBubble({
  message,
  streaming = false,
  actionsDisabled = false,
  onEditConfirm,
}: {
  message: Message;
  streaming?: boolean;
  actionsDisabled?: boolean;
  onEditConfirm?: (newText: string) => void | Promise<void>;
}) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState('');
  const [pending, setPending] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const copyTimer = useRef<number | null>(null);
  useEffect(() => () => { if (copyTimer.current !== null) window.clearTimeout(copyTimer.current); }, []);

  const copy = async () => {
    if (!(await copyTextToClipboard(message.content))) return;
    setCopied(true);
    if (copyTimer.current !== null) window.clearTimeout(copyTimer.current);
    copyTimer.current = window.setTimeout(() => setCopied(false), 1500);
  };

  const origin = message.provenance
    ? [message.provenance.channel || message.provenance.source, message.provenance.chat_id].filter(Boolean).join(' · ')
    : '';

  return (
    <article data-message-id={message.id} className="group my-4 flex min-w-0 justify-end">
      <div className="flex min-w-0 max-w-[min(78%,100%)] flex-col items-end">
        {origin ? <div className="mb-0.5 px-1 text-2xs text-fg-muted">{origin}</div> : null}
        <div className="w-fit max-w-full min-w-0 rounded-2xl bg-brand-fill px-4 py-3 text-read text-brand-fg shadow-sm">
          {message.attachments?.length ? (
            <div className="mb-2 flex flex-wrap justify-end gap-1.5">
              {message.attachments.map((item, index) => (
                <img
                  key={`${item.name ?? 'image'}-${index}`}
                  src={item.data_url}
                  alt={item.name || item.mime_type}
                  className="max-h-40 max-w-full rounded-xl border border-brand-fg/20 object-cover"
                />
              ))}
            </div>
          ) : null}
          {editing ? (
            <div className="min-w-[16rem] space-y-2">
              <Textarea
                value={draft}
                onChange={(event) => setDraft(event.target.value)}
                className="min-h-20 bg-canvas text-fg"
                aria-label={t('chat.edit')}
                autoFocus
              />
              <div className="flex justify-end gap-1">
                <button
                  type="button"
                  className="flex h-7 w-7 items-center justify-center rounded-md text-brand-fg/85 transition-colors hover:bg-brand-fg/15 hover:text-brand-fg disabled:cursor-not-allowed disabled:opacity-40"
                  disabled={!draft.trim() || !onEditConfirm || pending}
                  aria-label={t('chat.editSave')}
                  title={t('chat.editSave')}
                  onClick={async () => {
                    if (!onEditConfirm) return;
                    setPending(true);
                    setActionError(null);
                    try {
                      await onEditConfirm(draft);
                      setEditing(false);
                    } catch (error) {
                      setActionError(error instanceof Error ? error.message : String(error));
                    } finally {
                      setPending(false);
                    }
                  }}
                >
                  <Check className="h-3.5 w-3.5" />
                </button>
                <button
                  type="button"
                  className="flex h-7 w-7 items-center justify-center rounded-md text-brand-fg/85 transition-colors hover:bg-brand-fg/15 hover:text-brand-fg disabled:cursor-not-allowed disabled:opacity-40"
                  disabled={pending}
                  aria-label={t('chat.editCancel')}
                  title={t('chat.editCancel')}
                  onClick={() => setEditing(false)}
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>
              {actionError ? <p className="rounded bg-danger px-1.5 py-0.5 text-2xs text-danger-fg" role="alert">{actionError}</p> : null}
            </div>
          ) : (
            <MarkdownBody content={message.content} streaming={streaming} className="prose-inherit" />
          )}
        </div>
        {streaming || editing ? null : (
          <div className="mt-1 flex items-center gap-0.5 px-1 opacity-0 transition-opacity duration-150 group-hover:opacity-100 focus-within:opacity-100">
            <MessageAction label={copied ? t('chat.copied') : t('chat.copy')} onClick={() => void copy()}>
              {copied ? <Check className="h-3.5 w-3.5 text-brand-ink" /> : <Copy className="h-3.5 w-3.5" />}
            </MessageAction>
            <MessageAction
              label={t('chat.edit')}
              disabled={actionsDisabled || !onEditConfirm}
              onClick={() => { setDraft(message.content); setEditing(true); }}
            >
              <Pencil className="h-3.5 w-3.5" />
            </MessageAction>
          </div>
        )}
      </div>
    </article>
  );
}
