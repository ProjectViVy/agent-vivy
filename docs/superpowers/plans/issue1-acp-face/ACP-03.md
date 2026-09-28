# ACP-03 Session and Committed Turn Adapter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Story / Epic:** ACP-03 / E1. **Goal:** Run ACP initialize, session creation and prompt turns against real Vivy sessions, emitting only ordered committed updates.

**Architecture:** A packable T2 Module implements `std/face@v1`. Its instance uses the pinned SDK stdio transport and `face.Host.Call/OnEvent` for the existing private Control RPC, with a connection-owned session/run table and bounded writer. The adapter never accesses Journal or Policy directly. **Tech Stack:** Go 1.26.4, pinned compatible ACP SDK from ACP-01, Vivy JSON-RPC Control, `github.com/eino-contrib/acp` if approved. **Spec:** [reviewed design](../../specs/2026-09-28-issue1-acp-face-design.md), baseline `3c4ed66`. **State / dependencies:** [index](index.md); requires accepted ACP-02 `face.Options.In`, startup, `faceinstance` and G0 contract; G1 scheduling required.

## Global Constraints

- Compiled ID `projectvivy/acp`, T2 source `repo:plugins/acp`, provider ID `projectvivy.acp`, `std/face@v1`, `core/face-host@v1`, requested/effective `rpc.client` only. No extra Host/Port or Eino type in Face Port.
- Stable v1 local NDJSON stdio, one connection per process, many connection-owned sessions within G0 bound, at most one active prompt per session. `FaceCode` is sent on `turn/start` as `face: code`.
- Use **real** Control `session/create` / `turn/start` / `run/subscribe`; reply only after terminal committed Journal event. `Host.OnEvent` must be registered before `turn/start`; subscribe from seq 0.
- All accepted fields, bounds and client-visible errors come from ACP-01's frozen contract; never forward private/raw payload or reasoning text. Do not claim `loadSession`, modes, terminal, client FS, image/audio, or URL elicitation.

## Review Focus

1. Same canonical directory reached via a symlinked parent is accepted but a different project root is rejected before `session/create`.
2. Session ID from another connection or a guessed ID cannot start/cancel a Run, even if Control could address it.
3. Notification that races with subscribe replay does not duplicate or reorder `(run_id, seq)` updates.
4. A failed `turn/start`, `run/subscribe`, replay gap, or `run/stream_error` fails the prompt and cancels any accepted Run, never returns `end_turn`.
5. Content with oversized URI, embedded context, unsupported MIME, or nonempty `mcpServers`/`additionalDirectories` is rejected before Run creation.

---

### Task 1: Module and connection handshake

**Files:** Create (proposed) `plugins/acp/{go.mod,go.sum,vivy-module.yaml,module.go,face.go,connection.go,connection_test.go}`; modify `go.work` and `go.work.sum` only if the repo's external-module build demands it. No Recipe pin yet: ACP-05 seals the final source hash.

**Interfaces:** Consume `face.FaceProvider`, `face.Host`, `face.Options.In/Out/Err`, ACP-01's accepted SDK and contract; produce `New() module.Module`, `NewProvider() face.FaceProvider`, provider `Definition{ID:"projectvivy.acp",Kind:"acp"}` and `Instance.Run(context.Context, face.Options) (face.Result,error)`. Connection state is private to `plugins/acp`; all outbound ACP frames share one bounded writer, log output uses `Err` or filtered file logger.

- [ ] Write failing tests for descriptor/provider identity, `initialize` version/capability negotiation, NDJSON stdout purity (including pre-handshake failure), bad version, optional method rejection, inbound-size and output backpressure bounds.
- [ ] Run `go test ./...` from `plugins/acp` and focused `go test ./sdk/port/face` from repo root; expect missing provider/connection before implementation.
- [ ] Implement the minimal Module lifecycle and SDK stdio transport with `Options.In/Out` and G0-approved logger. Make all reverse requests, notifications and responses share one bounded serializer. No listener or subprocess.
- [ ] Rerun tests; expect stable ACP v1 initialize output and fixed sanitized error categories; record dependency graph to ensure no undesired transport or credential logger.

