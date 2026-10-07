# A3 — vivy-code `--mode rpc` (subprocess JSONL protocol)

**Commit:** `feat(vivy-code): rpc subprocess mode`
**Depends on:** A1 (flags/face.Options), A2 (facerun + JSONLSink projection)
**Spec:** `docs/superpowers/specs/2026-10-06-vivy-code-parity-design.md` §5.3

## What landed

`vivy-code --mode rpc` is now a live stdin/stdout JSONL command loop — pi's
headless embedding protocol:

- stdin: one JSON object per line `{id, type, ...params}`.
- stdout: `{id, type:"response", command, success, data|error}` plus
  un-correlated run events (same record names as `--mode json` via
  `facerun.JSONLSink`: session → agent_start → message_* → turn_end →
  agent_end → agent_settled).
- stdout is reserved for protocol; nothing else writes there.

### §5.3 command map — implemented now

| pi command | backend |
|---|---|
| `prompt`, `follow_up` | `turn/start`; while a run is active, follow-ups queue face-locally and drain into the next turn (kernel truth lands with B1) |
| `abort`, `interrupt` | `run/cancel` / `turn/interrupt` |
| `clear_queue` | drains the face-local queue; returns `{steering, followUp}` |
| `new_session` | `session/create`, adopts the new id |
| `switch_session` | `session/get` (exists-check) + adopt |
| `set_session_name` | `session/rename` |
| `get_state` | `{session_id, isStreaming, isCompacting, pendingMessageCount, steeringMode, followUpMode, autoCompactionEnabled}` |
| `get_messages`, `get_last_assistant_text`, `get_fork_messages` | `session/messages` (+ shaping) |
| `get_session_stats` | `stats/tokens` |
| `get_available_models` | `settings/providers` |
| `set_model` | `settings/model/select` |
| `compact` | `context/compact` (`customInstructions` already passed through; kernel honors it when D1 lands) |
| `fork`, `clone` | `session/fork` (clone forks at the last message id) |
| `get_commands` | static descriptor with per-command `available` flag |

### Deferred — answer `success:false` with a reason (not silent)

`steer` + `streamingBehavior:"steer"`, `set_steering_mode`,
`set_follow_up_mode` → B1. `set_thinking_level`,
`cycle_thinking_level`, `get_available_thinking_levels`, `cycle_model` → F1/F3.
`set_auto_compaction`, `set_auto_retry`, `abort_retry` → settings/retry surface.
`export_html`, `get_tree`, `get_entries` → C1.
`bash`, `abort_bash` → refused by design: exec must go through the governed
ToolHost, never a raw shell. `extension_ui_response` → no-op ack (no
extension-UI surface by spec O1). Unknown types → `success:false`.

## Plumbing

- `faceport.Options` gained `In io.Reader`; `cmd/vivy-code` wires `os.Stdin`.
  `--mode rpc` is the only consumer; all other modes ignore it.
- Session resolution mirrors facerun: `--session-id` > `-c/--continue`
  (newest) > `session/create`.
- `model.delta` payload (`{delta}`) projects to `message_update` records;
  terminal journal events (run.completed/failed/cancelled) settle through the
  same turn_end→agent_end→agent_settled tail as `--mode json`.

## Deviations

- `prompt` returns `{disposition:"started"|"queued", run_id}` — a VIVY
  addition so embedders can correlate events to the run; pi returns nothing.
- `get_commands` rows carry `available` so embedders can feature-detect
  against this build instead of trying and failing.
- `switch_session` accepts pi's `sessionPath` field (VIVY session ids double
  as paths) or `session_id`.
