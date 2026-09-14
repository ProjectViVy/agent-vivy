# PLG-P8 Gates B/C verification

Date: 2026-09-13

Go 1.26.4 was used with `GOPROXY=https://proxy.golang.org` and
`GOFLAGS=-buildvcs=false`; the latter is required only by nested builds in this
linked-worktree layout.

## SCX conformance

- `go test ./internal/contexthost ./internal/observerhost ./internal/runtime ./internal/app ./sdk/internal/assembly`
  — PASS.
- `go test -race ./internal/contexthost ./internal/observerhost` — PASS.
- `go test -race ./...` in `plugins/scx-reference` — PASS.
- `go vet ./...` in `plugins/scx-reference` — PASS.
- `TestPackAndInspectSCXCandidateIsDeterministicAndRemovable` — PASS; two
  rebuilds have the same Generation, source identities, and binary bytes, and
  the removal Recipe contains neither Module nor generated binding.
- `TestSCXCandidateRollbackRestoresPriorSealedGenerationWithoutJournalMutation`
  — PASS; packed health probe, two Studio installs, rollback, and Journal
  sentinel preservation are exercised rather than inferred.

The Runtime integration test uses the real `scxreference.Provider` on both the
Context Source and receipt-aware Observer paths. It verifies scoped Context
projection into the model input, a committed terminal projection, bounded
summary/View fields, redaction, and one logical update.

An additional app-wide race sweep reaches a known race inside pinned
`github.com/cloudwego/eino-ext/components/model/claude@v0.1.25` during the
pre-existing loopback approval test. The same focused test reproduces unchanged
on current `main`; no P8 file appears in either racing stack. It is not a PLG-P8
gate. The P8-owned ContextHost, ObserverHost, Runtime, and SCX provider race
paths pass.

## Complete repository gate

- tracked Go formatting check — PASS.
- plugin-v1 validator tests and eight-case fixture corpus — PASS.
- `go vet ./...` — PASS.
- `go test -timeout 20m ./...` — PASS; `sdk/internal` completed in 518.417s.
- headless compile for `cmd/vivy`, `cmd/vivy-code`, and `ui` — PASS.
- per-module `go vet ./...` and `go test ./...` for every standalone Module in
  `plugins/` and `faces/` — PASS.
- UI typecheck, 316 tests, Vite production build, 1,396-key catalog
  completeness, and eight cross-face i18n tests — PASS.
- `git diff --check` — PASS.

## Sealed artifacts

`pack` and `inspect-artifact` agree on each final identity:

| Recipe | Generation ID | Expected state |
| --- | --- | --- |
| `recipes/scx.vivy.yml` | `40c2e3c2302a23b277bab5db159d2bbd59d8af60477ce6f568ce81c5d5d3e5a5` | `scx/reference-fixtures@0.1.0` present; required Source and terminal Observer policies sealed |
| `recipes/default.vivy.yml` | `b5f5b71eb388974cb8b0fef58b077046ad1ae5790fa58c29b5f8e4ae8104469d` | default Context/Skill and channel composition retained; SCX reference Module absent |
| `recipes/minimal.vivy.yml` | `647278455905b6c1bc83abdbf8b48a546d1eacb7fcd28bd7a007d2051db714ad` | Context/Observer optional graph omitted; channels and MCP `NOT_COMPILED` |

SCX artifact details:

- recipe digest:
  `ab5219f0c3c2d8896320c0143da401b4a76b31413809db2c8e8c41119f52074c`;
- Module source:
  `repo:plugins/scx-reference` at
  `3abef450f9dd9ccd7735e0a2d2db13421db53d4b0297bd9f50c1af3ce127d665`;
- internal source:
  `c5a989e920b226666eeff965029f980e5a5f339f7010dd9eca3e77091c26894c`;
- packed binary:
  `4e57ce39c927ed452348f4d3a98c2f3611145e98e7055fdd522bd2db0e494d0d`.

An independent reviewer reran focused suites, race checks, deterministic
pack/inspect/removal, packed health, and actual Studio rollback against the
frozen worktree and reported no correctness, security, authority, lifecycle,
or evidence blocker.
