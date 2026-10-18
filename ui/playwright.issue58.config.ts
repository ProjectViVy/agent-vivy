import path from 'node:path';
import {mkdtempSync} from 'node:fs';
import {tmpdir} from 'node:os';
import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';

// Dedicated supported split pair. The singleton producer exists only in the
// disposable Go test process; this never enables a production LIFE entry.
process.env.VIVY_ISSUE58_SPLIT_TEST = '1';
process.env.VIVY_ISSUE58_SPLIT_STOP_FILE ??= path.join(mkdtempSync(path.join(tmpdir(),'vivy-issue58-split-')),'stop');
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const go = process.env.GO_EXE || 'go';
const node = process.execPath;
// Playwright executes webServer commands through the platform shell. JSON
// escaping would double Windows path separators, so quote executable paths
// for that shell while keeping the fixed arguments below separate.
function executable(value: string): string {
 if (process.platform === 'win32') {
  if (/["%&|<>^!\r\n]/.test(value)) throw new Error('Executable path contains unsupported shell characters');
  return `"${value}"`;
 }
 return `'${value.replace(/'/g, "'\\''")}'`;
}
export default defineConfig({
 globalTeardown:'./e2e/issue58-global-teardown.ts',
 testDir: './e2e', testMatch: 'issue58-recap.spec.ts', timeout: 90_000,
 outputDir: './test-results/issue58-recap',
 workers: 1, retries: 0, reporter: 'list',
 use: { baseURL: 'http://127.0.0.1:3015', locale: 'en-US', screenshot: 'on', trace: 'retain-on-failure' },
 webServer: [
  {command: `${executable(go)} test -p=1 ./internal/app -run "^TestIssue58RecapSplitFixture$" -count=1 -v -timeout=18m`, cwd: repoRoot, url: 'http://127.0.0.1:8787/healthz', env: {...process.env, VIVY_ISSUE58_SPLIT_TEST:'1', VIVY_ISSUE58_SPLIT_SECONDS:'900'}, reuseExistingServer:false, timeout:120_000},
  {command: `${executable(node)} ./ui/scripts/stage-ui-assembly.mjs && ${executable(node)} ./ui/node_modules/vite/bin/vite.js ./ui --port 3015 --host 127.0.0.1 --strictPort`, cwd: repoRoot, url: 'http://127.0.0.1:3015', env:{...process.env,VIVY_BACKEND_ADDR:'http://127.0.0.1:8787'}, reuseExistingServer:false, timeout:120_000},
 ],
});
