# VC-1g-2 verification record

Date: 2026-08-31

## Commands and results

| Command | Result |
| --- | --- |
| `go build ./...` | exit 0 |
| `go vet ./internal/rpc ./internal/runtime ./internal/storage/... ./internal/domain` | clean |
| `go test ./internal/rpc ./internal/runtime ./internal/storage/sqlite ./internal/domain` | ok (including new `TestTurnStartAttachmentsValidationAndRoundTrip`, `TestBuildRunContextProjectsImageAttachments`, and `TestMessagesPersistImageAttachments`) |
| `cd ui && pnpm typecheck` | exit 0 |
| `cd ui && pnpm test` | 24 files / 195 tests passed (new store case: attachments queue with the message and dispatch with the attachment after completion; the existing dispatch assertion was updated to 5 arguments) |
| `just ci` (full gate) | The first run failed in `fmt-check` (control.go struct-tag alignment); after `gofmt -w`, the rerun exited 0 |

## just ci notes

- The first `just ci` exited 1: `fmt-check` reported that `internal/rpc/control.go` was not formatted
  (the struct tag was misaligned after `messageResult` gained a field). `gofmt -w` fixed it;
  rerunning `just ci` exited 0 (log `/tmp/just-ci-vc1g2b.log`).

## Smoke policy

This slice is a user-visible change (UI image pasting/file selection). Under `smoke-for-user-visible-change`, it should use the real path at
`http://127.0.0.1:3015`; however, an end-to-end image run requires a real provider key
(after TEST-1 removed the mock provider there is no local fake model), and this round had no key, so it could not produce a real
vision response. The exception is recorded here according to the usual practice.

Real substitute verification that was performed:

- `internal/rpc` integration tests used the real RPC Handler through the complete path: invalid MIME / invalid
  base64 / empty data / over 5MiB / over 4 images → InvalidParams; valid png → run completes →
  `session/messages` returns the same name and MIME with a fully round-tripped base64 data URL.
- `internal/runtime` asserts that `buildRunContext` projects an image-bearing user message into eino
  `UserInputMultiContent` (text part + image part, correct Base64Data/MIMEType, Content empty),
  and that image bytes are excluded from the text-byte budget.
- `internal/storage/sqlite` asserts attachment persistence, ordering, and DeleteSession cleanup.
- UI store tests assert that attachments pass through `api.startTurn` as 5 arguments when dispatched through the queue.
- At the UI component level, a browser walkthrough with `pnpm dev` + `just run` as a split pair (file selection,
  screenshot paste, thumbnail removal, gate notices, post-send bubble thumbnails) is deferred to the next round with a key,
  together with VC-2 SupportsImages gating (see the manual path in acceptance.md).
