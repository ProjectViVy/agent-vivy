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
      count: 29,
      files: [
        'plugins/vivy-masks-ui/ui/vivy-masks/src/MaskEditor.tsx',
        'plugins/vivy-masks-ui/ui/vivy-masks/src/MaskHeader.tsx',
        'plugins/vivy-masks-ui/ui/vivy-masks/src/MaskIdentity.tsx',
        'plugins/vivy-masks-ui/ui/vivy-masks/src/MaskPage.tsx',
        'plugins/vivy-persona/ui/vivy-persona/src/view.tsx',
        'plugins/vivy-workflow/ui/vivy-workflow/src/editor/EditorPane.tsx',
        'plugins/vivy-workflow/ui/vivy-workflow/src/panes/RunsPane.tsx',
      ],
    },
    textSize: {
      count: 45,
      files: [
        'plugins/vivy-masks-ui/ui/vivy-masks/src/MaskLibrary.tsx',
        'plugins/vivy-persona/ui/vivy-persona/src/view.tsx',
        'plugins/vivy-workflow/ui/vivy-workflow/src/WorkflowPage.tsx',
        'plugins/vivy-workflow/ui/vivy-workflow/src/editor/EditorPane.tsx',
        'plugins/vivy-workflow/ui/vivy-workflow/src/editor/NodePanel.tsx',
        'plugins/vivy-workflow/ui/vivy-workflow/src/editor/WorkflowNode.tsx',
        'plugins/vivy-workflow/ui/vivy-workflow/src/panes/RunsPane.tsx',
        'plugins/vivy-workflow/ui/vivy-workflow/src/panes/WorkflowsPane.tsx',
      ],
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
});
