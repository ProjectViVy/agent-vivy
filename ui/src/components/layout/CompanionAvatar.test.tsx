import { describe, expect, it } from 'vitest';
import { photoFailureFor } from './CompanionAvatar';
import { COMPANION_PHOTO_MAX_BYTES } from '@/hooks/use-companion-avatar';

describe('photoFailureFor', () => {
  it('admits the supported image types at a sane size', () => {
    for (const type of ['image/png', 'image/jpeg', 'image/webp']) {
      expect(photoFailureFor({ type, size: 1024 })).toBeNull();
    }
  });

  it('separates an oversized photo from something that is not a photo', () => {
    // A too-large file is fixable by picking another one; a text file is not a
    // photo and must not be described as oversized.
    expect(photoFailureFor({ type: 'image/png', size: COMPANION_PHOTO_MAX_BYTES + 1 })).toBe('tooLarge');
    expect(photoFailureFor({ type: 'text/plain', size: 32 })).toBe('failed');
    expect(photoFailureFor({ type: '', size: 32 })).toBe('failed');
    expect(photoFailureFor({ type: 'image/gif', size: 32 })).toBe('failed');
  });

  it('treats the exact size limit as acceptable', () => {
    expect(photoFailureFor({ type: 'image/jpeg', size: COMPANION_PHOTO_MAX_BYTES })).toBeNull();
  });
});
