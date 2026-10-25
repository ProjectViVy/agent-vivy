import { useEffect, useRef, useState } from 'react';
import type { ClipboardEvent, FocusEvent, KeyboardEvent } from 'react';
import { History, X } from 'lucide-react';
import type { AttachmentInput, PermissionPreset, RunMode, SessionContext, ThinkingMode } from '@/lib/api';
import { useVivyStore } from '@/lib/store';
import { useTranslation } from '@/i18n';
import { ContextReferenceChip } from '../ContextReferenceChip';
import { ComposerCommandControls, useComposerCommands } from '../ComposerCommands';
import { ComposerApproval } from './composer-approval';
import { ComposerFooter, type SendTrack } from './composer-footer';
import { ComposerPermissionMenu, type PermissionMode } from './composer-permission-menu';
import { ComposerQueue, type KernelQueueItem } from './composer-queue';
import { ComposerTextarea } from './composer-textarea';
import { ComposerThinkingMenu } from './composer-thinking-menu';
import { ComposerToolbar } from './composer-toolbar';

export interface ChatInputProps {
  onSend: (content: string, mode: RunMode, attachments?: AttachmentInput[], thinking?: ThinkingMode) => Promise<void> | void;
  /** Busy sends queue (Crush parity): the server gate still decides. */
  onQueue?: (content: string, mode: RunMode, attachments?: AttachmentInput[], thinking?: ThinkingMode) => Promise<void> | void;
  /** pi dual track: busy Enter = steer (turn/steer), Shift+Alt+Enter =
   *  follow-up (turn/follow_up); without them the local FIFO is used. */
  onSteer?: (content: string, mode: RunMode, attachments?: AttachmentInput[], thinking?: ThinkingMode) => Promise<void> | void;
  onFollowUp?: (content: string, mode: RunMode, attachments?: AttachmentInput[], thinking?: ThinkingMode) => Promise<void> | void;
  /** Send now: interrupt the active run and start this message instead. */
  onSendNow?: (content: string, mode: RunMode, attachments?: AttachmentInput[], thinking?: ThinkingMode) => Promise<void> | void;
  /** pi Alt+Up: pop the newest pending follow-up back into the editor. */
  onDequeue?: () => Promise<string | null>;
  onCancel?: () => Promise<void> | void;
  /** Opens the history picker, which the transcript view owns. */
  onOpenHistory?: () => void;
  disabled?: boolean;
  running?: boolean;
  placeholder?: string;
  /** Real server-side session/context occupancy; the meter reads 0 when null. */
  context?: SessionContext | null;
  /** Rewind prefill: a new seq writes the text into the draft and focuses it. */
  draftPreset?: { text: string; seq: number } | null;
}

const TEXT_ENCODER = new TextEncoder();

// Image attachment gate, identical to turn/start's server-side check (the
// server remains authoritative).
const ATTACHMENT_MIMES = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp']);
const MAX_ATTACHMENT_BYTES = 5 * 1024 * 1024;
const MAX_ATTACHMENTS = 4;

const fileToAttachment = (file: File): Promise<AttachmentInput> => new Promise((resolve, reject) => {
  const reader = new FileReader();
  reader.onload = () => {
    const result = String(reader.result ?? '');
    const comma = result.indexOf(',');
    resolve({ name: file.name, mime_type: file.type, data: comma >= 0 ? result.slice(comma + 1) : result });
  };
  reader.onerror = () => reject(reader.error ?? new Error('file read failed'));
  reader.readAsDataURL(file);
});

