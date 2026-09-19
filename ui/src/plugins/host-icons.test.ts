import { describe, expect, it } from 'vitest';
import { HOST_ICON_NAMES } from '@vivy/ui-sdk';
import { HOST_ICONS, hostIconNames, resolveHostIcon } from './host-icons';

describe('host icon set', () => {
  it('implements every name the SDK publishes, so an entry icon can never dangle', () => {
    for (const name of HOST_ICON_NAMES) {
      expect(HOST_ICONS[name], `missing host icon for ${name}`).toBeTypeOf('object');
      expect(hostIconNames()).toContain(name);
    }
  });

  it('resolves a known name to its own implementation and leaves nothing undefined', () => {
    for (const name of HOST_ICON_NAMES) {
      const Icon = resolveHostIcon(name);
      expect(Icon, name).toBe(HOST_ICONS[name]);
      expect(Icon.displayName ?? Icon.name ?? 'component', name).toBeTruthy();
    }
  });

  it('falls back to the group icon for a name this host does not implement', () => {
    expect(resolveHostIcon('not-an-icon')).toBe(HOST_ICONS.sparkles);
    expect(resolveHostIcon(undefined)).toBe(HOST_ICONS.sparkles);
  });
});