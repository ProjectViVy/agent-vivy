import fs from 'node:fs';
import path from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '..');
const workdir = fs.mkdtempSync(path.join(tmpdir(), 'vivy-toolkits-e2e-runtime-'));
const backendPort = portFromEnv('VIVY_TOOLKITS_E2E_BACKEND_PORT', 18787);
const uiPort = portFromEnv('VIVY_TOOLKITS_E2E_UI_PORT', 3015);
if (backendPort === uiPort) {
  throw new Error('Toolkit E2E backend and UI ports must differ; set VIVY_TOOLKITS_E2E_BACKEND_PORT and VIVY_TOOLKITS_E2E_UI_PORT.');
}
const backend = `127.0.0.1:${backendPort}`;
const ui = `127.0.0.1:${uiPort}`;
const externalBackend = process.env.VIVY_TOOLKITS_E2E_EXTERNAL_BACKEND === '1';
const binaryName = process.platform === 'win32' ? 'vivy.exe' : 'vivy';
const binary = process.env.VIVY_TOOLKITS_E2E_BINARY ||
  path.join(repoRoot, '.workspace', 'toolkits-e2e', 'gate', 'default', binaryName);
const pnpm = process.platform === 'win32' ? 'pnpm.cmd' : 'pnpm';

if (!fs.existsSync(binary)) {
  throw new Error(`Toolkit E2E needs a packed Vivy binary at ${binary}; run \`node scripts/toolkits-e2e.mjs\` (just toolkits-e2e) or set VIVY_TOOLKITS_E2E_BINARY after an explicit SDK pack.`);
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
      command: `"${binary}"`,
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
        VIVY_UI_PORT: String(uiPort),
        VIVY_BACKEND_ADDR: `http://${backend}`,
      },
      reuseExistingServer: externalBackend,
      timeout: 120_000,
    },
  ],
});

function portFromEnv(name: string, fallback: number): number {
  const raw = process.env[name];
  if (raw === undefined || raw === '') return fallback;
  if (!/^\d+$/.test(raw)) {
    throw new Error(`${name} must be an integer between 1 and 65535, got ${JSON.stringify(raw)}.`);
  }
  const port = Number(raw);
  if (port < 1 || port > 65535) {
    throw new Error(`${name} must be between 1 and 65535, got ${port}.`);
  }
  return port;
}
