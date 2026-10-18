import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig, devices } from '@playwright/test';

// BML closeout UI acceptance: one real packed backend plus the split Vite dev
// origin, never the embedded :8787 face. The packed binary carries the sealed
// generation identity module actions are authorized against; `go run` would
// not. The backend is spawned inside the spec because restart persistence
// needs a real process restart; Playwright owns only Vite.
const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '..');
const runId = process.env.VIVY_BML_E2E_RUN?.trim() || 'run';
const workdir = path.resolve(repoRoot, '.workspace', 'issue81-bml-e2e', runId);
const packRoot = path.join(workdir, 'pack');
const memoryConfigPath = path.join(workdir, 'config.yaml');

export const BACKEND = process.env.VIVY_BML_E2E_BACKEND_ADDR?.trim() || '127.0.0.1:8806';
export const UI_PORT = Number(process.env.VIVY_BML_E2E_UI_PORT?.trim() || 3015);
export const memoryBins = {
  selected: path.join(packRoot, 'vivy'),
};
export const memoryRunRoot = workdir;
export const memoryRunConfig = memoryConfigPath;

/** Pack the default recipe once per candidate and report the sealed identity. */
function packOnce(): void {
  const binary = memoryBins.selected;
  if (fs.existsSync(binary) && !process.env.VIVY_BML_E2E_REPACK) {
    console.log(`memory e2e: reusing packed binary ${binary}`);
  } else {
    if (!fs.existsSync(path.join(repoRoot, 'ui', 'dist', 'index.html'))) {
      throw new Error('memory e2e: ui/dist is missing; run `pnpm --dir ui build` before the pack');
    }
    const goExe = process.env.GO_EXE || 'go';
    fs.rmSync(packRoot, { recursive: true, force: true });
    execFileSync(goExe, ['run', './sdk', 'pack', '--recipe', path.join(repoRoot, 'recipes', 'default.vivy.yml'), '--output', packRoot], {
      cwd: repoRoot,
      stdio: 'inherit',
      env: process.env,
    });
  }
  const digest = createHash('sha256').update(fs.readFileSync(binary)).digest('hex');
  console.log(`memory e2e: packed binary sha256=${digest}`);
}

/** One disposable backend home per run; no tenant or daily data is touched. */
function prepareRunRoot(): void {
  if (!process.env.VIVY_BML_E2E_KEEP) {
    for (const child of ['data', 'state', 'workspace', 'test-results']) {
      fs.rmSync(path.join(workdir, child), { recursive: true, force: true });
    }
  }
  for (const child of ['data', 'state', 'workspace']) {
    fs.mkdirSync(path.join(workdir, child), { recursive: true });
  }
  const posix = (value: string) => value.replace(/\\/g, '/');
  fs.writeFileSync(
    memoryConfigPath,
    [
      'server:',
      `  addr: "${BACKEND}"`,
      '  allowed_origins: []',
      'storage:',
      '  backend: sqlite',
      `  data_dir: "${posix(path.join(workdir, 'data'))}"`,
      '  sqlite:',
      `    path: "${posix(path.join(workdir, 'state', 'e2e.db'))}"`,
      'providers:',
      '  active: deepseek',
      // Module actions follow normal policy; only full_auto grants writes
      // without an approval route (module actions have no approval row).
      'governance:',
      '  profile: full_auto',
      'runtime:',
      `  workspace_root: "${posix(path.join(workdir, 'workspace'))}"`,
      '  stream_buffer: 256',
      '  max_event_payload_bytes: 65536',
      '',
    ].join('\n'),
    'utf8',
  );
}

if (!process.env.VIVY_BML_E2E_PREPARED) {
  packOnce();
  prepareRunRoot();
  process.env.VIVY_BML_E2E_PREPARED = '1';
}

export default defineConfig({
  testDir: './e2e',
  testMatch: ['memory.spec.ts'],
  timeout: 180_000,
  workers: 1,
  retries: 0,
  reporter: 'list',
  outputDir: path.join(workdir, 'test-results'),
  use: {
    baseURL: `http://127.0.0.1:${UI_PORT}`,
    locale: 'en-US',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  // No Playwright browser is downloaded on this host; the system Chrome is
  // driven explicitly. The split Vite origin is the only UI under test.
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'], channel: 'chrome' } }],
  webServer: [
    {
      command: 'pnpm dev',
      cwd: here,
      url: `http://127.0.0.1:${UI_PORT}`,
      reuseExistingServer: false,
      timeout: 180_000,
      env: {
        ...process.env,
        VIVY_UI_PORT: String(UI_PORT),
        VIVY_BACKEND_ADDR: `http://${BACKEND}`,
      },
    },
  ],
});
