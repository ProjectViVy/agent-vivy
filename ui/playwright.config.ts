import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';
import { E2E_ADDR, e2eConfig, prepareE2eWorkdir } from './e2e/global-setup';

if (!process.env.VIVY_E2E_PREPARED) { prepareE2eWorkdir(); process.env.VIVY_E2E_PREPARED = '1'; }
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const goExe = process.env.GO_EXE || 'C:\\Program Files\\Go\\bin\\go.exe';
export default defineConfig({
  testDir: './e2e', timeout: 60_000, workers: 1, retries: 0, reporter: 'list', use: { baseURL: `http://${E2E_ADDR}`, locale: 'zh-CN' },
  webServer: { command: `${JSON.stringify(goExe)} run ./cmd/vivy`, cwd: repoRoot, url: `http://${E2E_ADDR}/healthz`, env: { ...process.env, VIVY_ADDR: E2E_ADDR, VIVY_CONFIG: e2eConfig }, reuseExistingServer: false, timeout: 30_000 },
});
