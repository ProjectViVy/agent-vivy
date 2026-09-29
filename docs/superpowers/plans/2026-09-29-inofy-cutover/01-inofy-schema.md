# S11-A — Canonical definition schema (INOFY)

> **For agentic workers:** Execute each task with superpowers:executing-plans or subagent-driven-development; use test-driven-development for implementation and verification-before-completion before claiming a gate. Check boxes are tracking, not evidence of completion.

**Spec:** [detailed architecture](../../specs/2026-09-29-inofy-cutover-design.md), [issue #23](https://github.com/ProjectViVy/agent-vivy/issues/23), [package index](./README.md). Baselines are VIVY `3c4ed668` and INOFY `dbfebcec`; re-read current heads, AGENTS.md, and scoped instructions before implementation. All tasks below are **Planned**, with no implementation or passing-test claim. Record actual command/output and commit SHA in the iteration log.

**Global constraints:** VIVY owns Run/session/policy/budget/Journal/Core Storage and UI authorization. INOFY is the only executable graph engine. No old descriptor translator, engine selector, INOFY App service/database, or second agent loop. Keep Eino imports within VIVY `internal/runtime`/`internal/provider`; only runtime imports executable INOFY. Preserve normal chat/direct delegation and native child sessions. Add paired append-only SQLite/PostgreSQL migrations; do not rewrite 033. Do not edit generated UI assembly. Product UI belongs to a selected VIVY UI Module and Recipe.

**Goal:** Export the library's canonical executable Definition JSON schema so VIVY can present a model-facing supported subset without copying a topology grammar.

**Predecessors:** INOFY S01–S10 accepted; no S11 predecessor. **Repository:** INOFY, branch `docs/s11-vivy-cutover-plan`; its [S11 plan](https://github.com/ProjectViVy/INOFY/blob/docs/s11-vivy-cutover-plan/docs/superpowers/plans/inofy/S11.md) is the INOFY-side pointer. **Unlocks:** S11-B.

**Files:** Inspect `definition.go` and the embedded schema/artifact decoder; add a small exported accessor in the root package (proposed `schema_export.go`) and tests in `definition_test.go` or `schema_export_test.go`. Update API documentation only where the export is described.

**Contract:** `DefinitionSchema() json.RawMessage` is a proposed accessor, with a defensive copy of the same strict schema used by `DecodeArtifact`/`ValidateDefinition`. Determine the actual package schema representation at current head before finalizing signature. The schema describes `inofy.workflow/v1`; host-specific node catalog, tool ceilings and effective limits remain VIVY validation, not a second INOFY grammar.

- [ ] Write a test proving accessor schema accepts a valid normalized Definition and rejects unknown properties, invalid schema version and malformed binding/topology shapes in agreement with canonical decoding. Verify returned bytes cannot mutate library state.
- [ ] Run `go test ./... -run 'Definition|Schema' -count=1` in INOFY; expect the new test to fail before implementation.
- [ ] Export from the existing canonical source, without a duplicate hand-edited schema or separate parser. Document what JSON-schema validation does not replace (catalog/semantic validation).
- [ ] Rerun focused tests and `go test ./...`; attach exact results and commit the enabling API change in INOFY.

**Review focus:** One source of truth; strict decoder still gates admission; no App or editor dependency. **Acceptance:** G1's schema agreement, including an executable consumer example. No VIVY production route changes in this Story.
