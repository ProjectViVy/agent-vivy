import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';
import { CONTINUITY_ADDR, MODEL_ADDR, UI_ADDR } from './e2e/continuity-setup';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const artifact = process.env.VIVY_MINIMAL_CHAT_ARTIFACT;
const assembly = process.env.VIVY_UI_ASSEMBLY_ENTRY;
if (!artifact || !assembly) throw new Error('Pack minimal-chat and stage its UI before running this suite');
const home = process.env.VIVY_MINIMAL_E2E_HOME ?? fs.mkdtempSync(path.join(os.tmpdir(), 'vivy-minimal-chat-'));
process.env.VIVY_MINIMAL_E2E_HOME = home;
fs.mkdirSync(path.join(home, 'workspace'), { recursive: true });
const config = path.join(home, 'config.json');
fs.writeFileSync(config, JSON.stringify({
  server: { addr: CONTINUITY_ADDR },
  storage: { backend: 'sqlite', sqlite: { path: path.join(home, 'vivy.db') } },
  runtime: { workspace_root: path.join(home, 'workspaces'), cache_warming: 'off' },
  tools: { enabled: ['read_file', 'list_dir'] },
}));

export default defineConfig({
  testDir: './e2e', testMatch: 'minimal-chat.spec.ts', workers: 1, retries: 0,
  timeout: 60_000, reporter: 'list',
  use: { baseURL: `http://${UI_ADDR}`, locale: 'en-US', screenshot: 'only-on-failure' },
  webServer: [
    { command: JSON.stringify(path.join(artifact, 'vivy')), cwd: repoRoot,
      url: `http://${CONTINUITY_ADDR}/healthz`, reuseExistingServer: false, timeout: 60_000,
      env: { ...process.env, VIVY_CONFIG: config, VIVY_USER_HOME: path.join(home, 'home'),
        VIVY_ADDR: CONTINUITY_ADDR, VIVY_PROVIDER: 'deepseek', VIVY_MODEL: 'deepseek-flash',
        DEEPSEEK_API_KEY: 'minimal-e2e-dummy', VIVY_API_BASE: `http://${MODEL_ADDR}/v1` } },
    { command: 'pnpm dev --host 127.0.0.1 --strictPort', cwd: path.join(repoRoot, 'ui'),
      url: `http://${UI_ADDR}`, reuseExistingServer: false, timeout: 120_000,
      env: { ...process.env, VIVY_BACKEND_ADDR: `http://${CONTINUITY_ADDR}`, VIVY_UI_ASSEMBLY_ENTRY: assembly } },
  ],
});
