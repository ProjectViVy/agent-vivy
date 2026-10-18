#!/usr/bin/env node
// Run the backend with the isolated development identity: its own user home,
// its own port, and the dev config overlay instead of the daily config.yaml.
import { spawn } from 'node:child_process';
import { mkdirSync, readFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { fail, repoRoot, runInherit } from './lib/process.mjs';

const prefix = 'run-dev';
const defaults = JSON.parse(readFileSync(join(repoRoot, 'scripts/dev.defaults.json'), 'utf8'));
const backendPort = Number(process.env.VIVY_DEV_BACKEND_PORT || defaults.backendPort);
const uiPort = Number(process.env.VIVY_DEV_UI_PORT || defaults.uiPort);
const devHome = resolve(repoRoot, defaults.userHome);

const laputa = runInherit(process.execPath, [join(repoRoot, 'scripts/ensure-laputa.mjs'), '--quiet']);
if (laputa.error) {
  fail(prefix, `could not start the Laputa bootstrap: ${laputa.error.message}`);
}
if (laputa.status !== 0) {
  fail(prefix, 'Laputa bootstrap failed');
}

mkdirSync(devHome, { recursive: true });
const env = {
  ...process.env,
  VIVY_USER_HOME: devHome,
  VIVY_ADDR: `127.0.0.1:${backendPort}`,
  GOPROXY: process.env.VIVY_GOPROXY || process.env.GOPROXY || 'https://goproxy.cn,direct',
  VIVY_CONFIG: process.env.VIVY_DEV_CONFIG || join(repoRoot, 'scripts/dev.config.yaml'),
};

console.log(`${prefix}: backend http://127.0.0.1:${backendPort}`);
console.log(`${prefix}: data root ${devHome}`);
if (env.VIVY_CONFIG) {
  console.log(`${prefix}: config ${env.VIVY_CONFIG}`);
}
console.log(
  `${prefix}: for the Vite half run ` +
    `cd ui && VIVY_UI_PORT=${uiPort} VIVY_BACKEND_ADDR=http://127.0.0.1:${backendPort} pnpm dev`,
);

const child = spawn('go', ['run', './cmd/vivy'], { cwd: repoRoot, env, stdio: 'inherit' });
for (const signal of ['SIGINT', 'SIGTERM']) {
  process.on(signal, () => {
    if (!child.killed) {
      child.kill(signal);
    }
  });
}
child.on('error', (error) => fail(prefix, `go could not start: ${error.message}`));
child.on('exit', (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 1);
});
