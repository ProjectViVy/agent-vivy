# S1 Task 3: SAME owner reconciliation handoff

Date: 2026-10-15. Status: **READY for independent review and Task 4 handoff**;
this is source/test evidence, not product or native Windows acceptance.

## Source and scope

Owner repository: `ProjectViVy/laputa`, branch `feat/issue-58-recap`.
Base: `4c7c6b20859375d0d764d3ba2a0188926cf73c9e`.
Source commit: `9ec9baa4db117c3ea3b4a0949fece14999904895`.
Human author/committer: `mastwet <1136523089@qq.com>`.
Consumer starting point: `d1a7c325261d50689908cea9c3a9eaebfd159afe`.

Exact committed files:

- `laputa/actmem/recap.go`
- `laputa/actmem/recap_format.go`
- `laputa/actmem/recap_history.go`
- `laputa/actmem/recap_reconcile_test.go`

Unchanged public DTOs/methods support `Cause="reconcile"` on the SAME `*Store`.
Admission requires `Sources.After == Sources.Through == current.CoveredThrough`,
nonblank opaque `Sources.Digest`, and `RestoredFrom == 0`. The new revision
preserves coverage and invalidation epoch; text and the entire opaque receipt
retain exact UTF-8 bytes. Writer admission, strict decoding and reachable-chain
transition checks were changed together. There is no host-envelope parser,
source-verification engine, Eino/model dependency, new clock, cursor, schema ID,
second owner or durability path.

Historical plain receipts and request hashing are unchanged. Tests preserve prior
immutable revision bytes, and replay returns the original full receipt after later
edit, restore and clear. Changed text/receipt bytes conflict; stale reconciliation
cannot overwrite clear. Ordinary empty-range summary still fails. Each of edit,
restore and clear rejects Sources and resets a fresh reconciliation certificate.
Persona, WORLD, legacy behavior, locks, platform replacement/recovery, unknown
commit freeze and qualified absence rules were not changed.

## Verification evidence

All commands ran from `/workspace/scratch/243d1cbb859f/laputa/laputa` with
`PATH=/workspace/scratch/243d1cbb859f/toolchains/bin:$PATH`.
Environment: `go version go1.26.4 linux/amd64`, `GOSUMDB=sum.golang.org`,
`GOPROXY=https://proxy.golang.org,direct`. No checksum bypass.

1. Initial required RED: `go test ./actmem -run '^TestIssue58RecapReconcile' -count=1`,
   exit 1. Expected missing-feature error: `invalid Recap cause, source range or restore payload`.
2. Captured RED before production edits: `go test -json ./actmem -run '^TestIssue58RecapReconcile' -count=1`,
   exit 1; 18 failed and 8 passed named tests/subtests, zero skips. Eight passing
   negative admission checks are expected; valid reconciliation setup failed.
3. First post-change focused run exposed a test fixture error: reopen passed the
   ACTMEM directory instead of its configuration parent, yielding revision zero.
   The fixture was corrected to reopen the same owner namespace. No production
   durability workaround was added.
4. Final captured focused GREEN: `go test -json ./actmem -run '^TestIssue58RecapReconcile' -count=1`,
   exit 0; **35 passed, 0 failed, 0 skipped**; package 0.068s.
5. Single final full ACTMEM race run: `go test -race ./actmem -count=1 -json`,
   exit 0; **114 passed, 0 failed, 0 skipped**; package 2.867s.
6. `git diff --check` and staged `git diff --cached --check`: exit 0.

The JSON flag records raw events without changing test selection. Raw evidence
and SHA-256/source-file hashes are recorded in `s1-task-3-verification.json`.

| Evidence | SHA-256 | Named pass/fail/skip |
| --- | --- | --- |
| [s1-task-3-actmem-race.jsonl](s1-task-3-actmem-race.jsonl) | `aa9e24991dc82a5a1f347cd67875b9d80f056602e0b0c2ef471f1a9fdc7ca37f` | 114/0/0 |
| [s1-task-3-reconcile-green.jsonl](s1-task-3-reconcile-green.jsonl) | `4b4fea802659a49250e3eb566243640a65a137b9d57d8a9c060f9d5c7a882ffc` | 35/0/0 |
| [s1-task-3-reconcile-red.jsonl](s1-task-3-reconcile-red.jsonl) | `e0fa122a3e6e637d7ba7a2d4552667bc0803711cab8229889110d4b37b7045a8` | 8/18/0 |

The focused suite covers exact Chinese/CRLF Markdown (including fences and
front-matter delimiters), zero and nonzero coverage, preserved post-clear epoch,
opaque future envelope version/whitespace, entire receipt request digest,
validation without writes, replay/conflict/CAS, and hash-valid malformed format
and transition fixtures. Recovery tests exercise pre-replace failure/cancellation,
post-replace unknown, lost response, all authoritative zero-data surfaces, four
continuing recovery failure seams, live-owner freeze after faults are removed,
exact positive replay after reopen, immutable history bytes and orphan exclusion.

## Review and acceptance limits

Self-review found no remaining in-scope defects. Independent transition/durability
review, publication and compatible consumer pinning remain controller/Task 4 work.
No source push, merge, deploy, live data access or consumer pin/source changes were
performed. This report and evidence are left uncommitted for the controller to
publish without colliding with the architect's consumer HEAD.

Native Windows was unavailable: neither native Windows tests nor Windows acceptance
were performed. The new platform-neutral fault-seam tests will run natively there
and explicitly require unsupported missing-head negative lookup to remain
`recap_commit_unknown`; existing Windows-specific tests remain intact.
No cross-compilation is claimed as native acceptance. Full Garden/Vivy integration
and product `just ci` belong to later tasks and were not run for this owner-only task.

Older readers' strict `decodeRecap` default rejects unknown causes; this is a
source-inspection compatibility conclusion, not a separately executed old-binary
acceptance run. `actmem/v3`, DTO shapes and canonical request hashing remain
unchanged. A compatible published owner must be pinned before host activation.

## Rollback

Disable extended host admission and retain all heads/history. Never open a head
containing reconciliation with an incompatible old owner, rewrite its receipt or
reactivate legacy writers. A source revert is safe only before such heads exist;
after activation retain a compatible reader/writer while disabling new admission.
