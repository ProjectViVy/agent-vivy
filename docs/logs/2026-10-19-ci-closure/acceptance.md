# CI closure acceptance

## Decision

**Accepted for the requested Linux integration lane and local gates. Native Windows acceptance is external and remains unexecuted.** No failing test or required Linux gate remains unresolved, and no verification shell is still pending. This is not publication, release or merge approval.

| Requirement | Observed acceptance evidence | Boundary |
|---|---|---|
| R1 / S1: Face lifecycle and shutdown race | Integrated App tests passed in complete CI and `race-ci`; ten focused Face race repetitions passed | Test-only ownership/lifetime repair; production owner and unknown-commit guards unchanged |
| R2 / S2: SDK stream separation | Complete Linux SDK tests passed; six focused parsing/hashing/failure test groups passed with `-race` | Only stdout becomes data; stderr and wrapped `exec.ExitError` retained on failure; importability guards unchanged |
| R3 / S3: portable Toolkit fixtures | Complete Linux runtime and runtime race package passed with the new real subprocess fixture | Native Windows still external; no platform skip or shell-selection weakening |
| R4 / S4: portable BML gate | Complete CI reached BML vet and uncached tests successfully; 20 bootstrap tests passed, including BML error/signal/stdio cases | Native Windows still external; Linux success does not prove `cmd.exe` execution |
| R5 / S5: published cancellation fix | Official private dependency bootstrap, pin checks, Python units, real-Go traces/replays, native pre/post-replacement regressions, ACTMEM and selected Garden compatibility passed | Post-replacement uncertainty remains fail-closed; not the full upstream test matrix |
| R6 / S6: integrated evidence | Complete `just ci`, complete official `just race-ci`, real `just test-postgres`, doctor, isolated split startup/RPC smoke and separate integration review passed | Full requested Linux gate evidence, not native Windows or blanket feature acceptance |

## Newly exposed failure was closed, not bypassed

The first `just ci` and PostgreSQL gate failed `TestNotebookLegacyImport` because its fixture constructed a malformed DSN. The focused test failed before the fixture edit and passed after it. The fixture now reuses the existing parsed-DSN/schema helper and adds close-on-failure cleanup, preserving migration/import/data assertions. Real PostgreSQL, full repaired CI and full official race all then passed. The historical failed attempt remains visible in `ci.log`; it is not an outstanding failure of the final source.

No new skips, xfails, disabled assertions, relaxed production deadlines, owner bypasses, unknown-commit bypasses or source-bootstrap bypasses were introduced. The repository's pre-existing DIVA memory-loop routing was followed: the ordinary root exclusion is followed by its sealed-assembly runner, and both memory-loop and generated-assembly recall checks completed.

## Complete gates versus partial evidence

- **Complete on this Linux source:** the actual `just ci` recipe, actual `just race-ci` recipe and actual `just test-postgres` recipe completed with explicit exit 0. Starting the shells was not treated as success.
- **Race scope:** `race-ci` races `./internal/...` with its existing memory-loop exclusion. Focused SDK, real BML/Recap and selected dependency race runs are additional evidence; they are not a claim that full `just test-race` or every independent module ran with the race detector.
- **Native Windows:** no Windows host executed these patches. Worker cross-compilation and utility-stripped Linux fixtures are useful portability evidence only. The owner's Windows runner must still execute the official bootstrap/BML and affected Face/SDK/Toolkit gates before claiming native Windows acceptance. Existing platform/source guards remain intact.
- **Memory scope:** representative real-Go model traces and native tests cover replay/tombstones, cancellation before and after replacement, stale/changed operation identities, restart/crash and owner boundaries. Finite traces do not prove every interleaving or the full autonomy/evolution product contract.
- **UI scope:** CI's typecheck/unit/build/i18n checks and executable split RPC smoke passed. No manual browser or Playwright UI acceptance was performed for this test/command/dependency repair.
- **Dependency scope:** ACTMEM and four selected Garden compatibility packages passed. No full upstream Laputa/Garden product-suite claim is made.
- **Environment warning:** doctor saw pnpm launcher `10.34.7` versus pin `10.33.0`; actual UI gate logs report pinned `10.33.0`. This was nonfatal and did not authorize global package/config changes.

## Review and handoff

The integration engineer inspected every patch scope and performed a separate review of the integrated invariants, independently of S1-S5 authors. The PostgreSQL follow-up also received a second manual pass against `openPool`, cleanup order and unchanged assertions. This does not assert that a new external reviewer ran; the owner may review the final working-tree diff before any future delivery decision.

The parent subsequently inspected the final code diff, checked dependency cleanliness and raw aggregate exit markers, and independently reran Face, SDK, bootstrap and vet checks. No additional code defect was found in that review. The deliverable is prepared for a local commit attributed to the responsible human; publication is not authorized by this record.

Original workspaces and production journals/data were untouched. The DSN and bootstrap token were never printed. The temporary dev pair was intentionally stopped after successful health/RPC checks. All finite gate shells have exited. There is no remaining Linux blocker; native Windows is the explicit external acceptance boundary, tracked in `docs/TODO.md` as `MEMORY-CI-WINDOWS-20261019`. No push, PR, merge or deployment was performed.
