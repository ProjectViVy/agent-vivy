# Architecture Design SOP — AGENT-VIVY V0

> **Status:** v0.1, draft
> **Updated:** 2026-08-06
> **Owner:** 📋 John (PM) → 🏗️ Winston (Architect, when activated)
> **Workflow source:** BMad `bmad-create-architecture` v6.9 (8-step micro-file workflow at `bmad-method/3-solutioning/bmad-create-architecture/`)
> **Inputs:** `prd-agent-vivy-v0.md`, `AGENT-VIVY-DIRECTION.md`, `AGENT-VIVY-ASSEMBLY-OPTIONS.md`, `REFERENCE-INDEX.md`

---

## 1. Why this document exists

BMad's `bmad-create-architecture` is a **collaborative, A/P/C-driven, 8-step micro-file workflow** with hard rules:

- Each step must show analysis before action.
- Each step ends with **A (Advanced Elicitation) / P (Party Mode) / C (Continue)** choices; the next step is **forbidden** until C is selected.
- It uses append-only document building with state in `frontmatter.stepsCompleted`.
- The skill is opinionated: starter evaluation (Step 3) is **skipped** for existing-workspace projects, and decisions (Step 4) are **delta-focused** for codebases that already have a locked stack.

AGENT-VIVY V0 is neither a greenfield project nor an existing-workspace extension. It is a **third case**: a new product line whose "stack" is partly locked by philosophy (PRD §5.0) and partly open by intent (PRD D-006 says single Go module, but the long-term shape is "module = one well-bounded unit").

This SOP records **how we will run the BMad workflow for AGENT-VIVY V0**, calling out where we follow it as-is, where we adapt, and where we explicitly diverge.

## 2. Inputs the workflow must see

Before Step 1 (Initialization), confirm these artifacts are loaded and recorded in `frontmatter.inputDocuments`:

| Artifact | Path | Status |
|---|---|---|
| PRD V0 | `diva-go/prd-agent-vivy-v0.md` (mirror at `_bmad-output/planning-artifacts/prds/prd-agent-vivy-v0.md`) | Verified: 36.5 KB, 477 lines, v0.4 |
| Product Direction | `diva-go/AGENT-VIVY-DIRECTION.md` | Verified: 8.8 KB, 205 lines |
| Assembly Options | `diva-go/AGENT-VIVY-ASSEMBLY-OPTIONS.md` | Verified: 11.5 KB, 315 lines |
| Reference Index | `diva-go/REFERENCE-INDEX.md` | Verified: 16.3 KB |
| Existing Capability Inventory | **⚠️ PENDING** — not yet authored | Open Question PRD-OQ-1 |

**Mandatory pre-flight check (per P9):**

- Confirm all four files above exist with the byte counts shown. Do **not** proceed if any is missing or stale; the workflow's `inputDocuments` array must be populated truthfully.
- Re-read PRD §13 (Decision Log) before starting Step 1; D-001 through D-025 are the philosophical and product constraints that the architecture must honor.
- Confirm with the user that the PRD's "philosophy preserved, implementation not inherited" stance (D-014..D-021) is still the active constraint. If the user has changed direction, stop and re-converge before running the workflow.

## 3. How each BMad step runs for AGENT-VIVY V0

### Step 1 — Initialization

- **Follow as-is.** Detect existing workflow state, discover input documents, copy `architecture-decision-template.md` to `_bmad-output/planning-artifacts/architecture.md`.
- **Adaptation:** list the four verified artifacts above in `frontmatter.inputDocuments`. Mark the Pending Capability Inventory as `missing — REQUIRED for downstream capability proposals, not blocking for V0 architecture`.
- **Output:** initialized `architecture.md` with frontmatter `stepsCompleted: [1]`.

### Step 2 — Project Context Analysis

