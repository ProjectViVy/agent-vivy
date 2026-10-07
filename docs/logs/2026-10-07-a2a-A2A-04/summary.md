# A2A-04 Summary — bounded TaskHost reads, cancellation, replay/live projection

## .1 refactor(journal): share bounded committed text projection — a256e1c7
- `internal/journalview/text.go`: shared assistant-text reducer
  (model.request reset → committed model.delta accumulation → pre-tool
  flush at tool.requested → model.completed v2 byte_len + sha256
  validation; hash never decoded as text). `TextMessageID` keeps the
  exact `msgp_<run>_<seq>_<slot>` identity.
- `internal/runtime/message_projector.go` now consumes the shared
  reducer; native transcript semantics unchanged (v1 reconcile skip
  preserved via `journalview.ErrUnsupportedCompletedVersion`).
- `storage.JournalPageReader.ReadJournalPage` on both backends: ≤256
  events and ≤1 MiB per page under one consistent read snapshot with a
  fixed committed `ThroughSeq` watermark; `ErrJournalPageLimit` for
  bound violations and oversized first events; empty+HasMore forbidden.

## .2 feat(channelhost): project governed native task operations — ace3ee98
- `internal/channelhost/tasks.go` + `task_projection.go`: channel
  .TaskHost Submit/Get/List/Cancel + TaskServiceInfoHost.
- Private `TaskPrincipal` ctx binding (Module/Provider/Instance/
  Principal/Revision); every op revalidates (app revision + per-env
  allow_from/instance pin in `envFor`).
- Submit covers new admission and ordinary pending-input answers;
  Replayed surfaces dedup.
- Projection: committed-journal-only §7 state map, answered/settled
  precedence, terminal-wins; history = user message + safe assistant
  text + gated safe prompts; approvals never forward prompt data;
  failures sanitized.
- ListTasks: scope scan under an explicit event budget, committed-status
  -time desc + RunID sort, exact totals only on complete in-budget scan;
  HMAC-signed ≤1 KiB tokens, 5-minute expiry, bound to scope+filters,
  reauthorized each page.
- `Deps.Tasks` mounts the contract on the per-channel env only when the
  pack is complete (assertion fails closed otherwise); app wires Submit/
  Cancel/Subscribe/Authorize.

## .3 feat(channelhost): stream bounded journal task projections — 7b292733
- `task_stream.go`: subscribe-before-read; snapshot under fixed H first
  (fresh subs) or tail-after-cursor; incremental projector emits
  artifact (full-replacement) + status updates with {v,run,seq,ordinal}
  cursors ≤256B.
- Bus close/drop → re-register + journal catch-up; 1s fallback tick for
  missed publishes; only a committed terminal or delivered interrupted
  state ends the stream; INPUT_REQUIRED/AUTH_REQUIRED deliver then EOF.
- Cursor validation is synchronous at SubscribeTask.

Digest repins: 64558784 (.2), 0fad9562 (.3).
