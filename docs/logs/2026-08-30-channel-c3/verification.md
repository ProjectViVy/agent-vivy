# CH-C3 — verification

Date: 2026-08-30. Worktree: `agent-vivy-channel-c2` (reused sequentially),
branch `feat/channel-c3`, baseline edf24c8 (including C1+C2).

## GOAL execution (subagent roles)

- explore (read-only): scanned the runtime Run/terminal observability surface, events bus,
  Journal constraints (empty run_id invalid + one terminal state per run), app assembly
  sequence, config envelope, importlint coverage, and fake mode.
- executor (writes): implemented the five internal/channelhost files + fake + TCK,
  runtime Provenance seam, app assembly, and capability interface method set.
- reviewer (independent read-only review): **PASS**, no blocker; two should-fix items
  (TODO reconciliation + SDK comment deferral wording) were handled during landing.

## Commands and results

| Command | Result |
|---|---|
| `gofmt -l ./internal ./sdk` / `go build ./...` / `go vet ./...` | Completely clean (executor) |
| `go test ./... -count=1` (executor) | All packages ok: app 8.3s, channelhost 9.3s, runtime 35.6s |
| TCK cases individually with -v | TestStartAllRefusesEmptyAllowFrom / TestPublishInboundDropsSenderNotInAllowFrom / TestPublishInboundAllowedSenderJournalsAndRuns / TestChannelSessionIDDeterministic / TestOnRunEventDeliversAssistantReply / TestOnRunEventFailedDeliversNothing / TestStartAllIgnoresConfigWithoutPlugin / TestDiscoverReportsOnlyImplementedCapabilities all PASS |
| Provenance regression | TestRunWithChannelProvenance / TestRunWithoutProvenanceKeepUISource / TestRunWithEmptyProvenanceSourceRejected PASS |
| Three importlint tests (eino quarantine / plugin window / reference material) | PASS (internal/channelhost automatically covered; test files were manually checked for being eino-free per convention, with reviewer grep confirmation) |
| `just ci` (after branch switch) | **exit 0**: all Go packages ok (channelhost 12.9s, runtime 61.8s, sqlite 44.5s, rpc 29.0s), UI 21 files / 175 tests, vite build green |
| reviewer recheck `go test ./internal/channelhost/... ./internal/app/ ./internal/runtime/` + full tree | All PASS; `git diff edf24c8 --stat` shows only 4 tracked-file changes plus the new set matching the file list; engine.go/plugins//go.mod/go.sum/routeTree.gen.ts untouched |

## Acceptance checklist (CH-C3.md §7)

- TCK all green ✅
- `internal/channelhost` has no eino import (including manual confirmation of test files) ✅
- Default `just run` has no ear and zero real-protocol HTTP
  (Register()=nil → empty channels → StartAll no-op; existing app tests all green) ✅
- Optional-interface files exist and are referenced by Discover; fake implements none ✅
- Empty allow_from refuses Start ✅; unknown `channels.<name>` fails startup ✅

## Honest statement

- The in-memory terminal-delivery window (crash loses the reply), `chanin_*`
  event retention, and Secret not being pinned to token_env are known implementation
  gaps intentionally deferred and recorded in §0.1 (CH-C3-N1/N2), for C4 hardening.
- The reread-convergence path for the EnsureSession creation race has only serialized
  test coverage (reviewer note #7).
- The CH-C3 PLAN says "importlint confirmation"; the existing
  `internal/app/importlint_test.go` provides automatic quarantine coverage (no new
  tool needed), confirmed by both tests and grep.
- Not pushed.
