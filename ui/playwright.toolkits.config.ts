import fs from 'node:fs';
import path from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '..');
const workdir = fs.mkdtempSync(path.join(tmpdir(), 'vivy-toolkits-e2e-runtime-'));
const backend = '127.0.0.1:18787';
const ui = '127.0.0.1:3015';
const externalBackend = process.env.VIVY_TOOLKITS_E2E_EXTERNAL_BACKEND === '1';
const binary = process.env.VIVY_TOOLKITS_E2E_BINARY ||
  path.join(repoRoot, '.workspace', 'toolkits-e2e', 'default', 'vivy');

if (!fs.existsSync(binary)) {
  throw new Error(`Toolkit E2E needs a packed Vivy binary at ${binary}; set VIVY_TOOLKITS_E2E_BINARY after running SDK pack.`);
}

// Every config evaluation gets a fresh root, so parallel Playwright workers
// cannot remove or reuse a database while the backend has it open.
fs.mkdirSync(path.join(workdir, 'state'), { recursive: true });
const quote = (value: string) => JSON.stringify(value.replaceAll('\\', '/'));
const config = path.join(workdir, 'config.yaml');
fs.writeFileSync(config, [
  'server:',
  `  addr: ${quote(backend)}`,
  '  allowed_origins:',
  `    - ${quote(`http://${ui}`)}`,
  'storage:',
  '  backend: sqlite',
  `  data_dir: ${quote(path.join(workdir, 'state'))}`,
  '  sqlite:',
  `    path: ${quote(path.join(workdir, 'state', 'toolkits.db'))}`,
  'providers:',
  '  active: deepseek',
  'runtime:',
  `  workspace_root: ${quote(path.join(workdir, 'workspace'))}`,
  `  skills_root: ${quote(path.join(workdir, 'skills'))}`,
  '',
].join('\n'), 'utf8');

export default defineConfig({
  testDir: './e2e',
  testMatch: 'toolkits.spec.ts',
  timeout: 90_000,
  workers: 1,
  retries: 0,
  reporter: 'list',
  use: {
    baseURL: `http://${ui}`,
    locale: 'en-US',
    channel: 'chrome',
  },
  webServer: [
    ...(!externalBackend ? [{
      command: JSON.stringify(binary),
      cwd: repoRoot,
      url: `http://${backend}/healthz`,
      stdout: 'pipe',
      env: { ...process.env, VIVY_CONFIG: config },
      reuseExistingServer: false,
      timeout: 120_000,
    }] : []),
    {
      command: 'pnpm dev',
      cwd: here,
      url: `http://${ui}`,
      env: {
        ...process.env,
        GOFLAGS: '-tags=http2legacy',
        VIVY_BACKEND_ADDR: `http://${backend}`,
      },
      reuseExistingServer: externalBackend,
      timeout: 120_000,
    },
  ],
});
