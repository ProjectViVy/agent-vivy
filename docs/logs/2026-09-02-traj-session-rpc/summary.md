# UI-TRAJ / UI-TRAJECTORY-DEMO — real session-trajectory RPC + panel wiring

## Summary

Switch the "Console → Trajectory" panel from demo data to real kernel data, deliver the
kernel `trajectory/session` RPC capability (UI-TRAJ), and wire the panel to the real API
(UI-TRAJECTORY-DEMO).

### Kernel (`internal/`)

- `internal/runtime/trajectory.go` (new): `Service.SessionTrajectory` projects the most
  recent N runs of a session (default 20, maximum 50) from the journal (`run_events`) +
  message storage into a turn-level trajectory structure. Semantics:
  - one run == one user turn; each model call within a turn is a Step;
  - folded events: `run.started` (the first run produces the Session section's system row;
    later runs only update provider/model provenance), `model.request` (opens a request
    slot), `model.usage` (tokens), `provider.retry` (retry count), `model.completed` (closes
    the request slot + ASSISTANT row), `tool.requested/started/finished` (tool row, with
    input/result in details), `context.compacted` (Compaction row with a null turn), and
    `run.failed` (Run-group failure row);
  - user copy comes from Messages storage joined by RunID (not journal replay);
  - bounded projection: text/details are truncated at 8 KiB (UTF-8 safe), and dangling
    requests are closed as error.
  - request payloads contain only hashes and byte lengths (D-010); no transcript
    reconstruction beyond persisted content.
- `internal/rpc/control.go`: add the `trajectory/session` route and handler
  (`session_id` required, limit optional; unconfigured Service → MethodNotFound; a session
  with no run retains 404 semantics for ErrNotFound, while an empty projection returns
  normally).
- Tests: `internal/runtime/trajectory_test.go` (hand-built two-run projection shape
  assertions, limit capping, real Echo-run end-to-end projection), and
  `TestTrajectorySessionRoute` in `internal/rpc/control_test.go` (missing-parameter
  InvalidParams + seeded-run shape assertion).

### UI (`ui/src/`)

- `ui/src/lib/api.ts`: register `trajectory/session` in RPC_METHODS; add snake_case wire
  types (TrajectorySessionWire / TrajectoryRecordWire / TrajectoryRequestWire /
  TrajectoryTokensWire) and `fetchSessionTrajectory`.
- `ui/src/components/trajectory/trajectory-types.ts` (new): the single source of truth for
  panel presentation types (TrajectoryCellKind, TRAJECTORY_KIND_LABEL, TrajectoryRecord,
  TrajectoryRequest, etc.); request types add the real RPC's `messages` / `preambleBytes`.
- `ui/src/components/trajectory/trajectory-session.ts` (new): wire → presentation-layer
  mapping (snake_case → camelCase, fill missing usage with 0, mark the purpose of requests
  in a Compaction group).
- `ui/src/components/trajectory/TrajectoryPanel.tsx`: session selector (listSessions +
  Select, first session by default) + refresh button; loading skeleton / no-session empty
  state / error state (DemoLoadError) + all existing folding, search, selection, and detail
  interactions now operate on real data. Demo helpers (requestByNumber / recordForRequest)
  are replaced by local lookup on the real data.
- The remaining trajectory subcomponents (Toolbar/Timeline/Ledger/DetailPanel) only change
  the source of their type imports.
- `ui/src/components/trajectory/trajectory-demo-data.ts`: reduced to a test fixture solely
  for `trajectory-utils.test.ts` (no production-chain reference; excluded from the bundle).
- i18n: change `dashboard.trajectoryDesc` to describe real data; add
  `pickSession` / `sessionEmpty` / `refresh` to the trajectory section (en + zh in sync).

## Explicitly not done

- The detail panel does not display `messages` / `preambleBytes` (types are in place; add UI
  presentation when needed).
- The trajectory projection does not include token cost (cost) — the journal has no price
  fact, so none is inferred.
- DSF `context` / `subtool` kinds remain only for rendering compatibility; the Vivy kernel
  does not produce these record types.
- Intermediate thinking content within a run is not in the projection (the journal does not
  persist plaintext thinking streams).
