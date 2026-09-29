# S11-F — VIVY reusable definitions and host actions

> **For agentic workers:** Execute each task with superpowers:executing-plans or subagent-driven-development; use test-driven-development for implementation and verification-before-completion before claiming a gate. Check boxes are tracking, not evidence of completion.

**Spec:** [detailed architecture](../../specs/2026-09-29-inofy-cutover-design.md), [issue #23](https://github.com/ProjectViVy/agent-vivy/issues/23), [package index](./README.md). Baselines are VIVY `3c4ed668` and INOFY `dbfebcec`; re-read current heads, AGENTS.md, and scoped instructions before implementation. All tasks below are **Planned**, with no implementation or passing-test claim. Record actual command/output and commit SHA in the iteration log.

**Global constraints:** VIVY owns Run/session/policy/budget/Journal/Core Storage and UI authorization. INOFY is the only executable graph engine. No old descriptor translator, engine selector, INOFY App service/database, or second agent loop. Keep Eino imports within VIVY `internal/runtime`/`internal/provider`; only runtime imports executable INOFY. Preserve normal chat/direct delegation and native child sessions. Add paired append-only SQLite/PostgreSQL migrations; do not rewrite 033. Do not edit generated UI assembly. Product UI belongs to a selected VIVY UI Module and Recipe.

**Goal:** Make draft, validation, publication and immutable revision admission a VIVY-owned product path using the same INOFY execution boundary.

**Predecessor:** S11-E. **Unlocks:** S11-G. **Repository:** VIVY. **Scope:** Same delivery cycle as S11 core cutover, separately accepted.

**Files:** INOFY `definitions/repository.go`, `definitions/service.go`, `definitions/publish.go` for contracts; VIVY `internal/storage/` paired migrations and repository adapters, runtime product service, host action module and `internal/rpc/control.go` only if host action conventions require it. Follow existing `internal/modules/masks`, `internal/actionhost/masks.go` pattern; inspect actual registry. Do not add a public Port without all required artifacts.

**Contract:** VIVY Core Storage implements draft CAS and immutable publication revision semantics; published revision hash and author/tenant scope are bound to a new Run admission with the same normalized Definition, trusted catalog, ProgramMeta, input and host authority as S11-B/C. Draft edits never change an admitted/published Run. Host action methods cover capabilities/catalog, list/load/update draft, validate, publish/get revision, start/list/get/cancel run, event paging/subscription and guarded resume only when the node catalog actually supports waits. Use host authentication/authorization and native Journal/projection. Connection methods expose existing VIVY provider profile capabilities or return unsupported honestly; do not duplicate credentials.

- [ ] Add failing backend tests for CAS conflict, immutable published revision, permission/tenant isolation, invalid catalog node, revision→run identity, duplicate operation key, event paging/auth, cancel, false wait/resume capability and disabled product capability leaving core task graph working.
- [ ] Run focused `go test ./internal/storage/... ./internal/runtime ./internal/modules/... ./internal/actionhost/... -run 'Definition|WorkflowProduct|INOFY' -count=1`; capture expected failures (adapt package paths to current tree).
- [ ] Implement paired storage schema, host repository/service/action facade, registering through established module contracts. Route published start through S11-B/C/D, never a second execution engine.
- [ ] Verify backend tests and `just ci`; attach storage parity and authorization evidence.

**Review focus:** CAS, tenant boundary, immutable identity, accurate capabilities, no shadow credential store. **Acceptance:** G9 draft/publish/host action backend; browser product path waits for S11-G.
