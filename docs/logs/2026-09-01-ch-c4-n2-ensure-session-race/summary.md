# CH-C4-N2 — EnsureSession concurrent dispatch race test

## What changed

Test-only slice closing TODO CH-C4-N2 (found in CH-C3, deferred from
CH-C4's envelope-only scope): the concurrent-dispatch re-read path of
`Host.EnsureSession` had no concurrency test.

Two tests in `internal/channelhost/host_test.go`:

- `TestEnsureSessionConcurrentSameChat` — 32 goroutines race
  `EnsureSession` for the same chat against a fresh backend behind a
  start barrier. Asserts every caller gets the same deterministic id
  with no error, exactly one session row exists, and the title is the
  canonical `channel/<channel>/<chat>`. This is the direct exercise of
  the read→create→re-read path: losers of the unique insert must
  re-read the winner's row instead of surfacing the constraint error.
- `TestConcurrentInboundSameChatDispatch` — the end-to-end face: 8
  concurrent `PublishInbound` calls (distinct MessageIDs, same chat)
  through the full pipeline (allow-list → EnsureSession → journal →
  run → delivery tracking). Asserts all 8 runs open on the one
  deterministic session with channel provenance, run ids are distinct,
  8 `channel.inbound` journal commits land, 8 user messages persist,
  and exactly one session row exists.

No production code changed: the re-read path (session.go) and the
mutex-guarded pipeline were already correct; the tests pin the
contract. The sqlite backend's `SetMaxOpenConns(1)` (D-027) serializes
statements, so no lock-contention flake is possible; the read-create
race window is between statements, which is exactly what the barrier
maximizes.

## Explicitly not done

- No MessageID dedup added (replay protection is a separate concern;
  CH-C3-N1's outbound durability row stays open).
- No topic-scoped concurrent variant (the mapping is deterministic over
  the same triple; the same-chat case is the interesting race).
