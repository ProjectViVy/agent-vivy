import { useRef, useState } from 'react';
import { ImagePlus, Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useTranslation } from '@/i18n';
import { cn } from '@/lib/utils';
import {
  COMPANION_AVATAR_SIZE,
  COMPANION_PHOTO_MAX_BYTES,
  COMPANION_PHOTO_MAX_DATA_LENGTH,
  COMPANION_PHOTO_MIMES,
  useCompanionAvatar,
  type CompanionAvatar,
} from '@/hooks/use-companion-avatar';

/**
 * The companion's face: one renderer plus one chooser so the header button and
 * any Module step show the same thing and cannot drift. The stored value is a
 * presentation preference (see `use-companion-avatar`); nothing here reaches
 * the backend.
 */

/** Renders the stored face at a fixed size, so swapping it never reflows. */
export function CompanionAvatarFace({ avatar, className }: { avatar: CompanionAvatar; className?: string }) {
  return (
    <span
      className={cn(
        'flex h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-full bg-accent text-lg leading-none',
        className,
      )}
    >
      {avatar.kind === 'photo'
        ? <img src={avatar.value} alt="" className="h-full w-full object-cover" />
        : <span aria-hidden="true">{avatar.value}</span>}
    </span>
  );
}

type PhotoFailure = 'tooLarge' | 'failed';

/** Size and type are different answers: one is fixable, one is not a photo. */
export function photoFailureFor(file: { type: string; size: number }): PhotoFailure | null {
  if (file.size > COMPANION_PHOTO_MAX_BYTES) return 'tooLarge';
  if (!(COMPANION_PHOTO_MIMES as readonly string[]).includes(file.type)) return 'failed';
  return null;
}

export function CompanionAvatarPicker({ className }: { className?: string }) {
  const { t } = useTranslation();
  const { avatar, presets, setAvatar } = useCompanionAvatar();
  const inputRef = useRef<HTMLInputElement>(null);
  const [failure, setFailure] = useState<PhotoFailure | null>(null);
  const [reading, setReading] = useState(false);

  const choosePhoto = async (file: File | undefined) => {
    if (!file) return;
    setFailure(null);
    const rejected = photoFailureFor(file);
    if (rejected) {
      setFailure(rejected);
      return;
    }
    setReading(true);
    try {
      const data = await downscalePhoto(file);
      if (data.length > COMPANION_PHOTO_MAX_DATA_LENGTH) {
        setFailure('tooLarge');
        return;
      }
      setAvatar({ kind: 'photo', value: data });
    } catch {
      setFailure('failed');
    } finally {
      setReading(false);
    }
  };

  return (
    <div className={cn('space-y-3', className)}>
      <div>
        <p className="text-xs font-medium text-muted-foreground">{t('companionAvatar.emojiLabel')}</p>
        <div role="radiogroup" aria-label={t('companionAvatar.emojiLabel')} className="mt-2 grid grid-cols-6 gap-1.5">
          {presets.map((emoji) => {
            const selected = avatar.kind === 'emoji' && avatar.value === emoji;
            return (
              <button
                key={emoji}
                type="button"
                role="radio"
                aria-checked={selected}
                aria-label={emoji}
                onClick={() => { setFailure(null); setAvatar({ kind: 'emoji', value: emoji }); }}
                className={cn(
                  'flex h-9 w-9 cursor-pointer items-center justify-center rounded-full text-lg leading-none transition-colors duration-150 motion-reduce:transition-none',
                  selected ? 'bg-primary/15 ring-2 ring-primary' : 'bg-muted hover:bg-accent',
                )}
              >
                <span aria-hidden="true">{emoji}</span>
              </button>
            );
          })}
        </div>
      </div>
      <div className="flex items-center gap-2">
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="cursor-pointer"
          disabled={reading}
          onClick={() => inputRef.current?.click()}
        >
          {reading ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <ImagePlus className="mr-1.5 h-3.5 w-3.5" />}
          {t('companionAvatar.photoAction')}
        </Button>
        <input
          ref={inputRef}
          type="file"
          className="hidden"
          accept={COMPANION_PHOTO_MIMES.join(',')}
          onChange={(event) => {
            void choosePhoto(event.target.files?.[0]);
            // Allow re-picking the same file after a failure.
            event.target.value = '';
          }}
        />
      </div>
      <p className="text-xs text-muted-foreground">{t('companionAvatar.photoHint')}</p>
      {failure ? (
        <p role="alert" className="text-xs text-destructive">
          {failure === 'tooLarge' ? t('companionAvatar.photoTooLarge') : t('companionAvatar.photoFailed')}
        </p>
      ) : null}
    </div>
  );
}

function readFileAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result ?? ''));
    reader.onerror = () => reject(reader.error ?? new Error('file read failed'));
    reader.readAsDataURL(file);
  });
}

function loadImage(source: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error('image decode failed'));
    image.src = source;
  });
}

/**
 * Centre-crops to a square and downscales before storing, so one photo cannot
 * fill the origin's localStorage quota. WebP keeps the result small; a browser
 * without a WebP encoder falls back to PNG, which the stored-value guard also
 * accepts.
 */
async function downscalePhoto(file: File): Promise<string> {
  const image = await loadImage(await readFileAsDataURL(file));
  const canvas = document.createElement('canvas');
  canvas.width = COMPANION_AVATAR_SIZE;
  canvas.height = COMPANION_AVATAR_SIZE;
  const context = canvas.getContext('2d');
  if (!context) throw new Error('canvas unavailable');
  const side = Math.min(image.width, image.height);
  context.drawImage(
    image,
    (image.width - side) / 2,
    (image.height - side) / 2,
    side,
    side,
    0,
    0,
    COMPANION_AVATAR_SIZE,
    COMPANION_AVATAR_SIZE,
  );
  return canvas.toDataURL('image/webp', 0.85);
}
