# MASK-1 Task 1 report

## Changed files

- `internal/maskcontract/masks.go`
- `internal/maskcontract/masks_test.go`
- `internal/modules/masks/catalog.go`
- `internal/modules/masks/catalog_test.go`
- `internal/modules/masks/prompts/mask-frame.md`
- `internal/modules/masks/prompts/programmer.md`
- `internal/modules/masks/prompts/researcher.md`
- `internal/modules/masks/prompts/writer.md`

## Decisions

- Added the MASK-C1 domain, management, and resolver values with explicit JSON
  tags, safe allowlisted errors, revision/reference metadata, and cause
  unwrapping that never includes cause text in `Error()`.
- Normalization trims names, normalizes CRLF/CR to LF in bodies, preserves
  literal Markdown/template-looking content, enforces byte limits after
  normalization, rejects invalid UTF-8/NUL/disallowed controls, and validates
  UUID/custom/reserved IDs.
- Digests use SHA-256 over deterministic `json.Marshal` structs with fixed
  field order and no trailing newline. Definition digests cover only ID/name/body;
  create request digests exclude OperationID and include description.
- Built-ins are loaded from package-owned `embed.FS` assets only. The catalog
  returns owned slices/value copies, sorts IDs lexically, starts revision at 1,
  and binds each digest to the supplied Generation ID. Missing assets and
  duplicate IDs fail closed.
- Markdown framing establishes runtime/persona precedence and explicit
  task-over-style precedence. Built-in bodies cover scoped coding/tests,
  evidence/inference/uncertainty, and audience/format/fact discipline.

## Commands and results

- `go test ./internal/maskcontract -run TestMask -count=1` — unavailable:
  `/bin/bash: go: command not found`.
- `go test ./internal/maskcontract ./internal/modules/masks -count=1` —
  unavailable for the same missing Go executable; this is environment
  unavailability, not a test failure.
- `git diff --check` — passed.
- Static review confirmed all imports are used, only the requested Task 1
  paths plus this report are changed, and the embedded asset paths match the
  `go:embed` pattern.

## Concerns

- Go compilation and unit-test execution remain unverified until a Go toolchain
  is available. No `go` or `gofmt` executable is present in this environment.

## Review follow-up

- Added `internal/storage/masks.go` with the MASK-C1 `MaskStore`, admission
  values, and `RunAdmissionStore` declarations only; no backend or success stub
  was added.
- Restricted catalog loading to the unexported `loadCatalog` test seam and
  embedded-only `NewCatalog`; removed the package-level fail-open asset helper.
- Replaced the conditional literal-body test with an unconditional quoted-name
  and template-looking-body fixture that asserts exact resolver output.
- Follow-up static check: `git diff --check` passed. Go test commands were not
  rerun because the executable remains unavailable.

## Review round 2 follow-up

- Manually aligned `internal/storage/masks.go` struct fields to repository
  `gofmt` spacing conventions, including `MaskCaptureCheck` and
  `RunPromptSnapshot`.
- The quoted-name/literal-body fixture now independently asserts the resolved
  digest against `DefinitionDigest(quotedID, quotedName, literalBody)`.
- `git diff --check` remains clean; Go formatting and tests are still
  unavailable because neither `gofmt` nor `go` exists in the environment.
