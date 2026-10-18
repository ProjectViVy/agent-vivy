import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';
import {
  WORKFLOW_BACKEND_ADDR,
  WORKFLOW_MODEL_ADDR,
  WORKFLOW_UI_ADDR,
  prepareWorkflowWorkdir,
  workflowConfig,
} from './e2e/workflow-setup';

if (!process.env.VIVY_WORKFLOW_E2E_PREPARED) {
  prepareWorkflowWorkdir();
  process.env.VIVY_WORKFLOW_E2E_PREPARED = '1';
}

const uiRoot = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(uiRoot, '..');
const goExe = process.env.GO_EXE || 'go';

// The Vite dev server is started from its local binary instead of `pnpm dev`:
// the suite needs no install step, and the UI projection is staged explicitly
// by the gate (`node scripts/stage-ui-assembly.mjs`) before the run.
export default defineConfig({
  testDir: './e2e',
  testMatch: 'workflow-correctness.spec.ts',
  timeout: 120_000,
  workers: 1,
  retries: 0,
  reporter: 'list',
  // System Chrome, matching the toolkits suite: no Playwright browser download
  // is part of this gate.
  use: { baseURL: `http://${WORKFLOW_UI_ADDR}`, locale: 'en-US', channel: 'chrome' },
  webServer: [
    {
      command: `${JSON.stringify(goExe)} run ./cmd/vivy`,
      cwd: repoRoot,
      url: `http://${WORKFLOW_BACKEND_ADDR}/healthz`,
      env: {
        ...process.env,
        VIVY_ADDR: WORKFLOW_BACKEND_ADDR,
        VIVY_CONFIG: workflowConfig,
        VIVY_PROVIDER: 'deepseek',
        DEEPSEEK_API_KEY: 'workflow-e2e-dummy',
        VIVY_API_BASE: `http://${WORKFLOW_MODEL_ADDR}`,
      },
      reuseExistingServer: false,
      timeout: 180_000,
    },
    {
      command: `./node_modules/.bin/vite --host 127.0.0.1 --port ${WORKFLOW_UI_ADDR.split(':')[1]} --strictPort`,
      cwd: uiRoot,
      url: `http://${WORKFLOW_UI_ADDR}`,
      env: { ...process.env, VIVY_BACKEND_ADDR: `http://${WORKFLOW_BACKEND_ADDR}` },
      reuseExistingServer: false,
      timeout: 120_000,
    },
  ],
});
