# F1 — Thinking levels (7-tier) + per-model policy

**Goal:** `off|minimal|low|medium|high|xhigh|max` thinking levels with per-model clamp, per-model default, and per-level sampling params; Anthropic/OpenAI adapters only.
**Epic:** F. **Requirements:** RQ-MDL.
**Spec:** [VCP-D1](../../specs/2026-10-06-vivy-code-parity-design.md) §5.7. **Baseline:** `f34f3ce`.

## Scope

**Files:** `internal/provider` (profile schema + adapter mapping — eino Claude `thinking.budget_tokens`, eino OpenAI `reasoning_effort`; check pinned eino-ext surfaces first), `internal/config` (`thinking` block per profile + global default), `internal/runtime`/`internal/modelhost` (effective level resolution), `internal/rpc` (`model/thinking` set/get), `sdk/tui/command` (`/thinking <level>` extends `auto|on|off`), `ui` settings card.

## Tasks

- [ ] Eino check: cite exact pinned eino-ext fields used for Claude budget and OpenAI effort before writing adapters.
- [ ] Ordered levels const; `auto`/`on`/`off` stay aliases (`on`→per-model default, `off`→off).
- [ ] Provider-profile `thinking: {supported_levels: [...], default: <level>, clamp: <max>}`; resolution = requested → clamp → default; sampling map `sampling_params_by_thinking_level` merged into request params (temperature/top_p etc.).
- [ ] RPC `model/thinking {session_id?, level}` (session-scoped override + persisted default) + `model/thinking/levels` list from active profile.
- [ ] TUI `/thinking <level>` + footer shows effective level; GUI settings select lists `model/thinking/levels`.
- [ ] Tests: clamp to supported max; per-model default; sampling params reach request body (adapter unit test capturing raw body, like `claude_test.go`'s cache_control check); alias mapping.
- [ ] `go test ./internal/provider ./internal/modelhost ./internal/rpc -run 'Thinking'`; `just ci`.
- [ ] Commit `feat(model): seven-level thinking with per-model policy`.

## Boundary

Anthropic/OpenAI only (O2). No virtual-model routing. Persisted default lands in settings overlay, not per-session JSONL.

## Acceptance

`/thinking xhigh` → adapter emits the mapped budget/effort; unsupported level clamps visibly; persisted default survives restart.