### Task 2: Owned sessions and one active prompt per session

**Files:** Create (proposed) `plugins/acp/{session.go,session_test.go,prompt.go,prompt_test.go}`; extend `connection.go` only for dispatch and ownership map.

**Interfaces:** Session state maps returned Vivy `session_id` to `{activeRunID, subscriptionID, lastSeq, pendingReviewIDs}` under one mutex. `session/new` validates canonical `cwd` identity against `face.Options.ProjectRoot` supplied by the launcher, never against ACP-controlled input; calls Host `session/create` with `workspace_path: opts.ProjectRoot`. `session/prompt` validates and converts Text/ResourceLink into bounded user `text` and validated optional `context_paths`; calls `turn/start` with `session_id`, `text`, and `face: code`, stores `run_id`, subscribes `run/subscribe` `{run_id,after_seq:0}`, then awaits terminal.

- [ ] Write failing fake-Host tests for canonical cwd, nonempty MCP/directory inputs, unknown/unowned session, overlapping prompt, valid Text/ResourceLink, in-project file containment, and zero Host calls on invalid inputs. Test separate sessions can prompt concurrently without sharing state.
- [ ] Run `go test ./...` in `plugins/acp -run 'Test(Session|Prompt)' -count=1`; expect failures at new behavior.
- [ ] Implement validation and ownership gates before each mutation; use exactly the Control parameter keys verified in `internal/rpc/control.go` and ACP-01 fixture. Do not translate a ResourceLink into a fetched remote file or an instruction.
- [ ] Rerun focused tests; assert returned ACP session ID equals Control's ID and prompt's active run ID equals `turn/start` response. Preserve the Host/Journal as state authority.

### Task 3: Projection and terminal response

**Files:** Create (proposed) `plugins/acp/{events.go,events_test.go}`; extend `prompt.go` and its tests; inspect `internal/domain/event.go`, `internal/runtime/payloads.go`, `internal/rpc/control.go` for authoritative event shape.

**Interfaces:** Register `Host.OnEvent` before starting Runs. Dispatch only `run/event` belonging to an active owned `(session_id,run_id,subscription_id)`; sort/deduplicate by committed `seq`, replay via `run/subscribe` from `after_seq` on recoverable gaps, and fail closed if replay cannot close the gap. Project only G0-approved `model.delta` → `agent_message_chunk`, `tool.requested` → `tool_call`, `tool.started`/`tool.finished` → `tool_call_update`; reserve gate events for ACP-04. `run.completed` → `end_turn`; `run.cancelled` → `cancelled`; `run.failed` → sanitized JSON-RPC error. No raw Journal JSON escapes onto ACP stdout.

- [ ] Write failing tests with a fake committed stream for initial replay plus live race, duplicated and missing seq, two interleaved sessions, unknown event type, reasoning payload and private-path/secret sentinel. Test terminal response occurs only after earlier projected notifications are written.
- [ ] Run `go test ./...` in `plugins/acp -run 'Test(Event|Terminal|Replay)' -count=1`; expect failures before projection.
- [ ] Implement bounded field allowlists, stable tool IDs and ordered per-run event queue. Unknown event types are ignored without exposing raw payload; unfillable gap/`run/stream_error` cancels the run and fails its prompt.
- [ ] Rerun focused tests and the package suite; record actual transcript and event sequence. Do not claim real IDE interoperability yet; ACP-05 supplies it.

**Handoff:** ACP-04 consumes the exact connection-owned session/run map, reverse-request writer and event dispatcher. Report interface changes or SDK incompatibilities to the supervisor for ACP-04/05 plan revision. Evidence: fake-Host tests, stdio transcript, no direct Journal/Policy import, `just ci` when in a full CI environment.
