import { readdirSync, readFileSync } from 'node:fs';
import { join, relative, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

// Token ratchet: colour, type size and colour literals belong to the semantic
// token contract in styles.css, not to call sites. Counts and the violating file
// set may only shrink. A new violating file fails even when the total stays the
// same. To lower the baseline, convert the file and delete its entries below.
// Studio is a separate submodule and is checked by its own token check.

const THIS_DIR = fileURLToPath(new URL('.', import.meta.url)); // ui/src/theme
const SRC_ROOT = resolve(THIS_DIR, '..'); // ui/src
const REPO_ROOT = resolve(THIS_DIR, '..', '..', '..');
const SKIP_DIRS = new Set(['node_modules', 'dist', 'generated', '.git', 'test-results', 'coverage']);

const PATTERNS = {
  palette:
    /\b(?:bg|text|border|ring|from|to|via|fill|stroke|divide|outline|shadow|accent|caret|decoration|placeholder)-(?:(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-\d{2,3}|white|black)\b/g,
  textSize: /\btext-\[\d+(?:\.\d+)?(?:px|rem|em)\]/g,
  hex: /#[0-9a-fA-F]{3,8}\b/g,
} as const;

type Category = keyof typeof PATTERNS;
type Group = 'src' | 'plugins';
type Baseline = { count: number; files: string[] };

const BASELINE: Record<Group, Record<Category, Baseline>> = {
  src: {
    palette: {
      count: 0,
      files: [],
    },
    textSize: {
      count: 0,
      files: [],
    },
    hex: {
      count: 0,
      files: [],
    },
  },
  plugins: {
    palette: {
      count: 0,
      files: [],
    },
    textSize: {
      count: 0,
      files: [],
    },
    hex: { count: 0, files: [] },
  },
};

function sourceFiles(root: string, keep: (path: string) => boolean): string[] {
  const files: string[] = [];
  const visit = (dir: string) => {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      if (SKIP_DIRS.has(entry.name)) continue;
      const full = join(dir, entry.name);
      if (entry.isDirectory()) visit(full);
      else if (/\.(ts|tsx)$/.test(entry.name) && !/\.test\./.test(entry.name) && keep(full)) files.push(full);
    }
  };
  visit(root);
  return files;
}

function measure(files: string[], category: Category): { count: number; offenders: Map<string, number> } {
  const pattern = new RegExp(PATTERNS[category].source, 'g');
  let count = 0;
  const offenders = new Map<string, number>();
  for (const file of files) {
    const matches = readFileSync(file, 'utf8').match(pattern);
    if (!matches) continue;
    count += matches.length;
    offenders.set(relative(REPO_ROOT, file), matches.length);
  }
  return { count, offenders };
}

const GROUPS: Record<Group, string[]> = {
  src: sourceFiles(SRC_ROOT, () => true),
  plugins: sourceFiles(join(REPO_ROOT, 'plugins'), (path) => path.split(sep).includes('ui')),
};

describe('token ratchet', () => {
  it('patterns still detect known offenders', () => {
    const sample = 'bg-red-500 dark:text-amber-400 text-[10px] #fff';
    expect(sample.match(PATTERNS.palette)).toEqual(['bg-red-500', 'text-amber-400']);
    expect(sample.match(PATTERNS.textSize)).toEqual(['text-[10px]']);
    expect(sample.match(PATTERNS.hex)).toEqual(['#fff']);
  });

  for (const group of Object.keys(GROUPS) as Group[]) {
    describe(group, () => {
      for (const category of Object.keys(PATTERNS) as Category[]) {
        it(`${category} does not grow`, () => {
          const baseline = BASELINE[group][category];
          const { count, offenders } = measure(GROUPS[group], category);
          const allowed = new Set(baseline.files);
          const added = [...offenders.keys()].filter((file) => !allowed.has(file)).sort();
          const stale = baseline.files.filter((file) => !offenders.has(file)).sort();

          console.info(
            `[token-ratchet] ${group}/${category}: ${count} (baseline ${baseline.count}); ` +
              `${offenders.size} files (baseline ${baseline.files.length})`,
          );

          expect(count, `token-ratchet ${group}/${category} grew beyond ${baseline.count}`).toBeLessThanOrEqual(
            baseline.count,
          );
          expect(added, `token-ratchet ${group}/${category} newly violated by these files`).toEqual([]);
          expect(stale, `token-ratchet ${group}/${category} baseline is stale for these files; delete them`).toEqual(
            [],
          );
        });
      }
    });
  }

  // Ink roles: `-fg` is the ink for a *solid* tone; a soft tint takes the tone
  // itself. `brand` is the accent mark, so an inked brand surface is
  // `brand-fill`, never `brand`. Both pairings render text at ~1.2:1 when
  // mistaken, so they fail here instead of surviving to a screenshot.
  const SOFT_WITH_SOLID_INK = /\bbg-(ok|warn|danger|info|brand)-soft\b/;
  const SOLID_INK = /\btext-(ok|warn|danger|info|brand)-fg\b/;
  const BRAND_MARK = /\bbg-brand(?![-\w])/;

  for (const group of Object.keys(GROUPS) as Group[]) {
    it(`${group}: ink matches the surface it is drawn on`, () => {
      const offenders: string[] = [];
      for (const file of GROUPS[group]) {
        readFileSync(file, 'utf8').split('\n').forEach((line, index) => {
          const soft = line.match(SOFT_WITH_SOLID_INK);
          const ink = line.match(SOLID_INK);
          if (soft && ink && soft[1] === ink[1]) {
            offenders.push(`${relative(REPO_ROOT, file)}:${index + 1} ${soft[0]} + ${ink[0]}`);
          }
          if (BRAND_MARK.test(line) && line.includes('text-brand-fg')) {
            offenders.push(`${relative(REPO_ROOT, file)}:${index + 1} bg-brand + text-brand-fg (use bg-brand-fill)`);
          }
        });
      }
      expect(offenders).toEqual([]);
    });
  }
});
