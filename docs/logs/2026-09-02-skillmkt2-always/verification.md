# Verification — SKILL-MKT-2

Commands and results (2026-09-02, Windows, repository root):

- `gofmt -l internal/runtime/` → clean after `gofmt -w` on the new files.
- `go build ./...` → BUILD-OK.
- `go vet ./internal/runtime/` → clean (caught a helper-name collision with
  `skills_backend_test.go`'s `writeSkillFixture` during development; renamed
  the new fixture helper to `writeAlwaysSkillFixture`).
- `go test ./internal/runtime/ -run 'TestEinoSkillBackendAlwaysSkills|TestRenderSkillDocumentPreservesAlways|TestEngineAlwaysSkills' -count=1`
  → ok 0.247s (selection, slug order + budget truncation, canonical
  re-render, engine injection idempotency, bare-backend negative).
- `go test ./internal/runtime/ -race -count=1` → ok 108.720s (full package,
  race detector).
- `just ci` (background, log `/tmp/ci-skillmkt2.log`) → **CI-EXIT:0**.

## Real-path smoke note

No UI surface changes in this slice, so no 3015 browser smoke applies (rule
targets user-visible UI/executable surfaces; the model-facing injection is
exercised at its real surface by the engine-level test, which runs the true
Eino middleware chain with a recording model — `TestEngineAlwaysSkillsInjection`
asserts the exact injected message shape and placement on every model call).

## Acceptance wiring

See `acceptance.md` for the manual ORCHID-probe recipe.
