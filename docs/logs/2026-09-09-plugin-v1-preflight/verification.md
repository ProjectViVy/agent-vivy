# Plugin v1 preflight verification

## Test-first fixture validator

1. RED: `node --test scripts/check-plugin-v1-fixtures.test.mjs`
   failed with `ERR_MODULE_NOT_FOUND` before the validator existed.
2. GREEN: the same command passed all 5 tests after the minimal validator was
   implemented.
3. Corpus: `node scripts/check-plugin-v1-fixtures.mjs` reported 9 cases: 2
   accepted and 7 rejected.

## Static checks

- `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"`
  passed.
- `git diff --check` passed with no whitespace errors.
- The intersection between this worktree's changed paths and
  `git diff --name-only origin/main origin/pr-5` was empty.

## Full repository gate

The local runtime does not provide Go, `just`, or PowerShell, so it cannot
faithfully execute this Windows-oriented repository gate. The new GitHub
Actions `just ci` job is the authoritative full run for this delivery. Record
the workflow URL and result here after the branch is published.
