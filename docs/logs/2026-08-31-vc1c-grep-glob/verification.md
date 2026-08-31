# Verification — VC-1c grep/glob

Host: Windows, ripgrep 14.1.0 present (rg tests run live; fallback tests force
`rgPath = ""` so both engines are exercised regardless).

## Commands and results

| Command | Result |
| --- | --- |
| `go get github.com/bmatcuk/doublestar/v4@v4.10.0 && go mod tidy` | doublestar promoted to direct require block |
| `go build ./...` | clean |
| `go vet ./internal/tools ./internal/runtime` | clean |
| `gofmt -l internal/` | no output |
| `go test ./internal/tools -run "Grep\|Glob" -count=1` | ok — 6 tests (decode/marshal, pattern-required for both tools, readonly specs, proposal previews) |
| `go test ./internal/runtime -run "TestBackendGrep\|TestBackendGlob\|TestServiceGrepToolEndToEnd" -count=1` | ok — 8 tests |
| `go test ./internal/tools ./internal/config ./internal/runtime` | ok (runtime 59s incl. full service suite) |
| `just ci` | pass (exit 0) — full gate after flake mitigation from 30f849c |

## Test coverage map

- `internal/tools/search_test.go`: request decoding, result marshaling,
  `pattern` required (InvokableRun + PrepareProposal), readonly spec flags,
  proposal previews with `orDot`.
- `internal/runtime/search_backend_test.go`:
  - Fallback walk (rgPath cleared): finds nested matches, prunes
    `node_modules`, skips binaries, correct line numbers/content.
  - rg path: same matches plus `.gitignore` honored (both `ignored/` and
    `node_modules/` absent — `--no-require-git` verified in a non-repo
    workspace).
  - Include filter asserted for both engines (only `*.go`).
  - Invalid regex → `invalid grep pattern` error.
  - Path-scoped grep stays under `nested/`.
  - Glob `**/*.go` recursion reaches `nested/deep/`, newest-first ordering
    proven with explicit `os.Chtimes` mtimes; basename matching for
    slash-free patterns; `node_modules` never descended; size/mtime
    metadata present.
  - Glob rejects `..` and `../outside/*`.
  - `TestEinoFilesystemBackendSearchAndEinoMethods` (pre-existing) still
    passes over the refactored `GrepRaw`/`GlobInfo` paths — the Eino
    middleware contract is regression-covered, including `(?i)` patterns.
- `TestServiceGrepToolEndToEnd`: full stack over sqlite with session pinned
  to the strictest `ask` policy — grep and glob run with **no** approval
  interrupt (readonly), journal carries both tool requests and exactly two
  untrusted tool results.

## Skipped slices

- 3015 browser smoke: a live tool call needs a real model provider behind
  the session, which this lane cannot fabricate (no provider credentials on
  the machine; secrets stay out of fixtures per D-010). The scripted-model
  service e2e drives the identical engine → adapter → backend → journal
  stack over sqlite instead. The UI itself did not change.
