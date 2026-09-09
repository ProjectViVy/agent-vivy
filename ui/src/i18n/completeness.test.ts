import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { resolve, join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { afterEach, describe, expect, it } from 'vitest';

const roots: string[] = [];
afterEach(() => { for (const root of roots.splice(0)) rmSync(root, { recursive: true }); });

function check(en: string, zh: string, source = '') {
  const root = mkdtempSync(join(tmpdir(), 'vivy-i18n-check-'));
  roots.push(root);
  mkdirSync(join(root, 'ui/src/i18n'), { recursive: true });
  writeFileSync(join(root, 'ui/src/i18n/en.ts'), `export const en = ${en};`);
  writeFileSync(join(root, 'ui/src/i18n/zh.ts'), `export const zh = ${zh};`);
  writeFileSync(join(root, 'ui/src/surface.tsx'), source);
  const result = spawnSync(process.execPath, [resolve('../scripts/check-i18n-completeness.js'), '--root', root], { encoding: 'utf8' });
  return { status: result.status, output: result.stdout + result.stderr };
}

describe('catalog completeness command', () => {
  it('accepts nested arrays and ignores comments, not runtime strings', () => {
    expect(check("{label: ['Hi {{name}}']}", "{label: ['你好 {{name}}']}", '// 中文注释\nconst id = "protocol-id";').status).toBe(0);
  });
  it('rejects missing keys', () => {
    const result = check("{a: 'A', b: 'B'}", "{a: '甲'}");
    expect(result.status).toBe(1);
    expect(result.output).toContain('Missing in zh: b');
  });
  it.each(['en', 'zh'])('rejects an extra own prototype-named key in %s', (locale) => {
    const base = "{ready: 'Ready'}";
    const extra = "{ready: 'Ready', toString: 'String label'}";
    const result = check(locale === 'en' ? extra : base, locale === 'zh' ? extra : base);
    expect(result.status).toBe(1);
    expect(result.output).toContain(`Missing in ${locale === 'en' ? 'zh' : 'en'}: toString`);
  });
  it('rejects a static translation reference that only exists on the prototype', () => {
    const result = check("{ready: 'Ready'}", "{ready: '就绪'}", 't("toString");');
    expect(result.status).toBe(1);
    expect(result.output).toContain('Unknown translation: toString');
  });
  it('accepts a prototype-named translation when both catalogs own the key', () => {
    expect(check("{toString: 'String label'}", "{toString: '字符串标签'}", 't("toString");').status).toBe(0);
  });
  it('preserves own __proto__ keys while flattening catalogs', () => {
    const result = check("{ready: 'Ready'}", "Object.defineProperty({ready: '就绪'}, '__proto__', {value: '原型', enumerable: true})");
    expect(result.status).toBe(1);
    expect(result.output).toContain('Missing in en: __proto__');
  });
  it('rejects mismatched named placeholders inside arrays', () => {
    const result = check("{a: ['{{count}}']}", "{a: ['{{name}}']}");
    expect(result.status).toBe(1);
    expect(result.output).toContain('Placeholder mismatch: a.0');
  });
  it.each(['const label = "保存";', 'const el = <button>保存</button>;', 'const label = `保存 ${count} 项`;'])('rejects runtime Han copy: %s', (source) => {
    const result = check("{a: 'A'}", "{a: '甲'}", source);
    expect(result.status).toBe(1);
    expect(result.output).toContain('Uncatalogued Han: ui/src/surface.tsx');
  });
  it('rejects direct English JSX chrome and missing translation references', () => {
    const result = check("{a: 'A'}", "{a: '甲'}", 'const el = <button title="Save now">Save</button>; t("missing.key");');
    expect(result.status).toBe(1);
    expect(result.output).toContain('Uncatalogued JSX');
    expect(result.output).toContain('Unknown translation: missing.key');
  });
  it('rejects English copy hidden in JSX string expressions', () => {
    const result = check("{a: 'A'}", "{a: '甲'}", 'const el = <button aria-label={"Save changes"}>{"Save"}</button>;');
    expect(result.status).toBe(1);
    expect(result.output).toContain('Uncatalogued JSX');
  });
  it.each(["{a: ''}", "{a: 7}", "{a: 'A', a: 'B'}"])('rejects invalid dictionary leaves or duplicate keys: %s', (dictionary) => {
    expect(check(dictionary, dictionary).status).toBe(1);
  });
});
