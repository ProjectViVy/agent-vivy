import { describe, expect, it } from 'vitest';
import { faceForMaskId, MASK_IDS } from './mask-catalog';

describe('faceForMaskId', () => {
  it('maps the programmer mask to the code face', () => {
    expect(faceForMaskId('programmer')).toBe('code');
  });

  it('leaves every other mask without a face override', () => {
    for (const id of MASK_IDS.filter((id) => id !== 'programmer')) {
      expect(faceForMaskId(id)).toBeUndefined();
    }
  });
});