- **Follow as-is.** Analyze PRD, extract FRs, assess scale.
- **Adaptation:** explicitly enumerate the **four philosophical anchors** from PRD §5.0 in the "Cross-Cutting Concerns" section. These are non-negotiable inputs to every subsequent architectural decision.
- **Adaptation:** classify the project's complexity as **low-to-medium at the V0 slice**, **medium-to-high at the long-term target**. The workflow template assumes one complexity verdict; AGENT-VIVY has two.
- **Output:** context analysis appended; `stepsCompleted: [1, 2]`.

### Step 3 — Starter Template Evaluation

- **Skip**, but for a different reason than the existing-workspace pattern.
- The BMad skill says: skip if the project extends an existing codebase. AGENT-VIVY is greenfield, but **the stack is already constrained** by philosophy:
  - Go is locked (PRD §10, philosophy "infrastructure is boring and fast").
  - Eino is locked as execution framework (D-003).
  - V0 ships as a single Go module (D-006).
- **Document this skip explicitly.** Write the Starter section as:

  ```markdown
  ## Starter Template Evaluation

  ### Primary Technology Domain
  Backend service + browser UI shell, personal-gateway deployment.

  ### Starter Options Considered
  None. The stack is locked by PRD §10 and D-006:
  - Language: Go (locked)
  - Execution framework: Eino (locked, D-003)
  - Module count: 1 (locked, D-006)
  - UI: separate browser shell, no starter required (D-013)

  ### Selected Starter: (none)
  **Rationale:** The PRD constrains the foundation more strictly than any starter template would. A starter would also bring defaults (e.g. its own config framework, logger, dependency injection shape) that conflict with the philosophy of "logs as first-class citizens" and "provider YAML pre-prepared" — adopting a starter risks re-introducing exactly the architectural debt the project exists to avoid.
  ```

- **Output:** Starter section written as a documented skip; `stepsCompleted: [1, 2, 3]`.

### Step 4 — Core Architectural Decisions

- **Adapt heavily.** This is where V0 differs most from the BMad default.
- The default Step 4 invites collaborative decisions across Data, Auth, API, Frontend, Infrastructure. For AGENT-VIVY V0:
  - **Most decisions are already constrained** by the PRD's Keep/Adapt/Defer/Drop decisions D-001..D-025 and the four philosophical anchors.
  - **The remaining delta** is concentrated in:
    - **D-001** Execution layer (Eino adapter shape — what goes in `internal/runtime` vs what stays in Eino).
    - **D-002** Persistence layer (SQLite schema, migration discipline, event store, retry semantics).
    - **D-003** API contract (HTTP vs IPC, JSON schema versioning, SSE shape for events).
    - **D-004** UI shell (Vite + browser, or a different browser stack).
    - **D-005** Observability surface (log format, structured error categories, correlation IDs).
    - **D-006** Secrets boundary (env-only vs OS keystore vs keyring).
- **Do NOT re-decide** the philosophical anchors. Do NOT propose alternative provider sets. Do NOT propose alternative execution frameworks.
- **Use the existing-workspace-pattern.md reference** for the "Already Decided" table structure: list D-001..D-025 + philosophy anchors as "Already Decided" and concentrate the collaborative session on the six delta decisions above.
- **Output:** decisions documented with rationale, version (where applicable), and affected components; `stepsCompleted: [1, 2, 3, 4]`.

### Step 5 — Implementation Patterns & Consistency Rules

