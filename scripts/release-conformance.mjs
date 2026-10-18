#!/usr/bin/env node
import { fail, repoRoot, runInherit } from './lib/process.mjs';

const result = runInherit(
  'go',
  [
    'test',
    './sdk/internal/conformance',
    '-run',
    '^TestCheckedInProviderConformanceMatchesExecutedSuites$',
    '-count=1',
  ],
  {
    cwd: repoRoot,
    env: { ...process.env, VIVY_RELEASE_CONFORMANCE: '1' },
  },
);
if (result.error) {
  fail('release-conformance', `go could not start: ${result.error.message}`);
}
process.exit(result.status ?? 1);
