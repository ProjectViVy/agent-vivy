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

- No push, merge to `main`, tag, deployment or tenant-data access.
- No `reset`, `clean` or overwrite of existing work; `main` stays at `5715950d`.
- No reimplementation or "recovery" claim for historical work: the reviewed
  Laputa owner was obtained from the locked upstream commit and verified by tree
  identity. Nothing was rebuilt.
- No change to the pinned Laputa tree. The upstream test-helper patch was applied
  only as a reversible local experiment to isolate the cause, then reverted; the
  pristine `bb57e93` checkout was restored and verified byte-identical.
- S3/S4/S5/S6 remain unimplemented; no resident LIFE activation is claimed.
- `#66`'s sidebar producer is not implemented here. This lane consumes the
  governed seam only.

## Known gaps carried forward

| Gap | Status |
|---|---|
| Real PostgreSQL migration parity and snapshot isolation | Pending — no DSN, docker or psql locally; skips never counted as pass |
| `-race` concurrency/durability gates | Blocked — no mingw-w64 C toolchain on this machine |
| Native Windows durability for the pinned Laputa owner | Blocked — needs a published upstream test-helper patch and re-pin |
| Garden module tests | Unavailable natively on Windows (`renameio` is `!windows` upstream) |
| `just ci` on a frozen candidate | Open — this step ran focused and broad gates instead |
| S3–S6 implementation, `#66` producer, installed/realmodel acceptance | Not started |
