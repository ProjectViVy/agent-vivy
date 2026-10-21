# Development rules and layered CI contraction

## Delivered scope

Project rules now prioritize useful capability, fast real feedback and total
cost. Routine checks and records are proportional. vivy-kernel-ci remains the
environment-aware validation guide. Retired hash mechanisms cannot be introduced
under alternate names. Existing product Recipes and runtime behavior are unchanged.

One .github/ci-scopes.json table drives local and Actions selection through
`just ci-small --base <ref> [--scope <id> ...] [--plan]`. Explicit scopes are
additive; unowned executable inputs fail. Recipe sessions preserve their own
checks and tests. Non-source inputs resolve to Go owners; traversal and empty
required selections fail. Known plugin combinations select their actual Recipe.
UI dependency preparation follows SDK base-shell requirements, including
non-minimal profiles without an explicit ui section.

PR/branch development runs small CI. The epic-final PR label enables the existing
full product lanes after small CI; updates rerun them and cancel the older run.
The just ci aggregate separately reports full CI not requested/not verified.
The legacy kernel workflow remains manual diagnostic only. No push, remote label,
branch-protection mutation, global plugin edit, production-data access or runtime
feature retirement was performed.

## Verification

- Windows Node selector/bootstrap/owner tests: 34 passed in the broad script run;
  later focused regression checks also passed. Includes path unions, renames,
  deletes, untracked files, unknown inputs, Recipe validation, command failures,
  label events and actual aggregate CLI cancelled/skipped-lane rejection.
- YAML parsing and orchestration assertions: passed. just --list parses the new
  entry. All five changed primary skills pass quick_validate with UTF-8 enabled.
- Independent Linux-filesystem checkout with its own pinned Laputa and offline
  UI install: hello-fs Go owner tests, actual pack and inspect passed. The actual
  App TestToolResultReachesSecondModelRequest passed with a required executed-test
  assertion. Selected minimal-chat typecheck and 82 UI tests passed.
- Real split-Vite browser: all 3 minimal-chat scenarios passed (causal tool
  continuation/narrow viewport, cancel/one terminal, provider failure). Initial
  missing shared-library failures were recorded, not passed. Debian libraries
  and fonts were downloaded/extracted into this lane's user-owned scratch with
  process-local LD_LIBRARY_PATH/FONTCONFIG_FILE; no system install was performed.
- Independent reviews found incorrect owner/Recipe associations, a check/Recipe
  cross product, missing UI prerequisites, skipped authoritative UI tests and Go
  traversal. They were corrected with focused regressions.
- First native Windows full just ci at 51618a04: stopped at UI build when
  esbuild could not remove a system TEMP file (Access is denied). Bootstrap,
  formatting, UI typecheck and the UI test stage ran before this failure. The
  final tooling fix isolates TEMP/TMP only inside the existing UI recipe child
  process; Go test environments remain unchanged. No product runtime workaround
  was added.
- The subsequent native full run passed bootstrap, formatting, UI typecheck,
  661 UI tests, UI build, i18n and Go vet, then encountered Go heap/page allocator
  out-of-memory failures. A bounded Go continuation was stopped at the user's
  direction. Full product CI is not certified; no further product CI is required
  to finish this instruction/tooling change.
- The user clarified the standing boundary: skill, instruction and documentation
  work without product code or behavior changes uses relevant static checks only
  and must not expand into CI rewiring or product CI. Root instructions now state
  that boundary. Existing CI changes in this isolated branch came from the earlier
  explicit CI implementation plan; they have not been pushed or merged.
- UI source changes also use existing Vitest related --run; a real
  ModelSettingsCard selection executed its 5 owner tests. The CLI plan retains
  Recipe-specific commands instead of displaying a check/Recipe cross product.
- Live GitHub triggers/cancellation and actual required-check compatibility are
  unverified because this branch is not pushed. Branch protection could not be
  read with the existing integration permissions; maintainer confirmation remains
  an explicit rollout action.

## Acceptance and rollback

A docs-only PR selects static checks. Feature PRs show their selected scope,
Recipe, commands and reasons. A missing owner fails with an actionable mapping
error. A PR with epic-final requires every existing full product lane to succeed;
without it the aggregate says full CI was not requested, not that it passed.

The user can review the isolated codex/development-ci-contraction branch and
then publish through the normal PR route. Revert this focused change to restore
previous development policy/CI wiring; no data migration or product rollback is
needed. No release artifact is introduced by this tooling delivery.
