# CMP-2 — Independent model for compaction summaries: `summary_model`

## Scope

- `internal/config`: `CompactionConfig.SummaryModel string \`yaml:"summary_model"\`` —
  an optional cheaper model ID on the same active provider, used instead of the
  main model to generate compaction summaries; empty = continue using the main
  model. Normalize during Validate (TrimSpace; reject newlines/NUL, using the
  same structural validation as `runtime.small_model`; unknown IDs are not
  rejected at startup—call failure triggers failover).
- `internal/provider`: exports `NewOverrideModel(catalog, resolver, modelID)` — a
  D9 single-source helper that pins the auxiliary model ID (`modelOverrideSource`
  plus a resolving model, resolving the active provider's live spec at call
  time); reused by the `TitleCandidates` refactor.
- `internal/runtime`: `EngineConfig.SummaryModel` (the D-007 boundary: the app
  side passes the opaque alias `runtime.SummaryModel =
  model.BaseModel[*schema.Message]`, and the app does not import `eino`).
  `buildCompactionHandlers` wires in native Eino summarization failover: when an
  override is configured, `primary = override` and
  `Failover{MaxRetries:1, BackoffFunc:0, GetFailoverModel: main model}`;
  `GetFailoverModel` rebuilds the default input shape—after removing the original
  leading system message, it inserts the middleware's system/user summary
  instructions without duplicating the system message. Without an override,
  there is no failover (retrying the same model once is pointless, and behavior
  remains exactly as before).
- `internal/app`: both `buildEngineConfig` paths, at startup and during
  settings-save reload, pass `summaryModel` (the normalized config ID →
  `provider.NewOverrideModel`).
- `config.example.yaml`: comment example for `compaction.summary_model`.

## Semantics

| Configuration | Summary generation | Failure behavior |
|---|---|---|
| `summary_model` empty | Main model (previous behavior) | No failover; same as before |
| `summary_model` set | Override model once | Failure → main-model fallback exactly once (no backoff); a second failure reports an error from the run |

## Not done

- The settings.yaml overlay / UI panel does not add a `summary_model` field (this
  slice only requires the configuration surface; the overlay through settings
  RPC + UI needs a separate slice).
- The reduction layer is unaffected (deterministic clearing does not use a
  model).
- Manual `CompactSession` summary-model selection is unchanged (it follows the
  existing path and is outside this slice).
