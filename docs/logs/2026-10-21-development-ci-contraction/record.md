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

## Instruction and trigger follow-up

The owner's checklist exposed omissions after the initial implementation. The
original workspace remains on stable; this delivery is in the isolated
`codex/development-ci-contraction` worktree and has not been merged into it.
That visibility difference did not account for all omissions: the plugin
completion contract still required full CI, Module review still imposed a
platform questionnaire on every edit, and skill routing still cascaded broadly.

| Checklist area | Result |
| --- | --- |
| Repeated full CI and split startup | Root owns scope. Plugin completion now references it; browser startup applies only to browser behavior. No CI file was changed or product CI run during this follow-up. |
| Fixed three-file delivery and generic numeric thresholds | Removed in the initial rule contraction; ordinary delivery uses one record, minor instructions can use a commit. |
| Old machine paths, click coordinates and approval bypass | Removed from testing-vivy-ui in the initial pass. Environment skill now routes matching symptoms to conditional historical references instead of repeating workarounds in routine guidance. |
| First-party defaults and ordinary Provider work | Explicit Recipes choose capabilities. Providers inherit an established Port; only new/changed public contracts need platform design. The residual every-edit questionnaire and fixed test-first instruction were removed. |
| Eino checks | New/replacement capabilities inspect pinned APIs. Existing adapter fixes reuse the decision and inspect affected interfaces. Root, skill and architecture clauses now agree. |
| Skill triggers and reading | Narrowed kernel, plugin, UI, Eino, environment and legacy redirect descriptions. No automatic workflow from prose mentions; no blanket four-document read or cascading required subskills. External caches remain untouched. |
| Ownership and evidence ritual | Root retains one authority and derived copies. Evidence machinery must name a consumer and decision; unused producers, projections and exclusive checks retire together. No universal per-fact conformance dossier remains. |
| Existing runtime hash/evidence mechanism deletion | Not delivered here. The ban on new mechanisms is a rule change, not evidence that existing mechanisms were removed. Issue #91 and its existing deletion plan remain open. |

A bounded source inspection confirmed that Port support state affects
`sdk/internal/assembly/compiler.go` selection through `requireSelectablePort`,
and Provider suite results feed that state through
`sdk/internal/conformance/generation.go`. Evidence identifiers also enter
validation and Inspect projections. These are consumers, but their existence
alone does not justify every field or the whole mechanism. A complete field-by-
field deletion audit was not performed. Existing open board entries, including
ISSUE-91-DELETION and UI-MODULE-DEPS-NOT-INSTALLED, retain that runtime work;
this instruction cleanup does not claim to close them.

Follow-up verification: diff/wording review passed; all six changed skill
entrypoints passed quick_validate; `git diff --check` passed. No product code, dependency,
Recipe, CI workflow or CI command implementation changed in this follow-up.
