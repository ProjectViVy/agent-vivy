# Verification — VC-1d multiedit + patch tolerance

## Commands and results

| Command | Result |
| --- | --- |
| `go build ./...` | clean |
| `go vet ./internal/tools ./internal/runtime` | clean |
| `gofmt -l internal/` | no output |
| `go test ./internal/runtime -run "TestApplyStringPatch" -v` | ok — 11 engine tests |
| `go test ./internal/tools -run "MultiEdit" -v` | ok — 4 tool tests |
| `go test ./internal/runtime -run "TestBackendPatchFile\|TestBackendMultiPatchFile\|TestEinoFilesystemBackend" -v` | ok — 9 tests incl. pre-existing patch/glob regressions |
| `go test ./internal/tools ./internal/config ./internal/runtime` | ok (full packages) |
| `just ci` | pass (exit 0) |

## Test coverage map

`internal/runtime/patch_engine_test.go` — the engine matrix:
- exact match wins even when a tolerant match would also apply (exact-path
  semantics unchanged, including its ambiguity error);
- trailing-whitespace tolerance; CRLF preservation through replacement;
- file indentation restored when the model guessed (and kept) its own
  leading whitespace; deliberate reindent respected;
- line-count-differing replacement verbatim; flexible `replace_all` across
  two regions with per-region indentation transfer; trailing-newline
  anchoring stripped;
- ambiguous flexible match errors with the `(whitespace-insensitive)` hint;
  not-found and empty-`old_string` errors unchanged.

`internal/runtime/patch_engine_test.go` (backend half) — real filesystem:
- tolerant `PatchFile` writes the file with restored indentation and a diff;
- `MultiPatchFile` applies two edits with one combined diff and one write;
- a failing second edit leaves the file byte-identical on disk (atomicity);
- a tolerant edit inside multiedit restores file indentation;
- empty edit list rejected.

`internal/tools/multiedit_test.go` — decode/forward/spec/proposal:
- ordered edit decoding with `replace_all` string parsing and run-id
  forwarding; five validation failure shapes (empty list, missing
  `old_string`, identical strings, bad bool);
- mutating spec with required `path`/`edits` params;
- proposal routing: previewer stub wins, generic fallback previews the edit
  count; backend never called during validation failures.

## Skipped slices

- 3015 browser smoke: same provider constraint as VC-1c — multiedit calls
  need a live model to drive them; the mutating-tool approval path it would
  exercise is already covered by the approval e2e suite and the proposal
  unit tests. The board-level VC-1 acceptance walkthrough
  (read → grep → multiedit → bash → diff) lands as its own e2e when the
  remaining VC-1 items (VC-1e/1f/1g) are in.
