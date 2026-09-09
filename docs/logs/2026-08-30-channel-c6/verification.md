# CH-C6 — verification

Date: 2026-08-30. Worktree: `agent-vivy-channel-c2` (reused sequentially),
branch `feat/channel-c6`, baseline d3d4342 (including C1–C5).

## GOAL execution (subagent roles)

- explore: prepared by C4/C5 (telegram template + Host surface); this slice directly
  used picoclaw as the reference.
- executor (writes): the first attempt was interrupted by timeout (only go.mod/go.sum
  skeleton remained); a new executor was dispatched to complete the full list.
- reviewer (independent read-only review): **PASS**; two code-level should-fix items
  (late-callback fence after Stop; webhook-token leakage in the replyText parse-error
  path, D-010) were fixed by the executor and covered by tests.
- No browser smoke needed (no UI change); real DingTalk organization smoke is
  pre-release human acceptance (no credentials).

## Commands and results

| Command | Result |
|---|---|
| `gofmt -l` (internal / sdk / plugins/dingtalk—justfile fmt-check excludes plugins/, so this was manual) | Clean |
| Root `go build ./...` / `go vet ./...` / `go test ./...` | 24 packages ok; DingTalk SDK is not in the product build graph (`go list -deps ./cmd/vivy \| grep -c dingtalk` = 0) |
| `git diff d3d4342 -- go.mod go.sum` | Empty |
| `cd plugins/dingtalk && go vet && go test ./... -count=1` | All 10 tests PASS (including real SDK loopback `TestStreamLoopbackLifecycle`, stop fence `TestHandlerFencedAfterStop`, and D-010 two-phase redaction assertions) |
| `go test -race ./... -count=1` | ok (1.2s) |
| `go run ./sdk verify plugins/dingtalk` | ok |
| `go run ./sdk pack --with dingtalk --out <tmp>` + `inspect-artifact` | Candidate EXE links the SDK (UserAgent/gateway-domain strings present), recipe.plugins=["dingtalk"] |
| `go test ./sdk/... -count=1` (4 new fixtures) | ok |
| `go test ./internal/channelhost/... -count=1` (Secret extension regression + unchanged telegram semantics) | ok |
| `just ci` | **exit 0**: all Go packages ok (channelhost 9.9s, rpc 20.4s, runtime 41.3s, sdk/internal 9.7s), UI 21 files / 172 tests, vite build green |
| `git diff -- go.mod go.sum` (final) | Empty |

## Acceptance checklist (CH-C6.md §7)

- Candidate direct-message text loop: loopback test proves
  ticket→handshake→CALLBACK→inbound→sessionWebhook reply→no redial after Stop ✅
- Default body has no DingTalk dependency ✅
- Empty allow_from refusal (Host behavior, regression green) ✅
- No public webhook mode (transport is poll only; no Listen; no webhook grant) ✅

## Honest statement

- The two reviewer should-fix items were fixed: late-callback fence
  (+`TestHandlerFencedAfterStop`) and webhook-token redaction on the NewRequest
  parse-failure path (+ assertions for both failure phases; net/url permits numeric-port
  parsing and fails only during dialing, so the tests use one case per phase).
- The first executor timeout left only the go.mod/go.sum skeleton; the re-dispatched
  executor completed the full set, with no half-finished work included.
- The maximum ~50s Stop×redial mutual-exclusion window and silent redial for a dead ear
  are documented/recorded; see the not-done section of summary.
- Not pushed.
