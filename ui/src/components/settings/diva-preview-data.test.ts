import { describe, expect, it } from 'vitest';
import {
  DIVA_ADDITIONAL_SECTIONS,
  DIVA_PREVIEW_SECTIONS,
} from './diva-preview-data';

describe('DIVA settings preview data', () => {
  it('keeps the migrated settings sections unique and excludes sections merged into real tabs', () => {
    expect(new Set(DIVA_PREVIEW_SECTIONS).size).toBe(DIVA_PREVIEW_SECTIONS.length);
    expect(DIVA_ADDITIONAL_SECTIONS).not.toContain('general');
    expect(DIVA_PREVIEW_SECTIONS).not.toContain('theme');
    expect(DIVA_PREVIEW_SECTIONS).not.toContain('audit');
    expect(DIVA_PREVIEW_SECTIONS).not.toContain('language');
    expect(DIVA_PREVIEW_SECTIONS).not.toContain('compaction');
    expect(DIVA_PREVIEW_SECTIONS).not.toContain('network');
    expect(DIVA_PREVIEW_SECTIONS).not.toContain('channels');
    expect(DIVA_PREVIEW_SECTIONS).not.toContain('sandbox');
    expect(DIVA_ADDITIONAL_SECTIONS).toEqual([
      'self-evolution',
    ]);
  });
});