- **Adapt.** The default Step 5 covers naming/structure/format/communication/process patterns. For V0, **focus on the patterns that prevent drift between AI agents working on different slices** of the V0 vertical.
- **Mandatory pattern categories for V0:**
  - **Naming:** Go package names, JSON field names (`snake_case` for API, `PascalCase` for Go structs, event types as `domain.action`-style strings), SQLite table/column names.
  - **Structure:** tests live next to source (`*_test.go`), no `internal/test` shadow tree, fixtures in a single `fixtures/` directory at repo root.
  - **Format:** API response envelope (success: `{ "ok": true, "data": ... }`, error: `{ "ok": false, "error": { "code": ..., "message": ..., "cause": ... } }`); event JSON envelope; provider YAML field ordering rule (must match D-024's field vocabulary).
  - **Communication:** `RunEvent` vocabulary (the ten events from PRD FR-5 are locked), SSE channel naming, UI event subscription pattern.
  - **Process:** error cause categories (provider / tool / permission / cancellation / internal), log correlation identity (`session_id` + `run_id` + `event_seq`), structured log format.
- **Cite Eino and Crush as references**, not sources. Where the pattern is borrowed from Eino's vocabulary (e.g. event types), name the Eino source; where it's borrowed from Crush's pattern (e.g. SQLite migration discipline), name the Crush source. Do not silently inherit.
- **Output:** patterns documented with examples and anti-patterns; `stepsCompleted: [1, 2, 3, 4, 5]`.

### Step 6 — Project Structure & Boundaries

- **Adapt.** Default Step 6 produces a generic full-stack tree. V0's structure is already constrained to the single Go module layout in PRD §10:
  ```text
  agent-vivy/
    cmd/vivy/
    internal/app/  config/  domain/  runtime/  provider/  tools/
    internal/session/  events/  httpapi/
    schemas/  fixtures/  ui/
  ```
- **Map every PRD FR to a specific directory:**
  - FR-1..2 (Session/Message) → `internal/session/`
  - FR-3 (Provider) → `internal/provider/`
  - FR-4..5 (Run lifecycle + events) → `internal/runtime/` + `internal/events/`
  - FR-6..7 (Tools + Approval) → `internal/tools/`
  - FR-8 (Persistence) → `internal/session/` (SQLite) + `internal/runtime/` (event store)
  - FR-9 (UI) → `ui/`
  - FR-10 (Config + Secrets) → `internal/config/`
  - FR-11 (Observability) → `internal/app/` (logger init) + `internal/events/` (event payloads)
- **Document the boundaries:**
  - API boundary: HTTP+SSE on `127.0.0.1` only (personal-gateway philosophy).
  - Process boundary: single binary, single SQLite file, single log file (no separate services in V0).
  - Provider boundary: `ProviderRef` is the only type any non-`internal/provider/` package may see.
  - Eino boundary: nothing outside `internal/runtime/` imports Eino directly.
- **Output:** structure tree + FR→directory mapping + boundary definitions; `stepsCompleted: [1, 2, 3, 4, 5, 6]`.

### Step 7 — Architecture Validation & Completion

- **Adapt.** Default validation asks "are all decisions coherent / all FRs covered / agents ready to implement". For V0 add:
  - **Philosophy compliance check:** every ADR must trace back to at least one PRD Keep/Adapt/Defer/Drop decision or one of the four philosophical anchors.
  - **Anti-clone check:** no ADR may reference Diva's `agent-diva-*` crate, Tauri command, or `agent-diva-deep-governance` runtime as a source. Eino and Crush are allowed sources; Diva's `providers.yaml` schema is an allowed source per D-022.
  - **V0 scope check:** every FR in scope has an architectural support; every "Out of Scope" PRD item is explicitly absent from the architecture (do not silently include).
- **Run the 16-item completion checklist.** Mark `[x]` only on verified items. Any unchecked item is reflected in Gap Analysis and reflected in the Overall Status (`READY FOR IMPLEMENTATION` only when all 16 are `[x]`).
- **Output:** validation results + checklist + readiness status; `stepsCompleted: [1, 2, 3, 4, 5, 6, 7]`.

### Step 8 — Architecture Completion & Handoff

- **Follow as-is.** Update frontmatter, present completion summary, recommend next artifact (`bmad-check-implementation-readiness`).
- **Adaptation:** before completion, confirm that the four philosophical anchors are quoted verbatim in at least one ADR's preamble. This is the audit trail for future sessions.
- **Output:** completed architecture with `stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]`, `status: 'complete'`.

## 4. Where this SOP explicitly diverges from BMad defaults

| BMad default | V0 adaptation | Why |
|---|---|---|
| Step 1 expects greenfield or extension | We are a new product line with constrained stack | Philosophy + PRD D-006 lock the stack before Step 1 |
| Step 3 expects starter evaluation | Documented skip with rationale | Stack locked; starter would add unwanted defaults |
| Step 4 expects broad collaborative decisions | Delta-focused, citing D-001..D-025 as "Already Decided" | PRD is the source of truth; architecture does not re-litigate philosophy |
| Step 5 expects generic patterns | Locked vocabulary anchored to FRs and philosophy | V0 must produce consistent code, not just a coherent design |
| Step 6 expects generic full-stack tree | Specific mapping of every FR to a directory | V0 needs to be implementable, not illustrative |
| Step 7 expects default coherence checks | Adds philosophy-compliance and anti-clone checks | V0's distinguishing constraint is the philosophy boundary |
| Step 8 expects generic completion | Requires anchor quote-tracing in at least one ADR | Audit trail for future sessions |

## 5. Run-time rules for whoever runs this workflow

1. **Read the PRD before Step 1.** Specifically: §5.0 (philosophy), §13 (decision log), §16 (revision notes).
2. **Read the BMad skill steps in full before executing each.** The skill has CRITICAL rules scattered throughout; partial reads produce incomplete decisions (Pitfall P1 / P5).
3. **Never skip A/P/C user choice.** The workflow is collaborative by design. Even if the user has given general direction ("you handle it"), each A/P/C pause must surface — the user has explicitly authorized autonomous execution, not silent skipping.
4. **Apply the anti-clone check at every step.** If a proposed design cites `agent-diva-*`, `agent-diva-deep-governance`, or Diva's Tauri command set, that's a violation of D-005 / D-014..D-021 — flag and revise.
5. **Cite provenance for every borrowed pattern.** Eino or Crush pattern? Cite the source file. Diva schema field? Cite D-022 and the original `agent-diva-providers/src/providers.yaml` line. Unattributed borrowing is forbidden.
6. **Frontmatter is the source of truth for workflow state.** Update `stepsCompleted` only after the corresponding step's content has been written and the user has selected C.
7. **P9 verification before any "0 real" / "does not exist" / "no LICENSE" claim.** This SOP already states "no LICENSE in claude-code". That statement was verified by `find`. Do not assert absence without verification.

## 6. Pre-run checklist

Before starting Step 1, confirm with the user:

- [ ] PRD v0.4 is the authoritative PRD (no v0.5 pending).
- [ ] Philosophy anchors (PRD §5.0) are unchanged.
- [ ] Decision log D-001..D-025 is unchanged.
- [ ] Capability Inventory is **not required** for V0 architecture (deferred to a separate document per PRD OQ-1).
- [ ] The user is ready to participate in A/P/C pauses, or has explicitly delegated them.

If any item is not checked, stop and re-converge before running the workflow.

## 7. Post-run deliverables

After Step 8, the architecture workflow produces:

- `_bmad-output/planning-artifacts/architecture.md` (final).
- A list of open architectural questions (separate from PRD OQ-1..OQ-9).
- A list of new decisions added by the workflow (these will need to be appended to PRD §13 if they are user-direction, or to the architecture document if they are implementation-only).
- A recommended next artifact (`bmad-check-implementation-readiness` or `bmad-create-epics-and-stories`).

## 8. What this SOP is NOT

- It is not a substitute for the BMad skill. The skill remains the source of truth for step structure, A/P/C protocol, and frontmatter schema.
- It is not the architecture itself. It is the runbook for producing the architecture.
- It does not pre-decide any architectural question. Each A/P/C pause must surface.

## 9. Revision Notes

- v0.1 (2026-08-06): Initial draft. Records how AGENT-VIVY V0 will run the BMad 8-step architecture workflow, given that V0 is neither greenfield nor an existing-workspace extension. Pre-flight checks, per-step adaptations, anti-clone guard, and P9 verification rules included.
