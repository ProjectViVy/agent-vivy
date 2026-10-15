# Issue #58 takeover: pinned baseline recovery

## What changed

This increment recovers the reviewed Issue #58 dependency closure and establishes
a baseline that is reproducible **on this machine**, then fixes the one defect that
made the recovered baseline untrustworthy locally.

- `internal/storage/conformance/subject_history.go` — the shared C2 conformance
  fixture `Issue58Identity` now registers the organism identity through
  `domain.SubjectIdentity` with a native canonical owner path instead of a
  hard-coded POSIX literal.

No product behaviour, contract, schema, migration, dependency pin or RPC surface
changed. `domain.SubjectIdentity.Valid` and every admission check are untouched.

## Scope and reasoning

`domain.SubjectIdentity.Valid` requires `filepath.IsAbs(OwnerPath)` and
`filepath.Clean(OwnerPath) == OwnerPath`. The fixture previously wrote
`"owner_path":"/trusted/owner"`. On Windows `filepath.IsAbs("/trusted/owner")` is
false, so the stored identity was invalid, every `identity()` comparison returned
`storage.ErrSubjectBinding`, and the entire C2 conformance suite failed on this
platform. Production never supplies a POSIX literal: `canonicalOwnerPath` runs
`filepath.Abs` and resolves existing ancestors, so this was a test-fixture
portability defect, not a production defect.

The fix therefore changes the fixture, not the validator. Weakening `Valid()` to
accept a non-native path would have traded a real identity invariant for a green
run, which this lane does not do.

## Explicitly not done

- No push, merge to `main`, tag, deployment or tenant-data access for this
  repository. The single authorized push in this delivery is the upstream Laputa
  test-helper fix described below.
- No `reset`, `clean` or overwrite of existing work; `main` stays at `5715950d`.
- No reimplementation or "recovery" claim for historical work: the reviewed
  Laputa owner was obtained from the locked upstream commit and verified by tree
  identity. Nothing was rebuilt.
- The Laputa test-helper fix was first isolated as a reversible local experiment,
  then — after explicit owner authorization — published as a real upstream commit
  and followed by the coordinated re-pin. It changes one test helper only.
- S3/S4/S5/S6 remain unimplemented; no resident LIFE activation is claimed.
- `#66`'s sidebar producer is not implemented here. This lane consumes the
  governed seam only.

## Known gaps carried forward

| Gap | Status |
|---|---|
| Real PostgreSQL migration parity and snapshot isolation | Pending — no DSN, docker or psql locally; skips never counted as pass |
| `-race` concurrency/durability gates | Verified locally on Windows — portable WinLibs mingw-w64 (GCC 16.2.0 UCRT) via `winget`; focused `TestIssue58` and Laputa `actmem` ran with zero data races. Not yet wired into the `just` recipes |
| Native Windows durability for the pinned Laputa owner | Verified on Windows at the published pin; see `verification.md` |
| Garden module tests | Unavailable natively on Windows (`renameio` is `!windows` upstream) |
| `just ci` on a frozen candidate | Open — this step ran focused and broad gates instead |
| S3–S6 implementation, `#66` producer, installed/realmodel acceptance | Not started |

## Published upstream change and re-pin

`ProjectViVy/laputa` `feat/issue-58-recap` advanced `bb57e93 → 8c5328d`. The commit
`8c5328daa7a4e9cdd089d78858de087a36fb0843` excludes the lifetime owner lock from the
`issue58Files` test snapshot in `laputa/actmem/recap_test.go`, because Windows holds
that lock under an exclusive byte-range lock and `os.ReadFile` on it fails before any
assertion runs. Durable-content assertions are unchanged and no production path is
touched.

This repository then advanced all three pins together to
`v0.0.0-20261015195414-8c5328daa7a4`: `go.mod` (three requires), `go.sum` (six lines,
cross-checked against a canonical download of the published version) and
`laputa-source.lock.json`. `just ensure-laputa` and `just bootstrap-test` both pass on
the new pin.

Note on scope: this makes the pinned Laputa tests *runnable* on Windows. It is not by
itself evidence of durability — Windows durability still rests on the owner's own
crash, recovery and reconcile suites.
