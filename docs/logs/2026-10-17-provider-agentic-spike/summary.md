# Provider agentic spike (S0–S4) — run

## What changed

Ran the Route B spike planned in `docs/plans/provider-agentic/` (issue
ProjectViVy/agent-vivy#73), scheduled by the owner on 2026-10-17. Results
were written into each stage file's final "Result" section; S4 holds a
**proposed, unsigned** `GO-CONDITIONAL` decision record.

- **S0** — module-graph delta measured on two baselines (upstream `main`
  `e7dcf85`, eino v0.9.13; plan baseline `a5f1f90`, eino v0.9.20): +10 / −2 /
  ~5 modules on both. The second OpenAI client (`openai-go/v3` + `azcore`) is an
  **addition**; `go-openai` is bumped, not removed. Component probe +5.19 MB
  (+15.2 %). No pin conflict: the current code builds and passes
  provider/runtime tests at eino v0.9.21.
- **S1** — every cited Eino API confirmed; verdict `ADOPT-WITH-ADAPTER` for all
  three components (thin per-request option projections only). **No bridge or
  codec needed.** Added C12–C14 (caching, cache-write tokens, timeout). Eight
  Vivy-owned middlewares, not seven.
- **S2** — zero-network `httptest` harness, one typed ADK agent per protocol,
  `-race`. Text, tool calls, isolation and cancellation pass on all three.
  Findings: Responses streaming drops `encrypted_content`; Claude `TotalTokens`
  wrong under output-only `message_delta`; cut-off streams report success (also
  true on the current stack); completions drops deferred tools silently;
  completions user content becomes a parts array; the shipped Claude component
  has a data race. Reasoning is **already** visible on the current stack.
- **S3** — inventory re-cited at `main`: 119 lines / 23 files of
  `schema.Message` (plus 28 multimodal-part refs), consumers checked for every
  deletion, Shape A only, titler moves (D7), risks R14–R21 added, phases P0–P4.
- **S4** — `GO-CONDITIONAL` proposed behind B1–B7 (Eino core upgrade consent +
  pending-run policy, cost/benefit acceptance, Responses `store`, truncation,
  Claude total, Claude cache semantics, vendor content shape).

Backlog: `docs/DEFER.MD` `PROVIDER-AGENTIC-MIGRATION` status updated and
`DEEPSEEK-REASONING-CONTENT` annotated as likely stale; two new `docs/TODO.md`
§0.1 rows (`STREAM-TRUNCATION-SILENT-SUCCESS`, `EINOEXT-CLAUDE-STREAM-RACE`).

The branch also carries the two plan commits from upstream
`docs/provider-agentic-plan` (cherry-picked, docs only) on top of upstream
`main`; the 17 issue-58 commits under that branch were **not** brought in.

## Owner decisions (2026-10-18)

B1 consent half approved (Eino core upgrade OK); B2 accepted after the cost
explanation added to S0 §7.5. Remaining blockers: B1 pending-run policy, B3–B7.

## Not done

- No production code, `go.mod`/`go.sum`, `internal/` or UI change (D8).
- S4 is not signed; no blocker is declared closed.
- No live-provider call; B6/B7 need authorized credentials.
- Windows binary sizes not measured (linux sandbox).
- `DEEPSEEK-REASONING-CONTENT` not closed: component evidence only, no
  product-path check.
- The throwaway S0/S2 modules are not committed (S2 §2); see `notes.md`.
