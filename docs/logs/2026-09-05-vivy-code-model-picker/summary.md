# VIVY CODE authoritative global model picker

## Delivered

- Added a Crush-style global model picker shared by the built-in and packed fullscreen faces. `Ctrl+L` opens it directly and `/model [filter]` opens the same surface with an initial fuzzy filter.
- Added the narrow `settings/model/select` capability and RPC. It accepts only the true config default, a pre-baked bundle model, or a model belonging to the exact saved provider registry endpoint; invented models and mismatched gateway URLs fail closed.
- Exposed pre-baked bundle models separately from editable provider entries. The composition root now passes the real config baseline rather than mislabeling the active settings overlay as the config default.
- Added a runtime run-start fence: model changes are rejected while main, approval-suspended, shell-suspended, or child-authority work exists. The legacy `settings/update` selection path uses the same fence, and both selection routes are serialized.
- Selection is server-confirmed, not optimistic. The modal remains until completion, stale request epochs are ignored, failures preserve the candidate/current state, and a successful result refreshes the active-session sidebar.
- Candidate labels are ANSI/control/bidi cleaned and bounded. Endpoint URLs and credential state are selection identity only and are never rendered.
- Fixed terminal-failure cleanup so orphaned snapshots/ledgers/run ownership cannot leave model selection permanently busy.

## Explicitly not delivered

- No fake large/small tier or reasoning-effort picker was added; the current runtime exposes one route and a separate boolean thinking capability.
- The picker is global, not session-local. Its UI explicitly says the selection applies to the next idle turn.
- A local queued turn in another client is not visible to the server. It uses the globally active route when it actually starts; an optional enqueue-time model-affinity contract is tracked in `docs/TODO.md`.
- Split diff presentation and richer token/cost controls remain in `TUI-PARITY-2`.
