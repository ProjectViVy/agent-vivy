# VCP F1 — seven-level thinking with per-model policy

Story: `docs/superpowers/plans/vivy-code-parity/F1-thinking-levels.md`
Plan commit message: `feat(model): seven-level thinking with per-model policy`

## What landed

`off | minimal | low | medium | high | xhigh | max` thinking levels with
per-model clamp, per-model default, and per-level sampling params, on the
Anthropic and OpenAI adapters only. `auto` / `on` / `off` remain aliases:
`off` suppresses thinking, `on` resolves to the model's declared default
(or the highest declared level, or the medium-budget legacy default), and
`auto`/`""` behaves like `on`.

### Domain (`internal/domain`)

- `ThinkingMode` widened: three aliases plus six real levels in
  `ThinkingLevelOrder`; helpers `Valid()`, `IsLevel()`, `LevelIndex()`,
  and `ClampThinkingLevel(mode, supported)` (aliases pass through; an
  explicit level clamps to the declared maximum).
- `ModelInfo` gains `ThinkingLevels`, `DefaultThinking`, and
  `ThinkingSampling` (`{Temperature, TopP}` pointers — nil-preserving).

### Provider (`internal/provider`)

- Vendor model metadata gains `thinking_levels`, `default_thinking`, and
  `thinking_sampling` — all strict-validated in `ParseVendors` and
  mirrored in `provider.schema.json`. `default_thinking` must appear in
  `thinking_levels`; sampling keys must be known level names.
- `ResolveThinkingLevel(info, mode)` is the single resolution rule:
  `off` → off; explicit level → clamped to declared levels;
  `on`/`auto`/`""` → declared default (clamped), else the highest
  declared level for `on`, else the alias preserved.
- `decideThinking` maps the resolved level onto adapter surfaces:
  - anthropic → `thinking.budget_tokens` per tier
    (minimal 1024, low 2048, medium 4096, high 8192, xhigh 16384,
    max 32768) plus `max_tokens = budget + claudeDefaultMaxTokens`;
    totals above the SDK's non-streaming ceiling (128000/6) also carry
    `WithRequestTimeout(30m)` — the SDK's documented bypass for the
    "streaming is required" client guard on non-streaming calls.
  - openai-completions → `reasoning_effort` verbatim (levels beyond the
    eino enum — `xhigh`, `max` — pass through as strings; the upstream
    answers the truth).
  - deepseek endpoints stay binary: enabled + `reasoning_effort:high`,
    disabled on `off`.
  - per-level `thinking_sampling` merges `temperature`/`top_p` into the
    request through the common options surface.
- `vendors.yaml`: real declarations for the Anthropic and OpenAI family
  (opus/sonnet `[minimal..max]` default medium, haiku `[minimal..high]`,
  o1 `[low..high]`, gpt-5/5.1/5-mini `[minimal..high]`, gpt-5-nano
  `[minimal,low]` default low, gpt-5-pro `[high,xhigh,max]` default
  high). No `thinking_sampling` overrides declared — reasoning-family
  model ids (o1/o3/o4/gpt-5) legitimately reject sampling params
  client-side.

### Persistence + RPC (`internal/app/settings`, `internal/rpc`)

- `settings.thinking` is the persisted default (global overlay — same
  seam as the model selection); `auto` clears the key.
- `model/thinking {level?}` — set persists after validation, always
  returns `{thinking, effective, supported, supports_thinking,
  default_thinking, read_only}`.
- `model/thinking/levels` — the live model's declared surface; a
  thinking-capable model with no declaration reports all seven.
- All three turn-admission paths (`turn/start`, human admission, queued
  admission) now merge a per-turn explicit `thinking` param over the
  persisted default via `thinkingFor`.

### TUI (`sdk/tui`)

- `/thinking` accepts all nine values; bare `/thinking` and `Ctrl+T`
  cycle `auto → minimal → low → medium → high → xhigh → max → off → auto`.
- The selection persists through the duck-typed `PersistThinkingMode`
  driver method (live driver calls `model/thinking`); the footer shows
  the effective level via `ThinkingEffective()`.
- Capability gating is unchanged: levels are rejected on models without
  thinking support; `auto`/`off` always pass.
- `thinkingModes`/validation live as literals in the SDK — the SDK does
  not import `internal/domain`.

### GUI (`ui`)

- `ThinkingMode` widened to the nine-value union; `model/thinking` and
  `model/thinking/levels` registered in `api.ts` with typed calls.
- `ChatInput` thinking dropdown lists all nine values (level labels are
  the raw names; aliases stay i18n).
- New `ThinkingSettingsCard` on Settings → General reads `getThinking` +
  `thinkingLevels` and writes `setThinking`; en/zh i18n entries.
- `@vivy/ui-sdk`: `FaceThinkingMode` widened, `FaceThinkingReport` /
  `FaceThinkingLevelsView` added, `getThinking`/`setThinking`/
  `thinkingLevels` declared on `FaceClientAPI` — the face-compat type
  test stays green.

## Design notes

- Resolution is data-driven: `xhigh`/`max` work because
  `reasoning_effort` is a string on the wire, not because the eino enum
  gained members.
- The Anthropic SDK refuses non-streaming calls whose `max_tokens` would
  cross 128000/6 tokens; levels xhigh/max exceed it, so those calls carry
  a 30-minute request timeout instead of failing before launch.
- `on` keeps its legacy meaning ("the model's preferred thinking")
  rather than pinning a level — the medium budget is preserved as the
  fallback for thinking-capable models that declare no policy.
