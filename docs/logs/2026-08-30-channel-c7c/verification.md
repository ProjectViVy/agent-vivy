# CH-C7c — verification

Date: 2026-08-30. Worktree: `agent-vivy-channel-c2` (reused sequentially), branch `feat/channel-c7c`, baseline d5f4506 (includes C1–C7b).

## GOAL execution model (subagent roles)

- executor (writes to disk): the complete plugins/discord implementation + pion ban + fixture; the review note (explicitly pin LogLevel) was fixed.
- reviewer (read-only independent review): **PASS**; all five discordgo v0.29 source claims were checked and confirmed (Open synchronously reaches READY / reconnect ignores Close / Close emits a synthesized DISCONNECT / named handler function types are not recognized / ChannelMessageSendComplex is pure REST).
- GOAL owner: candidate-pack evidence + landing.

## Commands and results

| Command | Result |
|---|---|
| `gofmt -l` (full tree including plugins/discord and fixture) | clean |
| Root `go build ./...` / `go vet ./...` / `go test ./...` | all ok |
| `cd plugins/discord && go vet && go test ./... -count=1` + `go test -race` | ok (0.3s / 1.4s) |
| `go run ./sdk verify plugins/discord` | ok |
| `go run ./sdk verify sdk/internal/testdata/bad-pion-import` | **exit 1, rejected**: "import of github.com/pion/webrtc/v3 is forbidden: pion/webrtc is banned in plugins (no voice in Vivy channels)" |
| `go test ./sdk/... -count=1` (new fixture + all existing fixtures) | ok |
| `go run ./sdk pack --with discord --out <tmp>` + inspect-artifact + `go version -m` | candidate `gen_4d64767f37cb958e` links discordgo v0.29.0; pion modules in the EXE **0** |
| `go list -deps ./cmd/vivy \\| grep -c discordgo` | 0 |
| `git diff d5f4506 -- go.mod go.sum` | empty |
| Plugin-tree pion grep | 0 (production code has zero references; only comments declare its absence) |
| `just ci` | **exit 0** (all Go packages ok + UI 21 files / 172 tests + vite build) |

## Acceptance checklist (`CH-C7c.md` §7)

- Candidate DM/text-channel text (normalize + dispatch + Send test matrix, loopback/fake fully covered) ✅
- No pion in the dependency graph (species and candidate both verified) ✅
- Empty `allow_from` is rejected (Host regression green) ✅
- Zero porting of `voice.go` / pion / TTS / slash, and the pion ban is effective across every seam in verify ✅

## Honest declarations

- RESUME continuation is missing (session ID/seq are not exported in v0.29): bounded event loss during redial—the priority is deadlock avoidance, an intentional deviation recorded in the README.
- There is a theoretical window in which a callback dispatched by an old session after restart lands in the new lifecycle (same shape as the sibling plugins, bounded).
- `just ci` does not cover `plugins/*` (structural gap, registered as CH-C7c-N1; a `plugin-ci` recipe is recommended).
- The inspect ban-reason string also says "pion/webrtc" for non-webrtc pion modules (decorative; the test pins that substring).
- Real Discord smoke testing was not done (no credentials + the Intent must be enabled in the developer dashboard).
- Not pushed.
