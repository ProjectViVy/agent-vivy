# Verification — Channel Text Loop (tier-1)

Commands and outcomes, in batch order. Everything below ran on
`feat/channel-tier1` (Windows, Go toolchain pinned by the repository).

## Rebase onto gate-0

| Step | Command | Result |
|---|---|---|
| Integrate batch 0 | `git rebase feat/channel-gate0` | 5 commits replayed onto `0ab4854`; conflicts resolved: dispatch.go (approval hunks vs fence-aware splitRunes, disjoint regions), plugin manifests (both sides re-pinned self-describing digests), telegram plugin.go (gate-0's MaxMessageRunes removal wins over CH-R-1's insertion point; Health kept), P9 pin files |
| Build + vet after rebase | `go build ./... && go vet ./internal/channelhost/ ./internal/app/` | PASS |
| Plugin modules | `go build ./... && go vet ./...` per `plugins/{telegram,discord,qq,dingtalk,feishu}` | PASS |
| Evidence refresh | `go run ./sdk/internal/cmd/source-hash internal <declared>` + per-plugin fixed points, then `go test ./sdk/internal/conformance/ ./sdk/internal/assembly/` | PASS (`b835312d` internal; five fixed points verified) — committed as `a86b063` |
| Capability expectation flip | assembly `channel_capability_test.go` moved to `{Health: true}` per ear (the gate-0 row anticipated this flip) | PASS |

One rebase iteration was aborted and redone: the first conflict pass used a
`\n`-anchored resolution while `conformance_results.json` is CRLF in the
working tree, leaving conflict markers in the replayed commit. The abort
left no residue; the redo resolved with CRLF-tolerant patterns.

## Feature gates (focused runs after each feature)

| Feature | Command | Result |
|---|---|---|
| Typing (host) | `go test ./internal/channelhost/ -run 'TestTyping\|TestNoTyping' -count=1 -v` | PASS — starts/stops at completed, failed, StopAll, and the cap; no face starts nothing; delivery unaffected |
| Typing (adapters) | per-plugin `go test ./... -run TestTyping` | PASS — telegram chat-action wire shape, discord seam ping, qq InputNotify anchored to the passive window (and skipped without one) |
| Markdown (telegram) | `go test ./... -run TestMarkdownToHTML` (converter table) + `go test ./...` (Send HTML + plain fallback) | PASS |
| Markdown (dingtalk) | `go test ./... -run TestSendMarkdownFallsBackToPlainText` + full suite | PASS — markdown first attempt, errcode-driven text fallback |
| Markdown (qq) | `go test ./... -run TestSendMarkdownFlagWithFallback` + full suite | PASS — msg_type 2 anchored to the window; rejection burns one seq and retries as text |
| Reply threading | `go test ./internal/channelhost/ -run TestReplyThreadsFirstChunkOnly` + adapter threading tests | PASS — first chunk quotes `m-trigger`, follow-ups unthreaded; telegram reply_parameters, discord MessageReference, qq anchor = ReplyTo msg_id (verified against a superseded window) |
| Group triggers | per-plugin suites incl. new group tests | PASS — mention-only gate per ear, stripping, DM/p2p paths ungated, qq group round-trip routes to `/v2/groups/{group_openid}/messages` |
| Batch-wide | `go test ./internal/channelhost/ ./internal/app/ ./sdk/internal/assembly/ -count=1` + five plugin suites | PASS |

## Full gate

| Command | Result |
|---|---|
| `just ci` (fmt-check, ui-ci, vet, test, headless-compile, plugin-ci) | PASS — including the P9 pin gate over the final tree (`98f4026e` internal; telegram `01af8a2c`, discord `a78e7afc`, qq `5b3ecf61`, dingtalk `2bb37756`, feishu `987d1420`, all self-describing fixed points re-verified) |

## Real-path smoke

Honestly skipped: no bot tokens exist in this environment for any of the
five platforms, so live typing/markdown/reply/group behavior could not be
exercised against real deployments. In its place:

- **qq group payload verification (required by the handoff before coding):
  the official v2 event table was checked** —
  [GROUP_AT_MESSAGE_CREATE field table](https://bot.q.qq.com/wiki/develop/api-v2/autogen/event/group_at_message_create.html)
  pins `group_openid`, `author.member_openid`, `content` (the platform
  strips the @bot prefix), `id`; the send contract
  ([POST /v2/groups/{group_openid}/messages](https://bot.q.qq.com/wiki/develop/api-v2/autogen/api/v2_groups_group_openid_messages.post.html))
  confirms the passive `msg_id` anchor. This is exactly the shape the
  plugin's local decode struct implements, and it confirms the botgo
  v0.2.1 `group_id` defect is avoided rather than patched.
- Every wire shape (chat action, typing endpoint, markdown payloads,
  reply parameters/references, passive anchors and seq discipline, group
  routing) is asserted against loopback stubs that speak the real SDK
  clients: telego over an httptest Bot API, the real botgo REST client
  over a loopback domain, the governed DingTalk stream client over a
  websocket stub, discordgo through the session seam, and the lark SDK
  over a full TLS stub.
- A real-path smoke with one owned bot per platform remains the standing
  acceptance step for the next operator who holds tokens (see
  acceptance.md).
