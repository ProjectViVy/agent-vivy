# P2.2: Durable cognitive admission intent and recovery

## Result

Implemented C2 on the isolated branch `feat/issue32-remediation`. The runtime now persists one immutable admission identity before workflow admission and resolves that identity before considering a replacement supervisor or new input window.

## Changes

- New and empty cognitive snapshots use `StateSchema=2`; `Phase` records the admission/settlement outcome. Unsupported schema values fail closed.
- Persisted `Intent` contains the original supervisor Run ID, operation key, strategy ID, canonical input bytes, and bounded attempt. `PendingThrough` is committed in the same snapshot before `StartCognitiveWorkflow`.
- Recovery looks up the exact original parent/key and verifies its canonical input, digest, strategy catalog identity, and window. Active/accepted runs are adopted; completed runs settle exactly their stored Through; cancellation stays fenced.
- Safe retries require a terminal failed workflow projection, known effect-free strategy stages, terminal inference-child runs, and matching model request/mandatory finish records. Missing or malformed evidence, unknown events/effects, and failed lookups are unsafe.
- Legacy ActiveRunID can reconstruct intent from the durable revision. An untracked legacy window with SourceHigh above Watermark (or other outstanding-window evidence) becomes `unknown_outcome`; no new identity or watermark is invented.
- Regressions cover true SQLite close/reopen before admission, after completion and after settlement, same-Service-store active adoption, safe and unsafe retries, cancellation, later accepted input, legacy upgrade, unknown future schema, and replacement supervisor fencing.
- Independent review follow-up closed three recovery gaps: adoption of a deterministic supervisor created before snapshot persistence, fencing of an active native Run with a recovery-required projection, and recovery of the legacy attempt from the operation key. It also covers the native-failed/projection-pending interval so it does not create a permanent false-positive fence.

## Evidence

The fault matrix, cognitive race selection, full `internal/runtime` package, `go vet`, whitespace checks, and SQLite snapshot-version conformance passed. PostgreSQL was skipped because `VIVY_POSTGRES_TEST_DSN` is unset; aggregate `just ci` could not run because `just` is not installed. See `verification.md`.

This is local engineering implementation evidence. It does not claim integrated P2, product acceptance, or release acceptance.
