# Verification commands and results

| Command | Result |
| --- | --- |
| `cd ui; pnpm exec tsc -b --force` | Passed (type checking green after the equivalent zh/en key migration) |
| `just ui-e2e` | `1 skipped / 10 passed` (including the new `compaction-setting.spec.ts`), exit code 0 |
| `just ci` | Exit code 0 (golangci-lint + gofmt + go test ./... + UI tsc/eslint/vitest/build) |

## Diagnostic chain (for review)

1. The first e2e failure snapshot
   (`ui/test-results/compaction-setting-.../error-context.md`) showed that the
   card consisted entirely of `settings.compaction.*` raw keys. This directly
   disproved "missing keys," since the dictionary clearly contained a matching
   block, and located the issue as a key mismatch (`diva.compaction` vs
   `settings.compaction`).
2. `grep -n "^  \},\|^  [a-z]" zh.ts` established the top-level structure:
   `settings:` covered only 817–885, while the original compaction block was in
   the `diva` section. The first migration was mistakenly placed in the `demo:`
   section (the second e2e run still showed raw keys); the second migration put
   it in the `settings:` section.
3. The third e2e failure moved to the assertion for English-character leftovers:
   a hardcoded Chinese migration-note paragraph in `DivaSettingsPreview`
   contained the substring `Max tokens`, causing a non-exact match. The
   assertion was changed to an exact match, and the preview-area debt was split
   out into a separate TODO row.
4. The final `just ui-e2e` run was 10 passed / 1 skipped; `just ci` was green.

## Lessons (recorded in this iteration)

- `zh.ts`/`en.ts` have many top-level sections with identical indentation. When
  moving a key block across sections, first use structural grep to establish the
  section boundaries (`settings:` does not extend from `tabs` to the end of the
  file).
- The e2e server consumes the embedded artifact in `ui/dist`, so the build must
  be rerun after changing source (`just ui-e2e` includes `pnpm build`; running
  `tsc` alone does not refresh dist).
