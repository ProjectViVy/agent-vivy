# F2 — Prompt cache warming

**Goal:** ModelHost-side cache-warming scheduler, `off|streaming|idle` modes (default `streaming`), per-model cache-lifetime gate.
**Epic:** F. **Requirements:** RQ-MDL.
**Spec:** [VCP-D1](../../specs/2026-10-06-vivy-code-parity-design.md) §5.7. **Baseline:** `f34f3ce`.

## Scope

**Files:** `internal/modelhost` (scheduler), `internal/provider` (`cache_lifetime` metadata on profile; warm-request builder — Anthropic adapter already emits `AutoCacheControl` ephemeral breakpoint), `internal/config` (`cache_warming: off|streaming|idle`, `cache_warming_min_savings` floor), runtime usage accounting (cache tokens already on the wire).

## Tasks

- [ ] Provider profile gains `cache: {lifetime_seconds, supports_warming}`; only Anthropic-family profiles set it (OpenAI has no explicit cache API — mark profiles accordingly; no fake warming).
- [ ] Scheduler: after each settled model stream (`streaming` mode) refresh the breakpoint with the current prompt prefix; on idle timer (`idle` mode) refresh before lifetime expiry.
- [ ] Warm call = minimal request carrying the prefix + cache_control; response discarded; usage folded into session token totals as `cacheWrite`; NEVER added to model context or Journal messages (diagnostic event `cache.warmed` only).
- [ ] Gate: skip when estimated avoided cache-miss cost < floor (config; pi's default ≈ $0.05 equivalent — keep a token-count proxy if pricing data absent).
- [ ] Cancellation: warm calls cancel with the run; never block turn admission; failures are silent diagnostics, never run errors.
- [ ] Tests: warm fires after settle in streaming mode with fake provider counting calls; idle timer path; usage accounting shows cacheWrite; disabled mode makes zero extra calls; warm never pollutes `ListMessages`.
- [ ] `go test ./internal/modelhost ./internal/provider -run 'Cache|Warm'`; `just ci`.
- [ ] Commit `feat(model): prompt cache warming scheduler`.

## Boundary

Anthropic only today; no warm for providers without `supports_warming`. No `cache_warming_decision` hook (that's the extension hook — out of scope O1).

## Acceptance

With a fake 5-min-lifetime provider: streaming mode keeps cache age < lifetime across a long run; usage totals grow by only warm-request tokens.
