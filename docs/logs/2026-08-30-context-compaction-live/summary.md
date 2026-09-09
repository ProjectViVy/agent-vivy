# Live context compaction (2026-08-30)

## Changes

Turn context compaction from a "demo/dead config" into a real closed loop: the chat-box
context ring shows actual server usage; Eino's native official middleware compacts an
over-budget session automatically; the Settings "Context compaction" card persists and
drives the engine; and manual compaction persists a session-level summary that is
folded into subsequent feed.

### Research conclusion

Adopt the conclusion from `docs/research/AGENT-LOOP-PORT-COMPARISON.md` §4.3-E2 / P3:
Eino v0.9.13's official `reduction` + `summarization` middleware (full implementation
plus Vivy bridge); keep the custom `internal/runtime/compaction` package unchanged
(its meter semantics are reused for token estimation).

### Core (runtime / engine)

- New `internal/runtime/compaction_middleware.go`:
  - The `ContextMonitor` micro-middleware records the pre-compaction token count in
    `BeforeModelRewriteState`.
  - `buildCompactionHandlers` assembles Eino's official `reduction` (`SkipTruncation=true`,
    `Backend=nil` for in-memory placeholder storage only, `ClearRetentionSuffixLimit=keep_recent`)
    and `summarization` (`Model=primary model`, `Trigger=trigger_tokens`,
    `EmitInternalEvents=true`, `Retry=0`, `Callback` emits events); order is reduction
    first, then summarization (pinned by tests).
- `engine.go`: `EngineConfig.Compaction *CompactionPolicy`; when enabled, append the
  three middleware components to the handler chain.
- `mapper.go`: recognize the summarization middleware's `generate_summary`
  CustomizedAction and map events carrying Usage to `model.usage` (summary calls are
  visible in accounting).
- `hooks.go` / `governanceSink`: add compaction fields to `GovernanceEvent`; persist and
  publish the `context.compacted` branch; summarization mode additionally calls
  `ReserveModelCall` (`MaxModelCalls` cannot be bypassed, P3②).
- New `internal/runtime/compaction_policy.go`: `CompactionPolicy` + `TriggerTokens` =
  `min(max_tokens×threshold%, feed byte-budget tokens)`, ensuring the trigger can be
  reached within the byte limit.
- New `internal/runtime/compaction_service.go`:
  - `ContextStatus(sessionID)`: actual session pressure (feed bytes/tokens vs model
    window vs byte limit, would_compact, last_compaction).
  - `CompactSession(sessionID)`: manual compaction — the primary model generates a
    summary → persist to `session_compactions` → write `context.compacted` (mode=session)
    to the Journal.
  - `ScheduleEngineReload`: rebuild the engine immediately when idle after settings are
    saved, defer in-flight changes until the next idle run start, and capture the engine
    reference at the top of each run.
- `runMessages` / `foldSessionHistory`: while assembling the feed, replace old rows
  covered by a session summary with user messages prefixed `[Session compaction summary]`,
  while preserving the tail.

### Contract

- `EventContextCompacted EventType = "context.compacted"` (non-terminal), vocabulary 35
  (`domain_test` updated).
- `payloadContextCompacted` (numbers + mode only, D-010).
- `schemas/events/run-event.schema.json` enum + `payloads/context.compacted.json`.

### Configuration and settings

- `config.example.yaml` `runtime.compaction`: `enabled` (default true) /
  `max_tokens` (0 = model window, otherwise 128000) / `trigger_percent` (80) /
  `keep_recent` (12); `config.go` defaults + validation (trigger 1..100,
  keep_recent≥1, max_tokens≥0).
- `settings.yaml` `compaction` overlay (nil = use config; an empty override becomes
  nil); `Validate` uses the same bounds as config; `applySettingsOverlay` merges it at
  startup.
- `settings/get` returns `compaction` (effective values + `config_*` fallbacks);
  `settings/update` read-modify-writes the compaction section; after saving,
  `OnSettingsChanged` compares the effective policy and calls `ScheduleEngineReload` on
  change.

### Storage (migration 015)

- `storage.CompactionStore`: `SaveSessionCompaction` / `LatestSessionCompaction`.
- sqlite `migration015` creates `session_compactions` (PK session_id+created_at+run_id);
  postgres schema v14 has the same table; both sides implement `compaction.go`.

### RPC

- `session/context` → `ContextStatus`.
- `context/compact` → `CompactSession` (409 while running; return a `skipped` result
  when there is nothing to compact or the budget is not exceeded).
- Register the new methods in capabilities and `RPC_METHODS`.

### UI

- `api.ts`: `SessionContext` / `CompactResult` / `CompactionSettingsView` types +
  `getSessionContext` / `compactSession`; `settingsUpdateFrom` carries the compaction
  section (so full-document replacement cannot clear it).
- `store.ts`: `sessionContext` state + `loadSessionContext`; refresh after selectSession,
  terminal run states, and `context.compacted` events; `compactSession` action.
- `ChatView` / `ChatInput`: the ring now reads real `session/context` (tokens vs model
  window + byte details + "compression threshold reached / compressed" status); remove
  the client-side hard-coded 256KB estimate.
- New `CompactionSettingsCard.tsx` (Settings → General): enabled toggle, three inputs
  (config-fallback placeholders), actual usage bar, "Save configuration", "Compact
  now", and "Refresh usage"; the demo "Context compaction" card in
  `DivaSettingsPreview` becomes an explanatory placeholder.
- i18n: update `chatInput.context*` to token wording and `settings.compaction.*` to
  real copy (zh/en).

## Explicitly not done (this iteration)

- File-level recovery for `reduction` storage in Backend/offload (`Backend=nil` is only
  an in-memory placeholder) → `docs/TODO.md` CMP-1.
- Independent summary model `summary_model` → CMP-2.
- Retrieval UI for session summaries (the summary is already in the feed; there is no
  retrieval surface) → CMP-3 (G2 candidate).
- TurnLoop / P1 reward rounds / runtime `/compact` control channel (second batch);
  adding context fields to preflight (covered this time by `session/context`).

## Changed files

Backend: `internal/domain/event.go`, `internal/runtime/{engine,service,mapper,hooks,payloads,preflight}.go`, new `internal/runtime/compaction_{middleware,policy,service}_test*.go`, `internal/config/config.go`, `internal/app/settings/settings.go`, `internal/app/{app,compaction}.go`, `internal/storage/contracts.go`, `internal/storage/sqlite/{sqlite.go,compaction.go}`, `internal/storage/postgres/{schema.go,postgres.go,compaction.go}`, `internal/rpc/control.go`, `config.example.yaml`, `schemas/events/**`, and corresponding tests.

Frontend: `ui/src/components/settings/{CompactionSettingsCard.tsx (new),SettingsView.tsx,DivaSettingsPreview.tsx}`, `ui/src/components/chat/{ChatView,ChatInput}.tsx`, `ui/src/lib/{api,store}.ts`, and `ui/src/i18n/{zh,en}.ts`.