export function ChatInput({ onSend, onQueue, onSteer, onFollowUp, onSendNow, onDequeue, onCancel, onOpenHistory, disabled: inputDisabled, running, placeholder, context = null, draftPreset = null }: ChatInputProps) {
  const { t } = useTranslation();
  const [value, setValue] = useState('');
  const [pending, setPending] = useState<AttachmentInput[]>([]);
  const [notice, setNotice] = useState<string | null>(null);
  const [sending, setSending] = useState(false);
  const [thinkingMode, setThinkingMode] = useState<ThinkingMode>('auto');
  const sendLock = useRef(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const noticeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const commands = useComposerCommands(value, setValue);
  const disabled = Boolean(inputDisabled || sending);
  const canSend = Boolean(value.trim() || commands.draft?.name === 'plan' || commands.draft?.skill || commands.draft?.goalRef);

  const activeSessionId = useVivyStore((state) => state.activeSessionId);
  const sessions = useVivyStore((state) => state.sessions);
  const sessionBusyId = useVivyStore((state) => state.sessionBusyId);
  const setSessionPermission = useVivyStore((state) => state.setSessionPermission);
  const kernelQueue = useVivyStore((state) => state.kernelQueue);
  const queuedMessages = useVivyStore((state) => state.queuedMessages);
  const removeQueuedMessage = useVivyStore((state) => state.removeQueuedMessage);
  const removeKernelQueued = useVivyStore((state) => state.removeKernelQueued);
  const clearQueue = useVivyStore((state) => state.clearQueue);
  const draftReferences = useVivyStore((state) => state.draftReferences);
  const removeDraftReference = useVivyStore((state) => state.removeDraftReference);

  const activeSession = sessions.find((session) => session.id === activeSessionId);
  const permissionPreset: PermissionPreset = activeSession?.permission_preset ?? 'smart';
  const permissionLocked = Boolean(running) || sessionBusyId === activeSessionId || !activeSessionId;
  const draftBytes = TEXT_ENCODER.encode(value).length;

  // Kernel dual-track entries (a steer item is transient once enqueued; a
  // follow-up waits for settle) plus the local FIFO (attachments/references).
  const kernelItems: KernelQueueItem[] = [
    ...(kernelQueue?.steering ?? []).map((item) => ({ item, lane: 'steer' as const })),
    ...(kernelQueue?.follow_up ?? []).map((item) => ({ item, lane: 'follow_up' as const })),
  ];
  const totalQueued = kernelItems.length + queuedMessages.length;

  const showNotice = (message: string) => {
    setNotice(message);
    if (noticeTimer.current) clearTimeout(noticeTimer.current);
    noticeTimer.current = setTimeout(() => setNotice(null), 1800);
  };

  useEffect(() => () => { if (noticeTimer.current) clearTimeout(noticeTimer.current); }, []);
  // Switching sessions drops the pending attachments with the local draft.
  useEffect(() => { setPending([]); }, [activeSessionId]);
  // Rewind prefill applies once per seq (ref guard), then focuses the editor.
  const appliedPresetSeq = useRef(0);
  useEffect(() => {
    if (!draftPreset || draftPreset.seq === appliedPresetSeq.current) return;
    appliedPresetSeq.current = draftPreset.seq;
    setValue(draftPreset.text);
    textareaRef.current?.focus();
  }, [draftPreset]);

  const addFiles = async (files: Iterable<File>) => {
    if (disabled) return;
    let count = pending.length;
    for (const file of Array.from(files)) {
      if (count >= MAX_ATTACHMENTS) { showNotice(t('chatInput.attachmentMax', { count: MAX_ATTACHMENTS })); return; }
      if (!ATTACHMENT_MIMES.has(file.type)) { showNotice(t('chatInput.attachmentUnsupported', { name: file.name })); continue; }
      if (file.size > MAX_ATTACHMENT_BYTES) { showNotice(t('chatInput.attachmentTooLarge', { name: file.name })); continue; }
      try {
        const attachment = await fileToAttachment(file);
        if (count >= MAX_ATTACHMENTS) return;
        count += 1;
        setPending((current) => current.length >= MAX_ATTACHMENTS ? current : [...current, attachment]);
      } catch {
        showNotice(t('chatInput.attachmentReadFailed', { name: file.name }));
      }
    }
  };

  const send = async (track: SendTrack = 'send') => {
    if (disabled || sendLock.current || !activeSessionId) return;
    if (commands.menuOpen) { commands.choose(commands.index); textareaRef.current?.focus(); return; }
    const content = value.trim();
    if (!content && !commands.draft) return;
    if (commands.draft && useVivyStore.getState().workBusy) return;
    sendLock.current = true;
    setSending(true);
    try {
      const prepared = await commands.prepare(content, Boolean(pending.length || draftReferences.length));
      if (!commands.isCurrent()) return;
      if (prepared) {
        const outgoing = pending.length ? pending : undefined;
        if (running && !prepared.forceSend) {
          if (track === 'send_now') {
            if (!onSendNow) return;
            await onSendNow(prepared.text, 'normal', outgoing, thinkingMode);
          } else {
            const handler = track === 'follow_up' ? onFollowUp : onSteer;
            if (!handler && !onQueue) return;
            await (handler ?? onQueue)!(prepared.text, 'normal', outgoing, thinkingMode);
          }
        } else await onSend(prepared.text, 'normal', outgoing, thinkingMode);
      }
      if (!commands.isCurrent()) return;
      setValue('');
      setPending([]);
      commands.clear();
    } catch (error) {
      if (commands.isCurrent()) commands.setError(error instanceof Error ? error.message : String(error));
    } finally {
      sendLock.current = false;
      if (commands.isCurrent()) setSending(false);
    }
  };

  // pi Alt+Up: pop the newest pending follow-up back into the editor.
  const dequeue = async () => {
    if (!onDequeue) return;
    const text = await onDequeue();
    if (text) { setValue(text); textareaRef.current?.focus(); }
  };

  const applyPermission = async (preset: PermissionMode) => {
    if (!activeSessionId || permissionLocked || preset === permissionPreset) return;
    try {
      await setSessionPermission(activeSessionId, preset);
    } catch (error) {
      showNotice(error instanceof Error ? error.message : t('chatInput.permissionSwitchFailed'));
    }
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (commands.handleKeyDown(event)) return;
    // pi keys: busy Enter = steer, Shift+Alt+Enter = follow-up (browser-safe),
    // Alt+↑ = recall the newest queued item; Shift+Enter still breaks the line.
    // Cmd/Ctrl+Enter wins over plain Enter: while busy it sends now.
    if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) { event.preventDefault(); void send(running ? 'send_now' : 'send'); return; }
    if (event.key === 'Enter' && event.shiftKey && event.altKey && running) { event.preventDefault(); void send('follow_up'); return; }
    if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); void send('steer'); return; }
    if (event.key === 'ArrowUp' && event.altKey) { event.preventDefault(); void dequeue(); return; }
    // Two-stage Esc (Crush parity): the first clears the queue, the next cancels.
    if (event.key === 'Escape' && running) {
      event.preventDefault();
      if (totalQueued) clearQueue();
      else void onCancel?.();
    }
  };

  // Image paste (Crush parity): only a clipboard with images is taken over.
  const handlePaste = (event: ClipboardEvent<HTMLTextAreaElement>) => {
    const images = Array.from(event.clipboardData.files).filter((file) => file.type.startsWith('image/'));
    if (!images.length) return;
    event.preventDefault();
    void addFiles(images);
  };

  const handleBlur = (event: FocusEvent<HTMLTextAreaElement>) => {
    if (!event.currentTarget.parentElement?.contains(event.relatedTarget as Node | null)) commands.dismiss();
  };

  const placeholderText = placeholder || t(commands.draft?.name === 'goal' ? (commands.draft.goalRef ? 'composerCommands.resumePlaceholder' : 'composerCommands.goalPlaceholder') : commands.draft?.name === 'plan' ? 'composerCommands.planPlaceholder' : 'chatInput.placeholder');

  return (
    <div className="px-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] pt-2 sm:px-4 sm:pb-4">
      <div className="relative mx-auto w-full max-w-content rounded-2xl border border-line bg-raised shadow-sm">
        <ComposerToolbar
          onFiles={(files) => void addFiles(files)}
          disabled={disabled}
          history={onOpenHistory && activeSessionId ? (
            <button type="button" onClick={onOpenHistory} disabled={disabled} title={t('chatInput.attachHistory')} aria-label={t('chatInput.attachHistory')} className="shrink-0 rounded-lg p-1.5 transition-colors hover:bg-hover disabled:pointer-events-none disabled:opacity-50">
              <History className="h-4 w-4" />
            </button>
          ) : null}
          thinking={context?.thinking_supported ? <ComposerThinkingMenu value={thinkingMode} onChange={setThinkingMode} /> : null}
          permission={<ComposerPermissionMenu preset={permissionPreset} locked={permissionLocked} running={Boolean(running)} onApply={applyPermission} />}
          approval={<ComposerApproval />}
        />
        <ComposerQueue
          kernel={kernelItems}
          local={queuedMessages}
          onRemoveKernel={removeKernelQueued}
          onRemoveLocal={removeQueuedMessage}
          onClear={clearQueue}
        />
        {draftReferences.length ? (
          <div className="flex flex-wrap gap-1.5 border-t border-line px-3 py-2" aria-live="polite">
            {draftReferences.map((draft) => (
              <ContextReferenceChip key={draft.id} preview={draft.preview} onRemove={() => removeDraftReference(draft.id)} />
            ))}
          </div>
        ) : null}
        {pending.length ? (
          <div className="flex flex-wrap gap-2 border-t border-line px-3 py-2">
            {pending.map((item, index) => (
              <span key={`${item.name ?? 'image'}-${index}`} className="relative">
                <img src={`data:${item.mime_type};base64,${item.data}`} alt={item.name || t('chatInput.attachment')} className="h-14 w-14 rounded-lg border border-line object-cover" />
                <button type="button" onClick={() => setPending((current) => current.filter((_, position) => position !== index))} title={t('chatInput.removeAttachment')} aria-label={`${t('chatInput.removeAttachment')}: ${item.name || item.mime_type}`} className="absolute -right-1.5 -top-1.5 rounded-full bg-danger p-0.5 text-danger-fg shadow-sm transition-opacity hover:opacity-90"><X className="h-3 w-3" aria-hidden="true" /></button>
              </span>
            ))}
          </div>
        ) : null}
        <ComposerCommandControls commands={commands} disabled={disabled} focus={() => textareaRef.current?.focus()} />
        <ComposerTextarea
          textareaRef={textareaRef}
          value={value}
          disabled={disabled}
          placeholder={placeholderText}
          label={t('chatInput.message')}
          menu={{ open: commands.menuOpen, id: commands.menuId, activeDescendant: commands.menuOpen && commands.options.length ? `${commands.menuId}-${commands.index}` : undefined }}
          onChange={commands.changeValue}
          onKeyDown={handleKeyDown}
          onPaste={handlePaste}
          onBlur={handleBlur}
        />
        <ComposerFooter
          context={context}
          draftBytes={draftBytes}
          workspacePath={activeSession?.workspace_path ?? ''}
          notice={notice}
          onNotice={showNotice}
          running={Boolean(running)}
          disabled={disabled}
          canSend={canSend}
          totalQueued={totalQueued}
          permissionLocked={permissionLocked}
          onSend={send}
          onClearQueue={clearQueue}
          onCancel={onCancel}
        />
      </div>
    </div>
  );
}
