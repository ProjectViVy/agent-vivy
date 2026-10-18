# Toolkit matrix acceptance — 2026-10-19

Every row below is evaluated against the lane candidate
`feat/l4-toolkit-matrix`: `fdbc032b` (reproducible gate) plus `09b5cfc7`
(T1–T4 tests and generated UI Assembly), with the docs commit that contains
this log. Browser rows additionally use the packed
`generation 9de83d553ac96d483d7bab25cd7bda2ab3816527ecae3734a6284e332179ed95`
(42 modules) produced by the launcher. A row is **not** accepted merely because
a related test exists; the table names the actual assertion and the remaining
gap.

## How a human confirms the delivered path

```bash
# Preferred tracked entry (on a host with just):
just toolkits-e2e

# This host: just is not installed, so the recipe body was used directly.
node scripts/toolkits-e2e.mjs

# Concurrent lane / isolated ports:
VIVY_TOOLKITS_E2E_BACKEND_PORT=18887 VIVY_TOOLKITS_E2E_UI_PORT=3115 \
  node scripts/toolkits-e2e.mjs
```

The command packs the default Recipe, inspects it, then runs the lifecycle and
accessibility suites. Expected observed lines: `generation 9de83d55…`,
`modules 42`, `1 passed` (lifecycle), `3 passed` (accessibility),
`all Toolkit suites passed`. The launcher exits non-zero if any suite fails.
The gate does not reuse another lane's server; it starts its own backend, Vite
server and fresh SQLite data root for each suite and stops them afterward.

Manual product view: run `just dev` (backend `:8788`, Vite `:3016`), open
`http://127.0.0.1:3016/toolkits`, and use the library/install/detail/consent
flow. This log did not capture screenshots; the browser assertions below are
the executable evidence.

## Requirement rows (B1–B14)

| ID | Exact evidence on this candidate | Outcome | Remaining gap / owner |
| --- | --- | --- | --- |
| B1 | `TestToolkitMCPConsentCaptureGovernsRealServiceRun` and `TestToolkitMCPPublicationRefreshesAppToolHostAndRecovery` exercise real Service/ToolHost; gate E2E reads a captured Skill through the content dialog. | Partial | No governed Rule run and no live unreachable/schema MCP run in this batch. TK-13. |
| B2 | `ui/e2e/toolkits.spec.ts` installs native PM and Dev Masks and keeps independent Skills visible. | Partial | Full selection/run-snapshot/update-race matrix not added. TK-13. |
| B3 | Same E2E asserts the PRD recommendation does not enable the implementation Skill; recommendation is shown as non-activating copy. | Partial | No before/after model, Grant and consent inventory assertion. TK-13. |
| B4 | New `TestToolkitReinstallDoesNotInheritConsent` proves a tombstoned BundleID gets a new InstallID and no inherited consent; existing identity/digest storage tests pass. | Partial | No live cross-source collision through the user-facing install action. TK-13. |
| B5 | New `TestToolkitCorruptSnapshotIsUnavailableWithoutLatestFallback` restores the exact captured epoch while the current catalog is a later epoch; existing root/report/workflow admission tests pass. | Partial | Queue/checkpoint/goal/continuity admission branches still lack one-epoch assertions. TK-13. |
| B6 | Gate E2E proves install leaves MCP inactive/unconnected; source acquisition tests validate declared bytes. | Partial | No process/network spies across Skill/Mask/Rule/Hook/MCP for install/update. TK-13. |
| B7 | Existing consent/revocation tests plus new `TestToolkitCapturedHookFailureMatrix` prove inert-until-armed, exact consent, and deny/rewrite/crash/timeout fail-closed through the dispatcher. | Partial | No live App run for the full Hook protocol; disarm is covered only through the existing revocation fence. TK-13. |
| B8 | New `TestToolkitBeforeCommitFailurePreservesCommittedEpoch` fails a real publication after owner preparation and proves stored epoch, installation, active View and gate are unchanged; retry commits epoch 1. Existing post-commit projection failure test passes. | Partial | No crash/CAS injection at every publication boundary. TK-13. |
| B9 | Existing runtime/storage revocation fence tests; gate E2E disables/revokes. | Partial | Pending-cancellation and every live-call boundary under concurrent runs are not covered. TK-13. |
| B10 | New snapshot restore test proves exact-epoch recovery and fail-closed missing/corrupt artifacts; existing app restart test preserves uninstall history. | Partial | Crash-after-commit/pointer recovery and cross-version snapshot compatibility are untested. TK-13. |
| B11 | Existing omission artifacts and App/UI tests report missing Mask/MCP owners; gate E2E renders a staged owner-unavailable state. | Partial | Not every owner/kind combination has a user-facing error test. TK-13. |
| B12 | Three inspected omission artifacts keep protected Tool exports unchanged; existing assembly tests keep omitted Modules out. | Partial | No runtime Grant/frozen-Generation invariance assertion for lifecycle changes. TK-13. |
| B13 | Gate runs one desktop lifecycle test and three accessibility tests (390, 768, 1440); new `ToolkitCandidate.test.tsx` proves blocker rendering; existing `ToolkitUpdate.test.tsx` covers lost reply and stale conflicts. | Partial | Remaining operation failure/recovery states (reload during prepare, cancel ambiguity) are not all browser-exercised. TK-13. |
| B14 | Gate E2E installs a native local Toolkit; parser tests reject foreign-only manifests. | Partial | No complete foreign-format/adversarial source matrix through the user action. TK-13. |

