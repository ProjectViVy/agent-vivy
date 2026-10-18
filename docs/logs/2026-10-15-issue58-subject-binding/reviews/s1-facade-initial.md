# Task 2 source review

## Spec compliance

- ✅ Source increment is compliant with the supplied Task 2 contract. Garden exposes the five bound C1 methods on its existing owner, and Vivy aliases the owner DTOs rather than creating another persistence contract (`garden/agentapi/recap.go:13-116`; `internal/cognitivecontract/recap.go:9-31`).
- ✅ Trusted composition is the only new binding producer seam. The callback receives the existing Store.Root before state/ingestion initialization; there is no ProfileID-to-Recap fallback (`garden/internal/runtimecore/runtime.go:149-158`; `internal/cognitivecontract/ports.go:30-35`; `internal/modules/diva-cognitive/factory.go:47-63`). Durable registration remains S2's responsibility.
- ✅ Reads cannot select scope; commits accept only an omitted or equal binding and delegate unchanged C1 requests to the Store. Reduced workers fail the existing user-only owner grant (`garden/agentapi/recap.go:41-48,74-85`).
- ✅ The optional provider is added without a second Open, synthetic session, scheduler, reflection dependency, or persona/context preparation changes (`internal/modules/diva-cognitive/factory.go:47-66`; `internal/modules/diva-cognitive/recap.go:5-9`).
- ⚠️ This approves the source increment, not the complete Task 2/S1 acceptance gate. Reviewed publication, coordinated garden/laputa/mentle pins and sums, `laputa-source.lock.json`, dependency closure/bootstrap, exact-pin tests, and final CI remain expressly deferred (`docs/superpowers/plans/issue-58/S1.md:79-80`; `docs/logs/2026-10-15-issue-58-recap-facade/verification.md:40-46`).
- ⚠️ The diff does not establish native Windows durability, Task 1 crash/cross-process/v1-v2 preservation, all-v3-writer CAS closure, real PostgreSQL acceptance, S2 durable path/identity registration, or S3/S4 runtime authorization. The controller must retain their existing acceptance gates; this review neither reopens nor approves those implementations.
- ⚠️ Nil resolver preservation is tested at the bundle's existing uninitialized-persona boundary, where Prepare returns ErrPersonaUninitialized (`internal/modules/diva-cognitive/recap_test.go:28-39`). End-to-end ordinary unnamed chat, initialized Frozen Core as sole persona authority, and workers receiving no implicit Persona/history are outside this source diff and require controller-owned evidence. No changed preparation path was observed.

## Strengths

- Lifecycle locking is explicit and spans every complete Store call, while Garden now closes the same owned Store (`garden/agentapi/recap.go:28-48,62-69`; `garden/internal/runtimecore/runtime.go:244-252`). Retained handles cannot race closure into a usable owner.
- Scope denial occurs before content operations; owner error codes survive through curated messages and stable dispatch classification (`garden/agentapi/recap.go:41-59,80-85`; `internal/modules/diva-cognitive/dispatch.go:140-149`).
- Tests exercise actual owner persistence and receipts: clear/restore keeps current coverage, stale CAS cannot resurrect text, replay returns the original receipt, and edits preserve persona documents (`garden/agentapi/recap_test.go:55-150`). Denied reads/lookup/history/revision/commit assert empty results (`garden/agentapi/recap_test.go:153-182`); Vivy tests check trusted root delivery, once-only callback execution, and close/reopen (`internal/modules/diva-cognitive/recap_test.go:41-124`).
- Source scope is narrow: C1 ownership methods are unchanged, binding logic stays in Garden, and the Vivy provider is a small interface exposure (`internal/modules/diva-cognitive/recap.go:5-9`).

## Issues

### Critical

None found in the reviewed source increment.

### Important

None found in the reviewed source increment. Pending publication/pins and durability acceptance are controller-owned gates, not implicitly satisfied by this verdict.

### Minor / evidence follow-up

- Reported verification is not pristine: full Garden initially lacked embedded console assets; reported normal Vivy module verification still fails for 19 local placeholder modules, and the full Vivy suite was still running at handoff (`docs/logs/2026-10-15-issue-58-recap-facade/verification.md:25-38`). The Garden prerequisite was reportedly repaired; controller must record the final Vivy outcome and distinguish the known placeholder verification condition from normal published dependency closure. No source defect is inferred from these reported environment conditions.

## Focused checks and limits

