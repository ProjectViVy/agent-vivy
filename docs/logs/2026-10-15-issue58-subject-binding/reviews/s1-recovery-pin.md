# S1 coordinated recovery-pin review

**Spec compliance: ✅ Approved for this narrow increment.**

**Code quality: Approved. No blocking or nonblocking defects identified.**

Reviewed Vivy `d54fa92e180f55c70963348ed44118f1ff899cba..49fe1e869f014ef8d2a00e8804b87e7356badc52`. Original facade and recovery source verdicts remain unchanged; this review covers only coordinated pins, sums, lock and evidence.

## Strengths and compliance

- All three requirements select `v0.0.0-20261015104519-4c7c6b208593` (`go.mod:18-19,63`). The pseudo-version agrees with published commit `4c7c6b20859375d0d764d3ba2a0188926cf73c9e` and timestamp `2026-10-15T10:45:19Z`; the lock records that exact full SHA (`laputa-source.lock.json:3`).
- Six new checksum entries cover the archive and go.mod for Garden, Laputa and Mentle, with previous entries preserved (`go.sum:771-776`). Every added line matches the retained normal-tooling checksum file `/tmp/issue58-recovery-pins.sum` exactly.
- The diff changes only the expected three dependency requirements, six sums, full-SHA lock and existing development/evidence logs. Source, replacements, tests, recipes, guard policy and parent planning index are excluded.
- Documentation distinguishes the recovery pin from earlier intermediate evidence and states the remaining gates explicitly (`docs/logs/2026-10-15-issue-58-recap-facade/verification.md:90-123`; `acceptance.md:20-24`, same directory). No new full-suite or product-acceptance claim is made.

## Evidence checks

Read the recovery brief first, used the existing Task 2 contract already reviewed, read the appended implementation report, and read the complete pin diff once. No source files were reread and no tests or git commands were run.

- Named risk — copied pins or sums might reference a different publication: inspected the three retained `/tmp/issue58-recovery-meta-{garden,laputa,mentle}.json` files and the generated checksum file. All three metadata records agree on module identity, version, timestamp and exact Origin.Hash; all six checksum lines match the diff. Controller-provided reviewed tree `0d728106f295dd28db2dc86d446af186cc43e76a` is the source-review handoff; this review does not repeat the tree/source audit.
- Named risk — new-pin test evidence might overstate passed or skipped coverage: parsed the existing `/tmp/issue58-recovery-{garden,cognitive,c2}.jsonl` outputs and read the bootstrap log summary, without rerunning commands. Counts match the report: bootstrap 11 passed/0 skipped; Garden 70/19 top-level passes and 77/22 pass events; cognitive 29 top-level passes/37 events; C2 22 top-level passes/46 events plus five explicitly skipped PostgreSQL tests. No fail events appeared. Packages without tests are separate package-level skips, not hidden skipped test cases.
- Normal sumdb settings, archive authentication and temporary-modfile `go mod verify` success are reported acquisition evidence. This review checks their retained metadata/checksum consistency without performing fresh network verification or repeating package validation.

## Findings

Critical: none. Important: none. Minor: none.

## Remaining gates

- ⚠️ Five real PostgreSQL tests skipped because the DSN is unset; a passing package result does not establish real-server acceptance (`docs/logs/2026-10-15-issue-58-recap-facade/verification.md:110-114`).
- ⚠️ Native Windows durability, centralized exact-final-candidate S6 CI, and product acceptance remain pending (`verification.md:118-123`; `acceptance.md:20-24`, same directory). Earlier raw-full-suite timeout/VCS evidence is not superseded by these focused passes.
- ⚠️ The previously disclosed 19 local-placeholder missing-ziphash conditions remain outside this increment; temporary-modfile verification is not a claim that root `go mod verify` became clean.

The narrow recovery dependency increment is approved. It does not independently accept S1 durability, S2 behavior, or S6/product completion.
