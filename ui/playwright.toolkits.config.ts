import fs from 'node:fs';
import path from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';
import { resolveGateEndpoints } from './e2e/e2e-ports.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '..');
const workdir = fs.mkdtempSync(path.join(tmpdir(), 'vivy-toolkits-e2e-runtime-'));
const endpoints = resolveGateEndpoints({
  backendVariable: 'VIVY_TOOLKITS_E2E_BACKEND_PORT',
  backendDefault: 18787,
  uiVariable: 'VIVY_TOOLKITS_E2E_UI_PORT',
  uiDefault: 3015,
});
const backend = endpoints.backend;
const ui = endpoints.ui;
const externalBackend = process.env.VIVY_TOOLKITS_E2E_EXTERNAL_BACKEND === '1';
const binaryName = process.platform === 'win32' ? 'vivy.exe' : 'vivy';
const binary = process.env.VIVY_TOOLKITS_E2E_BINARY ||
  path.join(repoRoot, '.workspace', 'toolkits-e2e', 'default', binaryName);
const pnpm = process.platform === 'win32' ? 'pnpm.cmd' : 'pnpm';

if (!fs.existsSync(binary)) {
  throw new Error(
    `Toolkit E2E needs a packed Vivy binary at ${binary}; run \`node scripts/toolkits-e2e.mjs\` ` +
      '(just toolkits-e2e), or set VIVY_TOOLKITS_E2E_BINARY to an inspected artifact binary.',
  );
}

// Every config evaluation gets a fresh root, so parallel Playwright workers
// cannot remove or reuse a database while the backend has it open.
fs.mkdirSync(path.join(workdir, 'state'), { recursive: true });
const quote = (value: string) => JSON.stringify(value.replaceAll('\\', '/'));
const config = path.join(workdir, 'config.yaml');
console.log(`[toolkit-e2e] backend ${backend} ui ${ui} binary ${binary} data ${workdir}`);
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
      command: `${pnpm} dev`,
      cwd: here,
      url: `http://${ui}`,
      env: {
        ...process.env,
        GOFLAGS: process.env.GOFLAGS || '-tags=http2legacy',
        VIVY_UI_PORT: String(endpoints.uiPort),
        VIVY_BACKEND_ADDR: `http://${backend}`,
      },
      reuseExistingServer: externalBackend,
      timeout: 120_000,
    },
  ],
});
