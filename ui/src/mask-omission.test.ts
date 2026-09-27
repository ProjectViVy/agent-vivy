import { describe, expect, it } from 'vitest';
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { fileURLToPath, URL } from 'node:url';

/**
 * MASK-4 omission assertions: after the placeholder mask authority was removed
 * the shell must carry no static mask route, component, locale entry, or
 * localStorage authority. Masks reach the UI only through the vivy/masks-ui
 * Module staged under src/generated/.
 */

const uiRoot = fileURLToPath(new URL('..', import.meta.url));
const srcRoot = fileURLToPath(new URL('.', import.meta.url));

function listSources(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = `${dir}/${entry.name}`;
    if (entry.isDirectory()) {
      if (entry.name === 'generated') continue;
      out.push(...listSources(path));
    } else if (/\.(ts|tsx)$/.test(entry.name) && !entry.name.endsWith('.test.ts') && !entry.name.endsWith('.test.tsx')) {
      out.push(path);
    }
  }
  return out;
}

describe('mask omission', () => {
  it('ships no shell mask route or components', () => {
    expect(existsSync(`${srcRoot}/routes/_layout.masks.tsx`)).toBe(false);
    expect(existsSync(`${srcRoot}/components/masks`)).toBe(false);
  });

  it('carries no core mask locale keys', () => {
    const en = readFileSync(`${srcRoot}/i18n/en.ts`, 'utf8');
    const zh = readFileSync(`${srcRoot}/i18n/zh.ts`, 'utf8');
    for (const source of [en, zh]) {
      expect(source).not.toMatch(/nav:\s*{[^}]*masks/);
      expect(source).not.toMatch(/\bmasks:\s*{/);
      expect(source).not.toMatch(/maskSwitcher/);
    }
  });

  it('keeps no localStorage mask authority or static mask imports', () => {
    for (const path of listSources(srcRoot)) {
      const source = readFileSync(path, 'utf8');
      expect(source, path).not.toContain('vivy.ui.activeMask');
      expect(source, path).not.toMatch(/from ['"].*components\/masks/);
    }
  });

  it('renders the masks surface only through the staged module extension', () => {
    const assembly = readFileSync(`${srcRoot}/generated/assembly.ts`, 'utf8');
    expect(assembly).toContain('vivy.masks-ui');
    const sidebar = readFileSync(`${srcRoot}/components/chat/ConversationSidebar.tsx`, 'utf8');
    expect(sidebar).not.toMatch(/masks/i);
  });
});
