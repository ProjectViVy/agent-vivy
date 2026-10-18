#!/usr/bin/env node
// Real-Postgres gate. Unlike the package tests, this entry fails closed when
// the DSN is unset so a skipped run can never be reported as a pass.
import { fail, repoRoot, runInherit } from './lib/process.mjs';

const dsn = (process.env.VIVY_POSTGRES_TEST_DSN || '').trim();
if (!dsn) {
  fail(
    'test-postgres',
    'VIVY_POSTGRES_TEST_DSN is unset; a skipped PostgreSQL run is not a pass. ' +
      'Start a real server and export the DSN first.',
  );
}

console.log('test-postgres: running against VIVY_POSTGRES_TEST_DSN');
const result = runInherit('go', ['test', '-count=1', './internal/storage/postgres'], { cwd: repoRoot });
if (result.error) {
  fail('test-postgres', `go could not start: ${result.error.message}`);
}
process.exit(result.status ?? 1);
