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
Actions `just ci` job is the authoritative full run for this delivery.

- Run 1 failed in `ui-ci`. The log showed two independent configuration
  defects: Windows had no `rg`, so `fmt-check` did no work, and pnpm 11.19.0
  rejected the unreviewed `esbuild@0.28.2` lifecycle script with
  `ERR_PNPM_IGNORED_BUILDS`.
- The follow-up replaces the non-portable `rg` dependency with `git ls-files`
  and explicitly allows only the lockfile-pinned esbuild version in
  `ui/pnpm-workspace.yaml`.

## Existing I18N baseline

- `pnpm install --frozen-lockfile` passed locally with pnpm 11.19.0 after the
  exact esbuild approval; its postinstall completed.
- `pnpm typecheck` passed.
- `pnpm test` reached 213 tests and reported 16 failures. Each observed failure
  is an active-I18N expectation mismatch: the implementation now emits English
  while the existing test still expects a Chinese literal. These failures are
  outside this isolated lane and are intentionally not patched here.
- [GitHub Actions run 2](https://github.com/ProjectViVy/agent-vivy/actions/runs/34370987236)
  confirmed the runner fixes: no missing-`rg` or unapproved-build failure;
  fixture checks, corpus validation, install, and typecheck passed. The full
  gate then stopped on the same 16 pre-existing active-I18N test expectations.
