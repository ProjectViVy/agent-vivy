# Verification

| Command | Result |
|---|---|
| `gofmt -l internal/channelhost sdk/plugin plugins/telegram` | clean |
| `go vet ./internal/channelhost ./sdk/plugin` | clean |
| `go test ./internal/channelhost -run 'TestSplitRunes\|TestDeliverySplitsAtAdapterRunesLimit' -race -count=3` | PASS |
| `go test ./internal/channelhost/... ./sdk/... -count=1` | PASS (sdk/internal 41s incl. verify gate) |
| `plugins/telegram: go vet ./... && go test ./... -count=1` | PASS |
| `just ci` | (see below) |

## Test notes

- `TestSplitRunes` — 9 cases pinning pass-through (limit ≤ 0), exact
  fit, hard break, newline preference inside the window, newline at
  window start never emptying a chunk, multibyte rune boundaries, and
  reassembly equality.
- `TestDeliverySplitsAtAdapterRunesLimit` — end to end through
  `PublishInbound` → run → `OnRunEvent(completed)` → delivery: a
  61-rune reply with a 50-rune adapter bound lands as 2 envelopes that
  reassemble exactly; a capability-less adapter still gets 1 envelope.

## just ci

Full gate green: fmt-check, go vet, go test ./..., headless-compile,
plugin-ci (telegram module included), ui tsc + eslint + vitest + build.

## ui-e2e skip reason

Channel-host delivery path only — no UI, RPC, or HTTP transport change.