## Failure and user-flow rows (A01–A28)

| ID | Exact evidence on this candidate | Outcome | Remaining gap / owner |
| --- | --- | --- | --- |
| A01 | E2E previews native PM/Dev source, shows exact digest/components, then installs disabled. Payload execution is not directly spied. | Partial | B6 spy gap; no direct no-exec assertion for every kind. TK-13. |
| A02 | New `TestToolkitPreviewSourceChangeDoesNotSubstitute`: source mutation changes the digest, another preview's digest is `preview_stale`, and the installed page contains the reviewed bytes. | Extended | Manager-level only; no user-facing stale-preview browser case. TK-13. |
| A03 | Existing upstream-only manifest validation rejects unsupported format; no conversion path exists. | Partial | User-facing foreign-source error not exercised in this batch. TK-13. |
| A04 | Existing strict manifest/frontmatter tests fail the whole candidate for duplicate/unknown/event/authority fields. | Partial | No consolidated table over every malformed field. TK-13. |
| A05 | Existing path/symlink/size-limit tests reject traversal and retain the prior object. | Partial | Case-collision and deep/large-limit combinations remain uncovered. TK-13. |
| A06 | New reinstall test proves distinct InstallID and no consent inheritance; existing conflict/rebind tests cover same-BundleID candidates. | Extended | No cross-source collision through the live install UI. TK-13. |
| A07 | E2E `View content` shows the captured Skill body inside the dialog; runtime Skill tests cover declared resources without a mount. | Partial | No resource read through `skill_view` in a real run. TK-13. |
| A08 | Existing traversal/binary/mutation tests return typed rejection before write. | Partial | No browser/Service path for the same adversarial fixtures. TK-13. |
| A09 | E2E shows PM recommendation while the implementation Skill remains independently available. | Extended | Ordinary-policy invocation of the independent Skill not re-run here. TK-13. |
| A10 | Existing Mask selection/snapshot tests keep the current run on its snapshot and custom copies independent. | Partial | Concurrent Mask change during publication not covered. TK-13. |
| A11 | Existing reference-blocker tests reject removing a selected Mask and uninstall/disable leaves an unavailable selection. | Partial | Browser UI for the blocked removal is not exercised. TK-13. |
| A12 | Existing Rule import/overflow admission tests fail before the model request. | Partial | No live prompt-size boundary run. TK-13. |
| A13 | E2E arms a Hook only after separate consent and requires re-consent after the declaration changes; runtime tests assert exact digest/revision. | Extended | No live App Hook execution on this candidate. TK-13. |
| A14 | New `TestToolkitCapturedHookFailureMatrix` covers allow, exit-2 deny, other-exit, non-JSON, rewrite, invalid rewrite, timeout and missing executable on a captured armed run; existing revocation test covers disarm. | Extended | Dispatcher-level, not a real tool invocation through Service; no PostToolUse matrix. TK-13. |
| A15 | E2E proves MCP binding/consent without eager connection; `TestToolkitMCPConsentCaptureGovernsRealServiceRun` performs an authorized real MCP call. | Partial | Live unreachable/timeout MCP transport not run. TK-13. |
| A16 | Existing MCP collision/schema/revocation tests reject candidate conflicts and stale identities. | Partial | No live reopen-after-fence transport assertion. TK-13. |
| A17 | Existing drain/blocker tests keep the epoch stable and show blockers; gate E2E observes a prepared candidate. | Partial | Approval-wait/checkpoint/child-work timing is not exercised. TK-13. |
| A18 | Existing settings/update contention tests retry the drain and rebuild from latest state. | Partial | Mask selection concurrent with publication is not in one barrier test. TK-13. |
| A19 | Existing root/report/workflow admission tests prove prompt/bundle/start atomicity for those branches. | Partial | Continuity/goal/queue/edit branches remain uncovered. TK-13. |
| A20 | Existing recovery tests restore before opening admission; new snapshot test covers corrupt/missing artifacts. | Partial | Subprocess kill before/after commit and pointer swap are not injected. TK-13. |
| A21 | New `TestToolkitCorruptSnapshotIsUnavailableWithoutLatestFallback`: intact snapshot restores epoch 1, tampered payload fails `snapshot digest mismatch`, missing artifact fails, and neither falls back to current epoch 2. | Extended | Bundles-level only; Runtime resume surfaced through a real Service run is not exercised. TK-13. |
| A22 | Existing revocation fence tests reject the next governed boundary after revocation. | Partial | No assertion that an in-flight dispatched call is not claimed undone. TK-13. |
| A23 | E2E uninstalls and leaves history/orphan metadata; storage retention tests pass. | Extended | User credentials/custom-copy retention not re-verified in this batch. TK-13. |
| A24 | Existing App authority tests reject non-human/direct-control consent actions; they were not rerun in this lane. | Partial | Anonymous/public-owner/agent-bound cases lack one consolidated matrix. TK-13. |
| A25 | Existing `ToolkitUpdate.test.tsx` covers lost publish reply and stale consent/bindings conflict; new ToolkitPage test discards a stale detail response after a second selection. | Extended | Reload/recovery of a pending operation and dirty-input preservation beyond the existing cases remain open. TK-13. |
| A26 | Three inspected default/omitted artifacts prove page/capability presence and no dynamic Module/Port loading; this is historical evidence, not a fresh omission pack in this lane. | Partial | Inspection is not rerun in this lane with fresh omission scratch; historical artifact evidence is reused. TK-13/T5. |
| A27 | Gate: 390px keyboard path, 768px and 1440px no-overflow, populated 390px content dialog; new unit test proves run/session blockers and hint render. Localized 工具包 label not asserted here. | Extended | No zh-locale browser assertion, no remaining operation error/recovery view. TK-13. |
| A28 | SQLite fresh/reopen and existing migration tests pass on this candidate. PostgreSQL and cross-version snapshot compatibility were not run. | Blocked | Requires an authorized disposable Postgres DSN (`just test-postgres`) and an old-release snapshot fixture. TK-13/T5. |

## Gate disposition

- Delivered and reproducible: T0 launcher, env-driven ports/binary, pack +
  Inspect before Playwright, both suites green on isolated ports, path-with-
  spaces explicit override, invalid/equal/occupied-port refusal.
- Delivered verification: T1 (3 tests), T2 (1 test), T3 (8 subtest rows),
  T4 (2 unit tests + 1 browser test with two viewports).
- Not delivered: full B/A acceptance, PostgreSQL parity, Windows venue,
  `just ci`/`just release-conformance`, and crash/queue/continuity rows listed
  above. The Toolkit Epic therefore remains **In progress**; this log is a
  completed reconciliation, not a claim that E4/TK-13 is closed.
