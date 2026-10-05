import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';
import { MODEL_ADDR } from './e2e/continuity-setup';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '..');
const workdir = path.join(repoRoot, '.workspace', 'composer-e2e');
const configFile = path.join(workdir, 'config.yaml');
const backend = '127.0.0.1:8787';

if (!process.env.VIVY_COMPOSER_E2E_PREPARED) {
  fs.rmSync(workdir, { recursive: true, force: true });
  const skillDir = path.join(workdir, 'skills', 'composer-review');
  fs.mkdirSync(skillDir, { recursive: true });
  fs.mkdirSync(path.join(workdir, 'workspace'), { recursive: true });
  fs.writeFileSync(path.join(skillDir, 'SKILL.md'), '---\nname: composer-review\ndescription: Review the composer change\n---\nCheck the composer request and report the result.\n');
  fs.writeFileSync(configFile, [
    'server:', `  addr: "${backend}"`,
    'storage:', '  backend: sqlite', `  data_dir: ${JSON.stringify(path.join(workdir, 'home'))}`, '  sqlite:', `    path: ${JSON.stringify(path.join(workdir, 'home', 'vivy.db'))}`,
    'providers:', '  active: deepseek',
    'runtime:', `  workspace_root: ${JSON.stringify(path.join(workdir, 'workspace'))}`, `  skills_root: ${JSON.stringify(path.join(workdir, 'skills'))}`,
    'tools:', '  enabled:', '    - enter_plan_mode', '    - submit_plan', '    - get_goal', '    - create_goal', '    - report_goal', '    - task_create', '    - task_update', '    - skills_list', '    - skill_view', '    - ask_user',
    'governance:', '  profile: full_auto', '',
  ].join('\n'));
  process.env.VIVY_COMPOSER_E2E_PREPARED = '1';
}

export default defineConfig({
  testDir: './e2e', testMatch: 'composer-commands.spec.ts', workers: 1, timeout: 90_000, retries: 0, reporter: 'list',
  use: { baseURL: 'http://127.0.0.1:3015', locale: 'en-US',
    ...(process.env.PLAYWRIGHT_CHROMIUM ? { launchOptions: { executablePath: process.env.PLAYWRIGHT_CHROMIUM } } : {}),
  },
  webServer: [
    { command: `${JSON.stringify(process.env.GO_EXE || 'go')} run ./cmd/vivy`, cwd: repoRoot, url: `http://${backend}/healthz`, timeout: 180_000, reuseExistingServer: false,
      env: { ...process.env, VIVY_CONFIG: configFile, VIVY_USER_HOME: path.join(workdir, 'home'), VIVY_PROVIDER: 'deepseek', DEEPSEEK_API_KEY: 'composer-e2e-dummy', VIVY_API_BASE: `http://${MODEL_ADDR}` } },
    { command: 'pnpm dev', cwd: here, url: 'http://127.0.0.1:3015', timeout: 120_000, reuseExistingServer: false,
      env: { ...process.env, VIVY_BACKEND_ADDR: `http://${backend}` } },
  ],
});
