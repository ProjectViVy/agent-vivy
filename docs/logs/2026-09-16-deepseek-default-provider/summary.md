# DeepSeek as the default provider

Date: 2026-09-16 · Scope: Vivy kernel + browser UI + product docs.

## What changed

DeepSeek is now the default and the single authoritative test provider for
agent-vivy:

| Field | Value |
|---|---|
| `providers.active` | `deepseek` |
| Credential env | `DEEPSEEK_API_KEY` |
| Default model | `deepseek-flash` |
| Base URL | `https://api.deepseek.com` — no `/v1` |
| Outbound endpoint | `https://api.deepseek.com/chat/completions` |

`openai` and `anthropic` remain available as optional bundles; neither was
removed.

### Kernel

- `fixtures/provider/deepseek.yaml` — new pre-baked bundle (six models:
  `deepseek-flash`, `deepseek-v4-pro`, `deepseek-v4-flash`, `deepseek-chat`,
  `deepseek-coder`, `deepseek-reasoner`), re-derived under D-025 with a
  `provenance` record pointing at the vendored Diva registry entry.
- `internal/provider/openai.go` — six DeepSeek model-metadata rows plus a
  `supportsThinking` flag surfaced through `ModelInfo`.
- `internal/provider/resolving.go` — `thinkingOptions` gained a DeepSeek
  branch. With `thinking=enabled` the outbound body carries
  `{"thinking":{"type":"enabled"},"reasoning_effort":"high"}`; off sends
  `{"type":"disabled"}`. Gated on the bundle's `SupportsThinking` metadata,
  and never injected for a non-`deepseek` bundle.
- `internal/config/config.go` — `Providers.DeepSeek` slot, `Default()` now
  `Active: "deepseek"`, validation set `{deepseek, openai, anthropic}`.
- `internal/app/app.go`, `internal/app/model.go`,
  `internal/app/settings/settings.go` (`ProviderDeepSeek`),
  `internal/rpc/control.go` (model refresh gated to `{openai, deepseek}`),
  `internal/eval/isolator.go`, `internal/modules/defaults/{providers,catalog}.go`.
- `internal/generated/assembly/zz_default.go` regenerated via
  `go generate ./internal/generated/assembly/`; the three profiles are
  `["deepseek", "openai", "anthropic"]`.
- `sdk/ui/src/module.ts` — the published UI Face contract widened so
  `FaceProviderEntry` / `FaceProviderEntryInput` accept `"deepseek"` and
  `FaceProviderRefreshInput.bundle` accepts `"openai" | "deepseek"`, matching
  the backend.

### Browser UI

- `ui/agent-diva-source/agent-diva-providers/src/providers.yaml` — the
  vendored DeepSeek entry: `default_model: deepseek-flash`, `deepseek-flash`
  first in `models`, base `https://api.deepseek.com`.
- `ui/scripts/gen-provider-catalog.py` — new mapping rule: the Diva entry
  `deepseek` becomes runtime bundle `deepseek` with `baseUrl: ''` (empty means
  "use the runtime bundle's built-in address"); `ProviderRuntimeBundle` gained
  `'deepseek'`.
- `ui/src/components/settings/provider-catalog.ts` — regenerated, never
  hand-edited.
- `ui/src/components/layout/WelcomeWizard.tsx` — `SUGGESTED_DEFAULTS` is now
  `{ provider: 'deepseek', model: 'deepseek-flash', baseUrl: '' }`.
- `ui/src/lib/api.ts`, `custom-providers.ts`, `saved-models.ts`,
  `ModelSettingsCard.tsx`, `MaskAndModelSwitcher.tsx` — bundle unions,
  editor defaults, and the native-provider refresh path accept `deepseek`.
- i18n `settingsModel.bundleDeepseek` added to both `en.ts` and `zh.ts`.

### Config, scripts, docs

- `config.example.yaml` — `active: deepseek`, a `deepseek` block, and the
  `/v1`-less base-URL note; `small_model` example now `deepseek-v4-flash`.
- `docker-compose.yml`, `docker-compose.postgres.yml` — pass
  `DEEPSEEK_API_KEY`.
- `dev.ps1` — the preflight "no provider API key" check now also looks at
  `DEEPSEEK_API_KEY`.
- `README.md`, `docs/dev/real-provider-smoke.md` (rewritten as a DeepSeek
  runbook with the original M4 report preserved as archived history),
  `docs/research/prd-agent-vivy-v0.md` (v0.6 revision note superseding the
  provider counts in D-018/D-022/D-023), `docs/IMPLEMENTATION-PLAN.md` §4.2,
  `schemas/providers.bundle.schema.json`,
  `schemas/events/payloads/run.started.json`.
- `docs/TODO.md` §0.1 — two new entries: `DEEPSEEK-REASONING-CONTENT` (the
  known `reasoning_content` gap) and `PROVIDER-PROFILE-DIGEST-PIN` (the
  two-place conformance digest pin that silently breaks on any `internal/`
  change).

### Tests

- New `internal/provider/deepseek_test.go`, anchored on
  `TestDeepSeekThinkingRequest`, which asserts the outbound request body
  (`model: deepseek-flash`, `reasoning_effort: high`, and the correct
  `thinking.type` per mode) — offline, no network.
- Every composition-level test that previously stubbed an OpenAI-shaped
  request/response now uses a DeepSeek-shaped stub, plus the app/rpc/runtime/
  eval/modules/config test sweep.
- `internal/provider/claude_test.go` and `resolving_thinking_test.go` were
  deliberately left untouched: they cover the Anthropic adapter protocol and
  are the retained non-DeepSeek coverage.
- Generic provider-profile fixtures (`internal/modules/{credential,model}`,
  `sdk/port/providerprofile`, and the `bundle: "openai"` + `base_url` custom
  gateway cases) keep their OpenAI shape on purpose, as the custom-gateway
  path coverage.
- `sdk/internal/conformance/reproduction_test.go` and
  `sdk/internal/assembly/conformance_results.json` re-pinned the `internal`
  source digest from `838dda65…` to `2a7e6402…`.

## Explicitly not done

- `internal/workflow/` and `internal/domain/workflow_test_support.go` — the
  untracked WF-1 lane WIP. Not touched. Because that package does not compile,
  the `test` and `vet` recipes in `justfile` now enumerate packages and skip
  exactly `agent-vivy/internal/workflow`; see `verification.md`.
- The upstream `eino-ext/components/model/deepseek` component stays rejected
  (no `reasoning_effort`, and it would pull `deepseek-go`/`ollama` into the
  dependency tree). `reasoning_content` is therefore still dropped — recorded
  as `DEEPSEEK-REASONING-CONTENT` in `docs/TODO.md` §0.1 rather than worked
  around with a Vivy-owned decoder.
- `studio/` submodule, historical `docs/logs`, and the third-party gateway
  catalog entries that legitimately use prefixed ids such as
  `deepseek/deepseek-chat`.
- No push.
