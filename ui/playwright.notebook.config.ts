import { execSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';

// Notebook N3 task 3: real disposable backend + Vite :3015 split pair.
// The backend is spawned per-test inside the spec (restart persistence needs a
// real process restart, which webServer cannot express); Playwright owns only
// the Vite dev server. Packed binaries carry the sealed generation identity
// module actions are authorized against; `go run` would not.
const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '..');
const workdir = path.join(here, '.e2e-notebook');
const nbConfig = path.join(workdir, 'config.yaml');
const packRoot = path.join(repoRoot, '.workspace', 'notebook-e2e');
export const BACKEND = '127.0.0.1:8797';
const UI = '127.0.0.1:3015';
const goExe = process.env.GO_EXE || 'go';

export const nbWorkdir = workdir;
export const nbConfigPath = nbConfig;
export const notebookBins = {
  selected: path.join(packRoot, 'default', 'vivy'),
  omitted: path.join(packRoot, 'no-notebook', 'vivy'),
  noReports: path.join(packRoot, 'no-reports', 'vivy'),
};

function packOnce(recipe: string, output: string): void {
  const binary = path.join(output, 'vivy');
  if (fs.existsSync(binary) && !process.env.VIVY_NB_E2E_REPACK) return;
  execSync(`${JSON.stringify(goExe)} run ./sdk pack --recipe ${JSON.stringify(recipe)} --output ${JSON.stringify(output)}`, {
    cwd: repoRoot,
    stdio: 'inherit',
    env: process.env,
  });
}

if (!process.env.VIVY_NB_E2E_PREPARED) {
  packOnce(path.join(repoRoot, 'recipes', 'default.vivy.yml'), path.join(packRoot, 'default'));
  packOnce(path.join(repoRoot, 'recipes', 'no-notebook.vivy.yml'), path.join(packRoot, 'no-notebook'));
  packOnce(path.join(repoRoot, 'recipes', 'no-reports.vivy.yml'), path.join(packRoot, 'no-reports'));
  fs.rmSync(workdir, { recursive: true, force: true });
  fs.mkdirSync(path.join(workdir, 'state'), { recursive: true });
  const dbPath = path.join(workdir, 'state', 'e2e.db').replace(/\\/g, '/');
  const workspaceRoot = path.join(workdir, 'workspace').replace(/\\/g, '/');
  fs.writeFileSync(
    nbConfig,
    [
      'server:',
      `  addr: "${BACKEND}"`,
      '  allowed_origins: []',
      'storage:',
      '  backend: sqlite',
      `  data_dir: "${workdir.replace(/\\/g, '/')}"`,
      '  sqlite:',
      `    path: "${dbPath}"`,
      'providers:',
      '  active: deepseek',
      // Module actions follow normal policy; only full_auto grants writes
      // without an approval route (module actions have no approval row).
      'governance:',
      '  profile: full_auto',
      'runtime:',
      `  workspace_root: "${workspaceRoot}"`,
      '  stream_buffer: 256',
      '  max_event_payload_bytes: 65536',
      'tools:',
      '  enabled:',
      '    - echo_info',
      '    - write_note',
      '    - ask_user',
      '  approval:',
      '    expiration: 5m',
      '',
    ].join('\n'),
    'utf8',
  );
  process.env.VIVY_NB_E2E_PREPARED = '1';
}

export default defineConfig({
  testDir: './e2e',
  testMatch: ['notebook.spec.ts', 'notebook-reports.spec.ts'],
  timeout: 120_000,
  workers: 1,
  retries: 0,
  reporter: 'list',
  use: { baseURL: `http://${UI}`, locale: 'zh-CN' },
  webServer: [
    {
      command: 'pnpm dev',
      cwd: here,
      url: `http://${UI}`,
      env: { ...process.env, VIVY_BACKEND_ADDR: `http://${BACKEND}` },
      reuseExistingServer: false,
      timeout: 120_000,
    },
  ],
});
