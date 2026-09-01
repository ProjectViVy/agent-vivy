# Verification — VC-3 slice 5 (read_file images)

Date: 2026-09-01. Worktree `agent-vivy-vc0`, branch `feat/vc1a-bash-tool`.

## Commands and results

| Command | Result |
| --- | --- |
| `go test ./internal/tools/` | ok (1.086s) — includes new `TestReadFileReturnsImagePartsEnvelope` (base64 roundtrip), `TestReadFileTextResultHasNoEnvelope` |
| `go test ./internal/runtime/` | ok (69.7s) — includes new `TestImageMIMEByExtension`, `TestReadFileAttachesImageBytes` (png bytes + text read unchanged), `TestReadFileRejectsOversizedImageInsteadOfTruncating`, `TestNormalizeEnhancedResultKeepsImageOverBudget` (text compacted, image kept), `TestNormalizeEnhancedResultRejectsUnknownPart`, `TestToolAdapterRunPassesEnvelopeUncompacted` |
| `gofmt -l internal/tools internal/runtime` | clean |
| `go vet ./internal/tools/ ./internal/runtime/` | clean |
| `just ci` | pass (CI_EXIT=0) — full gate: go build/vet/tests + UI build |

## Fix during slice

`tooladapter.go` envelope probe initially ran after the untrusted header was
prepended, so `json.Unmarshal` always failed and envelopes were still
byte-compacted — caught by `TestToolAdapterRunPassesEnvelopeUncompacted`,
fixed by probing the header-stripped string.

## Notes

- One earlier `go test` invocation ran in the root tree by accident (shell
  cwd reset); results were discarded and the suite re-run in the worktree.
- Kernel-only slice: `plugins/lsp` untouched, so no `vivy-sdk verify/pack`
  was required this slice.
- Real-provider image smoke (vision roundtrip against a live model) not
  run: no provider credentials in this worktree. Framework-level evidence:
  eino v0.9.13 `adk/wrappers.go` `toolResultToBlocks` was read in the
  module cache and preserves `ToolResult.Parts` image blocks to provider
  messages.
