# CH-C7b — verification

Date: 2026-08-30. Worktree: `agent-vivy-channel-c2` (reused sequentially), branch `feat/channel-c7b`, baseline 12a2a70 (C7a, on the `feat/channel-c6` line; includes C1–C7a).

## GOAL execution model (subagent roles)

- executor (writes to disk): the complete plugins/qq implementation + review fixes (first-connection report fence, ChanManager wording correction, quiet logger assertion).
- reviewer (read-only independent review): **PASS**; 4 should-fix items (first-connection Stop hang = code fix; ChanManager wording = corrected; quiet logger without an assertion = test added; missing log = this directory). The reviewer checked each of the four source claims against botgo v0.2.1 ((a) dto really lacks group_openid, (c) 11 token failures really panic, (d) Close with a nil connection really panics, (b) the ChanManager panic claim was only partly true—the wording was corrected).

## Commands and results

| Command | Result |
|---|---|
| `gofmt -l` (internal / sdk / plugins/qq) | clean |
| Root `go build ./...` / `go vet ./...` / `go test ./...` | all ok (24 packages; qq is not in the species compilation graph, `go list -deps ./cmd/vivy \| grep -c botgo` = 0) |
| `cd plugins/qq && go vet && go test ./... -count=1` | all PASS (0.65s; includes the real botgo OpenAPI loopback, bounded first-connection Stop, deduplication fence, and quiet logger assertion) |
| `go test -race ./... -count=1` × 5 | all ok (~1.7s) |
| `go run ./sdk verify plugins/qq` | ok |
| `go run ./sdk pack --with qq --out <tmp>` + `go version -m` candidate | candidate EXE links botgo v0.2.1; recipe.plugins=["qq"] |
| `git diff 12a2a70 -- go.mod go.sum` | empty |
| `just ci` | First run failed once in `internal/runtime TestServiceApprovalApproveFlow` (load-related flake: isolated `-run` test ok, package `-count=1` ok, then a full `just ci` rerun was green with **exit 0**; the runtime package has had no changes since C3, and qq only added plugins/qq/) — registered in §0.1 TEST-2 |

## Acceptance checklist (`CH-C7b.md` §7)

- Candidate text loop (real botgo OpenAPI client loopback test) ✅
- No botgo by default ✅
- Empty `allow_from` is rejected (Host regression green) ✅
- Code and documentation both declare not a personal account / not OneBot (package doc + settings comment + README) ✅

## Honest declarations

- Real network WS frame behavior was not tested (botgo's client was used only for link validation; protocol logic is covered with fakes)—same depth as telegram/dingtalk/feishu.
- A reply outside the passive window (no `msg_id` after restart) fails closed with an error and is not retried—the product semantics follow picoclaw's window limit.
- Not pushed.
