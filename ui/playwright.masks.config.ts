import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';

// Split-pair mask acceptance (MASK-4 task 3): disposable backend on :8787,
// hanging mock provider on :9911, Vite dev server on :3015. The default
// playwright.config.ts targets the embedded-server smoke path instead; this
// config is the separate recipe-selected artifact e2e.
const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '..');
const workdir = path.join(here, '.e2e-masks-workdir');
const maskConfig = path.join(workdir, 'config.yaml');
const BACKEND = '127.0.0.1:8787';
const MOCK_PROVIDER = '127.0.0.1:9911';
const UI = '127.0.0.1:3015';
// Masks require a sealed generation identity, which only a packed binary
// carries (go run/build of cmd/vivy embeds no manifest). Pack once with:
//   go run ./sdk pack --recipe recipes/masks-selected.vivy.yml --output .workspace/mask-verify/selected
const packedBinary =
  process.env.VIVY_MASKS_E2E_BINARY ||
  path.join(repoRoot, '.workspace', 'mask-verify', 'selected', 'vivy');

export const mockProviderBaseURL = `http://${MOCK_PROVIDER}/v1`;

if (!process.env.VIVY_MASKS_E2E_PREPARED) {
  fs.rmSync(workdir, { recursive: true, force: true });
  fs.mkdirSync(path.join(workdir, 'state'), { recursive: true });
  const dbPath = path.join(workdir, 'state', 'e2e.db').replace(/\\/g, '/');
  const workspaceRoot = path.join(workdir, 'workspace').replace(/\\/g, '/');
  fs.writeFileSync(
    maskConfig,
    [
      'server:',
      `  addr: "${BACKEND}"`,
      '  allowed_origins: []',
      'storage:',
      '  backend: sqlite',
      `  data_dir: "${workdir.replace(/\\/g, '/')}"`,
      '  sqlite:',
      `    path: "${dbPath}"`,
      'providers:',
      '  active: deepseek',
      // Mask write actions follow normal policy; only full_auto grants them
      // without an approval route (module actions have no approval row).
      'governance:',
      '  profile: full_auto',
      'runtime:',
      `  workspace_root: "${workspaceRoot}"`,
      '  stream_buffer: 256',
      '  max_event_payload_bytes: 65536',
      'tools:',
      '  enabled:',
      '    - echo_info',
      '    - write_note',
      '    - ask_user',
      '  approval:',
      '    expiration: 5m',
      '',
    ].join('\n'),
    'utf8',
  );
  process.env.VIVY_MASKS_E2E_PREPARED = '1';
}

export default defineConfig({
  testDir: './e2e',
  testMatch: 'masks.spec.ts',
  timeout: 90_000,
  workers: 1,
  retries: 0,
  reporter: 'list',
  use: { baseURL: `http://${UI}`, locale: 'zh-CN' },
  webServer: [
    {
      command: JSON.stringify(packedBinary),
      cwd: repoRoot,
      url: `http://${BACKEND}/healthz`,
      env: { ...process.env, VIVY_CONFIG: maskConfig },
      reuseExistingServer: false,
      timeout: 60_000,
    },
    {
      command: `node ${JSON.stringify(path.join(here, 'scripts', 'e2e-mock-provider.mjs'))}`,
      url: `http://${MOCK_PROVIDER}/healthz`,
      reuseExistingServer: false,
      timeout: 30_000,
    },
    {
      command: 'pnpm dev',
      cwd: here,
      url: `http://${UI}`,
      env: { ...process.env, VIVY_BACKEND_ADDR: `http://${BACKEND}` },
      reuseExistingServer: false,
      timeout: 120_000,
    },
  ],
});
