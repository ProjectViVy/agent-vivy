import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '..');
const scratch = path.join(root, 'work', 'queue-browser');
const config = path.join(scratch, 'config.yaml');
if (!process.env.VIVY_QUEUE_E2E_PREPARED) {
  fs.rmSync(scratch, { recursive: true, force: true });
  fs.mkdirSync(scratch, { recursive: true });
  fs.writeFileSync(config, [
    'server:', '  addr: "127.0.0.1:8787"',
    'storage:', '  backend: sqlite', `  data_dir: ${JSON.stringify(path.join(scratch, 'home'))}`,
    '  sqlite:', `    path: ${JSON.stringify(path.join(scratch, 'home', 'vivy.db'))}`,
    'providers:', '  active: deepseek', 'runtime:',
    `  workspace_root: ${JSON.stringify(path.join(scratch, 'workspace'))}`,
    'governance:', '  profile: default', '  profiles:', '    default:',
    '      rules:', '        - tool: diva.cognitive.persona.initialize',
    '          decision: allow', '          reason: Synthetic queue smoke initialization', '',
  ].join('\n'));
  process.env.VIVY_QUEUE_E2E_PREPARED = '1';
}

export default defineConfig({
  testDir: './e2e', testMatch: 'queue-options.spec.ts', workers: 1,
  timeout: 90_000, retries: 0, reporter: 'list',
  use: { baseURL: 'http://127.0.0.1:3015', locale: 'en-US',
    ...(process.env.PLAYWRIGHT_CHROMIUM ? { launchOptions: { executablePath: process.env.PLAYWRIGHT_CHROMIUM } } : {}),
  },
  webServer: [
    { command: `${JSON.stringify(process.env.GO_EXE || 'go')} run -tags vivy_headless ./cmd/vivy`, cwd: root,
      url: 'http://127.0.0.1:8787/healthz', timeout: 180_000, reuseExistingServer: false,
      env: { ...process.env, VIVY_CONFIG: config, VIVY_USER_HOME: path.join(scratch, 'home'),
        VIVY_PROVIDER: 'deepseek', DEEPSEEK_API_KEY: 'queue-loopback-dummy', VIVY_API_BASE: 'http://127.0.0.1:8797' } },
    { command: 'pnpm dev', cwd: here, url: 'http://127.0.0.1:3015', timeout: 120_000,
      reuseExistingServer: false, env: { ...process.env, VIVY_BACKEND_ADDR: 'http://127.0.0.1:8787' } },
  ],
});
