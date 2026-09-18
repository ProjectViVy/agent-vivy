# 2026-09-18 · Provider registry single-source plan

## What changed

Authored a complete, decision-complete action plan for giving Vivy a single write
point for provider configuration data and for correcting the sealed unit from
"vendor" to "protocol adapter". **Documentation only — no code, no data file, no
configuration was changed in this delivery.**

Created:

- `docs/plans/provider-registry/README.md` — program plan, 19-decision ledger,
  status board (all five phases `UNSCHEDULED`), critical path, expected net
  effect, and the explicit relationship to PLG-P5.
- `docs/plans/provider-registry/DESIGN.md` — the duplication inventory with
  evidence, the four-layer target model, the full data schema with a named
  consumer per field, the deleted-field list, endpoint identity and data flow,
  the default chain, unknown-metadata semantics, availability projection, worked
  examples, and rejected alternatives.
- `docs/plans/provider-registry/EINO-CAPABILITY.md` — the mandatory Eino
  capability check for the three adapters, citing pinned module-cache paths and
  line numbers, the `DEFERRED-INDEFINITE` verdict for `openai-responses`, the
  owner's committed `AgenticModel` migration path with its blast radius, and the
  out-of-scope `reasoning_content` gap.
- `docs/plans/provider-registry/MIGRATION.md` — per-file change/delete inventory
  across assembly, `internal/provider`, config, app wiring, settings, eval, RPC,
  UI, Docker, and docs; the three-row `settings.yaml` migration map; the
  conformance-digest step; the WF-1 untracked-file warning; and per-phase
  rollback.
- `docs/plans/provider-registry/PROV-P1-data-source-and-embed.md` through
  `PROV-P5-conformance-and-closeout.md` — five phase plans in the repository's
  `Goal / Files / Interfaces / Steps / Exit / Commit` form, each with a
  failure-first test list.
- `docs/logs/2026-09-18-provider-registry-plan/{summary,verification,acceptance}.md`
  — this record.

Modified:

- `docs/TODO.md` §0.1 — added `PROVIDER-REGISTRY-REDESIGN` (the unscheduled
  program pointer), `PROVIDER-AGENTIC-MIGRATION` (the deferral's lift
  conditions), and `PROVIDER-DATA-CONFIG-EDIT` (the `env_key` trust boundary).
  Updated the board's two date markers to 2026-09-18.

## Decisions recorded

The 19 decisions are listed in `docs/plans/provider-registry/README.md`. The
three that came from the owner directly in the design session:

1. Three adapters named after the real wire APIs — `openai-completions`,
   `openai-responses`, `anthropic-messages` — rather than two adapters plus a
   model-level dialect dimension.
2. `openai-responses` ships as `DEFERRED-INDEFINITE` **with an explicit
   commitment to migrate onto `AgenticModel`** as its only lift path.
3. `internal/provider/data/` is the single directory for all provider
   configuration data; path `//go:embed`, no runtime disk read.

## Scope

- **In scope:** the plan, its evidence, the backlog rows, and this log.
- **Out of scope (explicitly not done):** any Go, TypeScript, YAML, JSON Schema,
  Docker, or configuration change; scheduling any phase; pushing the branch.

## Deliberate deviations from the approved plan

1. **Worktree location.** The approved plan named
   `git worktree add ../agent-vivy-provider-sot`. It was created at
   `.worktrees/provider-sot` instead, on the same branch
   (`docs/provider-registry-plan`). Reason: `.worktrees/` is already the
   established convention for three existing lanes
   (`plugin-p8-gate-a`, `plugin-v1-p9-conformance`, `workflow-wf1`) and is
   gitignored (`.gitignore:54`), and keeping the checkout inside the session
   workspace keeps the deliverables addressable. The isolation requirement is
   satisfied either way: own worktree, own branch, root tree untouched.
2. **One extra backlog row.** The plan specified two new §0.1 rows. Three were
   added: `PROVIDER-REGISTRY-REDESIGN` was added so the unscheduled five-phase
   program is discoverable from the living board rather than only from the plan
   directory. The two planned rows are present unchanged in intent.

## Records

- Plan: `docs/plans/provider-registry/`
- Backlog: `docs/TODO.md` §0.1
- Branch: `docs/provider-registry-plan` (not pushed)