- Read the brief and implementation report, then both complete review packages. Tool output truncated the middle of the combined package read; recovered only the affected runtime/doc region, with a small boundary overlap. No git operations and no tests were run.
- Named risk — a reduced worker might inherit owner access: checked unchanged `garden/agentapi/policy.go:39-71` and `garden/agentapi/embedded_domain.go:164-184`. BindAgentSession stamps PrincipalAgent; OpActmemSave permits only PrincipalUser.
- Named risk — lifecycle lock ordering or shutdown could invalidate a live Store call: the Open/Close hunks did not contain their complete neighboring lifecycle functions, so inspected only `garden/agentapi/open.go:102-133` and the missing beginning of `garden/internal/runtimecore/runtime.go:215-242`. Client.Close takes the matching write lock; caller() takes no nested lock; ingestion closes before ACTMEM. No deadlock introduced by the new path was identified.
- Named risk — forwarding owner results might leak content on error: checked unchanged C1 entry points `laputa/actmem/recap.go:46-168` and their history loader `laputa/actmem/recap_history.go:17-94`. Error exits return zero heads/false/nil, including scope denial and partial history failures; filesystem read errors are domain-wrapped. This was a focused adapter-contract check, not a renewed C1 durability review.
- Reported red/green/race evidence was assessed as reported evidence, not independently rerun. No inspection of unrelated files, broad repository crawl, or subagent delegation occurred.

## Assessment

**Task quality: Approved for the source increment.**

The implementation is small, typed, and keeps authority, persistence, authorization, and lifecycle in the existing Garden owner. No blocking source issue was identified; complete S1 acceptance remains pending the explicitly listed controller gates.

## Published pin/closure follow-up — c1e3756..e5f03ab

**Spec compliance: ✅ Approved for the dependency increment.** The prior source verdict remains valid.

**Code quality: Approved.** No Critical, Important, or Minor defect found in the new pin/sum/lock changes.

- The Garden and Laputa direct requirements and Mentle indirect requirement all select `v0.0.0-20261015094757-5f26de678a99` (`go.mod:18-19,63`). Its timestamp and 12-character suffix agree with the controller-confirmed publication time `2026-10-15T09:47:57Z` and full SHA `5f26de678a999c3e52a6e0d89dfc4debb3b00b9b` (`laputa-source.lock.json:3`).
- The six new lines supply both archive and go.mod checksums for exactly those three versions; old sums are retained (`go.sum:765-770`). Named risk — transcription from the temporary checksum modfile: read `/tmp/issue58-task2-pins.sum` once and compared all six lines with the diff; every module/version/hash matches. No downloads or verification commands were rerun.
- Scope remains narrow: only the three requires, six sums, source lock and existing evidence documents change. No source behavior, tests, recipes, replacements, or guard weakening appear in this increment (`go.mod:18-19,63`; `go.sum:765-770`; `laputa-source.lock.json:3`).
- Documentation consistently moves publication/focused closure from pending to completed while retaining full candidate CI and platform/product gates (`docs/logs/2026-10-15-issue-58-recap-facade/acceptance.md:15-17`; `summary.md:18-21`; `verification.md:54-87`, in the same log directory). This supersedes the earlier review's publication/pin/focused-closure pending item only.
- Reported evidence now includes normal sumdb-authenticated archives, temporary-modfile verification, all 11 bootstrap/closure tests, exact published Garden tests, exact-pin Vivy tests, and readonly smoke. These were assessed as existing reported evidence, without rerunning tests. The controller independently confirmed published tree `f19774059478d1e28098768dca5d08e906a5e79f` matches source-reviewed `557b66b`; this follow-up does not repeat that source-tree audit.
- ⚠️ Raw full-root `go test ./...` is explicitly FAIL, with the 10-minute package timeout and VCS exit 128 now disclosed (`docs/logs/2026-10-15-issue-58-recap-facade/verification.md:30-36`). The reported clean-clone stamped headless build addresses the VCS build condition, but does not turn the raw suite into a pass or replace final `just ci` (`verification.md:81-87`, same directory). The root local-placeholder ziphash condition also remains disclosed (`verification.md:40-43`).
- ⚠️ Full candidate CI, native Windows durability, Task 1 durability acceptance, and centrally owned S2/S3/S4/product evidence remain pending. This scoped approval is not S1 durability/product acceptance (`docs/logs/2026-10-15-issue-58-recap-facade/acceptance.md:15-17`).

Review checks: read the supplied pin diff once and the appended implementation report; one focused checksum-source read; no source rereads, git operations, tests, or subagents.
