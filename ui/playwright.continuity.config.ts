import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';
import {
  CONTINUITY_ADDR,
  MODEL_ADDR,
  UI_ADDR,
  continuityConfig,
  prepareContinuityWorkdir,
} from './e2e/continuity-setup';

if (!process.env.VIVY_CONTINUITY_E2E_PREPARED) {
  prepareContinuityWorkdir();
  process.env.VIVY_CONTINUITY_E2E_PREPARED = '1';
}

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
// The existing playwright.config.ts pins a Windows Go path; this suite runs
// on Linux CI/dev boxes, so the Go executable is overridable via GO_EXE.
const goExe = process.env.GO_EXE || 'go';

export default defineConfig({
  testDir: './e2e',
  testMatch: 'session-continuity.spec.ts',
  timeout: 90_000,
  workers: 1,
  retries: 0,
  reporter: 'list',
  use: { baseURL: `http://${UI_ADDR}`, locale: 'zh-CN' },
  webServer: [
    {
      command: `${JSON.stringify(goExe)} run ./cmd/vivy`,
      cwd: repoRoot,
      url: `http://${CONTINUITY_ADDR}/healthz`,
      env: {
        ...process.env,
        VIVY_ADDR: CONTINUITY_ADDR,
        VIVY_CONFIG: continuityConfig,
        // Deterministic loopback model endpoint; the credential is a dummy
        // because the mock never authenticates.
        VIVY_PROVIDER: 'deepseek',
        DEEPSEEK_API_KEY: 'continuity-e2e-dummy',
        VIVY_API_BASE: `http://${MODEL_ADDR}`,
      },
      reuseExistingServer: false,
      timeout: 180_000,
    },
    {
      command: 'pnpm dev --host 127.0.0.1 --strictPort',
      cwd: path.dirname(fileURLToPath(import.meta.url)),
      url: `http://${UI_ADDR}`,
      env: { ...process.env, VIVY_BACKEND_ADDR: `http://${CONTINUITY_ADDR}` },
      reuseExistingServer: false,
      timeout: 120_000,
    },
  ],
});
