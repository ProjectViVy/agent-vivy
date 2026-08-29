import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';
import { E2E_ADDR, E2E_STUB_ADDR, e2eConfig, e2eHome, prepareE2eWorkdir } from './e2e/global-setup';

if (!process.env.VIVY_E2E_PREPARED) { prepareE2eWorkdir(); process.env.VIVY_E2E_PREPARED = '1'; }
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const goExe = process.env.GO_EXE || 'C:\\Program Files\\Go\\bin\\go.exe';
export default defineConfig({
  testDir: './e2e', timeout: 60_000, workers: 1, retries: 0, reporter: 'list', use: { baseURL: `http://${E2E_ADDR}`, locale: 'zh-CN' },
  webServer: [
    { command: 'node e2e/openai-stub.mjs', cwd: path.join(repoRoot, 'ui'), url: `http://${E2E_STUB_ADDR}/healthz`, reuseExistingServer: false, timeout: 15_000 },
    {
      command: `${JSON.stringify(goExe)} run ./cmd/vivy`,
      cwd: repoRoot,
      url: `http://${E2E_ADDR}/healthz`,
      env: { VIVY_ADDR: E2E_ADDR, VIVY_CONFIG: e2eConfig, VIVY_USER_HOME: e2eHome },
      reuseExistingServer: false,
      timeout: 30_000,
    },
  ],
});
