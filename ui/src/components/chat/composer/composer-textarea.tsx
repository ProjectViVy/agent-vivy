import { useEffect } from 'react';
import type { ClipboardEvent, FocusEvent, KeyboardEvent, RefObject } from 'react';
import { Textarea } from '@/components/ui/textarea';

interface ComposerTextareaProps {
  textareaRef: RefObject<HTMLTextAreaElement | null>;
  value: string;
  disabled: boolean;
  placeholder: string;
  label: string;
  menu: { open: boolean; id: string; activeDescendant?: string };
  onChange: (value: string) => void;
  onKeyDown: (event: KeyboardEvent<HTMLTextAreaElement>) => void;
  onPaste: (event: ClipboardEvent<HTMLTextAreaElement>) => void;
  onBlur: (event: FocusEvent<HTMLTextAreaElement>) => void;
}

/**
 * The composer's single autogrow owner. Height is written inline (the only
 * thing that reliably beats the textarea primitive's own `min-h-[60px]`), and
 * `40vh` is the one cap: the old pair of `max-h-40` + a 160px JS cap fought
 * each other and the primitive's minimum, so the box could neither grow to the
 * viewport nor shrink back.
 */
export function ComposerTextarea({ textareaRef, value, disabled, placeholder, label, menu, onChange, onKeyDown, onPaste, onBlur }: ComposerTextareaProps) {
  useEffect(() => {
    const textarea = textareaRef.current;
    if (!textarea) return;
    textarea.style.height = 'auto';
    textarea.style.height = `${textarea.scrollHeight}px`;
  }, [textareaRef, value]);

  return (
    <Textarea
      ref={textareaRef}
      role="combobox"
      aria-label={label}
      aria-autocomplete="list"
      aria-expanded={menu.open}
      aria-controls={menu.open ? menu.id : undefined}
      aria-activedescendant={menu.activeDescendant}
      value={value}
      onChange={(event) => onChange(event.target.value)}
      onPaste={onPaste}
      onBlur={onBlur}
      onKeyDown={onKeyDown}
      placeholder={placeholder}
      disabled={disabled}
      className="resize-none border-0 bg-transparent px-4 shadow-none focus-visible:ring-0"
      style={{ maxHeight: '40vh', minHeight: 'var(--control-lg)', overflowY: 'auto' }}
      rows={1}
    />
  );
}
