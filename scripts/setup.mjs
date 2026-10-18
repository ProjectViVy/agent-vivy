#!/usr/bin/env node
// Resolve the pinned source closure and download modules with a process-local
// proxy. Never writes `go env -w`: a developer mirror must not leak into the
// rest of the machine or into later gates.
import { join } from 'node:path';
import { fail, repoRoot, run, runInherit } from './lib/process.mjs';

const prefix = 'setup';
const proxy =
  (process.env.VIVY_GOPROXY || process.env.GOPROXY || 'https://goproxy.cn,direct').trim() ||
  'https://goproxy.cn,direct';

if (proxy === 'off') {
  fail(prefix, 'GOPROXY=off cannot download modules; unset it or set VIVY_GOPROXY');
}

const laputa = runInherit(process.execPath, [join(repoRoot, 'scripts/ensure-laputa.mjs'), '--quiet']);
if (laputa.error) {
  fail(prefix, `could not start the Laputa bootstrap: ${laputa.error.message}`);
}
if (laputa.status !== 0) {
  fail(prefix, `Laputa bootstrap failed (exit ${laputa.status})`);
}

function download(effectiveProxy) {
  return runInherit('go', ['mod', 'download'], {
    env: { ...process.env, GOPROXY: effectiveProxy },
  });
}

console.log(`${prefix}: go mod download with GOPROXY=${proxy} (process only; no go env -w)`);
let result = download(proxy);
if (result.error) {
  fail(prefix, `go could not start: ${result.error.message}`);
}
if (result.status !== 0) {
  const hasFallback = proxy.split(',').some((entry) => entry.trim() && entry.trim() !== 'direct');
  if (!hasFallback) {
    fail(prefix, `go mod download failed (exit ${result.status}) with GOPROXY=${proxy}`);
  }
  console.warn(
    `${prefix}: go mod download failed with GOPROXY=${proxy}. ` +
      'The comma fallback only covers 404/410, not 5xx; retrying once with GOPROXY=direct.',
  );
  result = download('direct');
  if (result.error) {
    fail(prefix, `go could not start: ${result.error.message}`);
  }
  if (result.status !== 0) {
    fail(prefix, `go mod download failed with the project mirror and with GOPROXY=direct`);
  }
  console.log(`${prefix}: module download succeeded with GOPROXY=direct`);
} else {
  console.log(`${prefix}: module download succeeded with GOPROXY=${proxy}`);
}
