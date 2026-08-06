# AGENT-VIVY — Eino Capability Verification (Task A1, M0 Spike)

> Status: VERIFIED — GO for the two-layer checkpoint bridge (C6).
> Closes: D-034, SR-2. Gate for C6 released.
> Target: ONLINE `github.com/cloudwego/eino@v0.9.13` (module cache), NOT the
> vendored `.workspace/eino/` reference.
> Evidence: `spike/einoverify/` — deterministic, API-key-free, reproducible
> via `go run ./spike/einoverify`. Step numbers below cite the spike's step log.
> Date: 2026-08-07

---

## 1. Verdicts

| # | Claim under test | Verdict | Evidence |
|---|---|---|---|
| (a) | `CheckPointStore{Get,Set}` exists and injects via `RunnerConfig` | VERIFIED | §2.1 |
| (a+) | Optional `CheckPointDeleter{Delete}` exists | VERIFIED | §2.1 |
| (b) | Default checkpoint serializer is engine-owned gob; store receives opaque `[]byte` pass-through | VERIFIED | §2.2 |
| (c) | Tool-level interrupt → checkpoint persist → `ResumeWithParams` with approval payload works at Runner level | VERIFIED | §2.3 |
| (c+) | Public hook ordering: checkpoint durable BEFORE interrupt event reaches the consumer (D-029 feasibility) | VERIFIED | §2.3, steps 17→19 |
| (d) | Streaming model output through `ChatModelAgent` + `Runner` | VERIFIED | §2.4 |
| (e) | Cancellation produces a single terminal `CancelError` event and persists checkpoint | VERIFIED | §2.5 |
| GO/NO-GO | Two-layer checkpoint bridge approach (IMPLEMENTATION-PLAN §5.3/§6) | **GO** | §3 |

No outer-loop fallback (RK-3 stop-loss) is needed for C6.

## 2. Evidence

### 2.1 CheckPointStore contract (verified against v0.9.13 source + runtime)

```go
// adk aliases (v0.9.13):
type CheckPointStore = core.CheckPointStore
// core.CheckPointStore:
Get(ctx context.Context, checkPointID string) ([]byte, bool, error)
Set(ctx context.Context, checkPointID string, checkPoint []byte) error

// optional:
type CheckPointDeleter = core.CheckPointDeleter
Delete(ctx context.Context, checkPointID string) error
```

- Injection: `adk.RunnerConfig{Agent, EnableStreaming, CheckPointStore}` —
  exactly the seam planned in IMPLEMENTATION-PLAN §6. The spike's
  `recordingStore` implements all three methods and was driven end-to-end.
- Checkpoint ID is caller-allocated per run via the run option
  `adk.WithCheckPointID(id)` on `Query`/`Run`. This matches D-029 step 1
  ("Vivy allocates checkpoint id"). Without this option, interrupts are NOT
  persisted (save path is skipped when the id is nil) — Vivy must always set it.

### 2.2 Default payload serializer

- Payload is a gob-encoded engine-internal struct (`RunCtx`,
  `InterruptID2Address`, `InterruptID2State`, `EnableStreaming`), NOT JSON.
- Observed: single-interrupt run → `store.Set("ckpt-run-1") len=28995`,
  cancel-path checkpoint `len=15281`; leading bytes `78fe0121…` (gob wire
  format) — step 17, 35.
- The store receives/returns opaque bytes; Vivy's `BlobStore` can treat them
  as pass-through blobs (D-028 holds: no need to parse engine bytes).
- Consequences (feed into `VersionedCheckpointStore`, D-030):
  - checkpoint bytes are version-coupled to the eino release → record the
    engine version per generation and refuse cross-version resume;
  - blobs are large relative to run size (~29 KB for a trivial run: full
    message history + tool schemas) → size budgets in BlobStore/SQLite.
- Resume ignores the runner's construction-time `EnableStreaming`; it uses
  the mode persisted in the checkpoint (runner source, v0.9.13). No action
  needed; documented so the adapter never assumes otherwise.

### 2.3 Interrupt → checkpoint → ResumeWithParams (the approval path)

Spike scenario V3 (tool `send_message`, scripted model):

```text
14  model.Generate call=0                       # model requests tool call
15  send_message invoked ... wasInterrupted=false
16  event: MESSAGE role=assistant toolcalls=1   # tool.requested source
17  store.Set("ckpt-run-1") len=28995           # checkpoint durable
18  event: INTERRUPT contexts=1                 # interrupt event delivered AFTER Set
19  interrupt received; checkpoint already durable=true   # D-029 ordering
20  root-cause interrupt id="5973c195-…"        # from InterruptContexts[IsRootCause].ID
```

D-029 write-ordering feasibility: the Runner persists the checkpoint
internally (`runnerSaveCheckPointImpl`) **before** forwarding the interrupt
event to the consumer iterator. Therefore Vivy's mandatory order
(2 Set-durable → 3 interrupt visible → 4 verify → 5 journal commit → 6 UI)
is achievable with zero custom synchronization: by the time the runtime sees
the interrupt event, step 2 is already done.

Resume with the approval decision:

```go
runner.ResumeWithParams(ctx, "ckpt-run-1", &adk.ResumeParams{
    Targets: map[string]any{rootCauseID: "approved"},
})
```

```text
22  store.Get("ckpt-run-1") -> existed=true
23  send_message invoked ... wasInterrupted=true     # tool re-executed
24  resume ctx: isTarget=true hasData=true data="approved"
26  event: MESSAGE role=tool tool=send_message content="send_message executed, decision=approved"
27  event: MESSAGE role=assistant content="run complete after approved tool"
```

Tool-side API (public, `components/tool`): `tool.Interrupt`,
`tool.StatefulInterrupt`, `tool.GetInterruptState[T]`,
`tool.GetResumeContext[T]`. Denial maps trivially: resume the same target
with a denial payload; the tool returns a denial result and the model
continues (same mechanism, different payload).

Interrupt addressing: `InterruptInfo.InterruptContexts[*]` exposes `ID`
(resume-target key), `Address` (agent/tool segments; for tools the segment
carries tool name + tool-call id as SubID), `Info` (user-facing), and
`IsRootCause` — everything the approval record binding (run_id + tool_call_id)
needs is reachable without parsing checkpoint bytes.

### 2.4 Streaming (V1)

`RunnerConfig{EnableStreaming: true}` + scripted `model.Stream` produced one
`STREAM` event carrying `MessageStream`; chunks `Hel` / `lo ` / `Vivy!`
received in order (steps 1–6). Maps to `model.delta` → `model.completed`.

### 2.5 Cancellation (V4)

`adk.WithCancel()` returns `(AgentRunOption, AgentCancelFunc)`; cancelling
mid-stream terminated the stream (`stream canceled`), surfaced exactly ONE
terminal error event (`CancelError: agent canceled: mode=0`), and persisted
the checkpoint when `WithCheckPointID` was set (steps 32–39). This supports
E1's exactly-one `run.cancelled` invariant; `CancelAfterChatModel` /
`CancelAfterToolCalls` safe-point modes also exist if V0 wants graceful-stop
semantics later.

## 3. GO/NO-GO for the checkpoint bridge (C6)

**GO.** The planned bridge
`EinoCheckpointAdapter → VersionedCheckpointStore → BlobStore` maps 1:1 onto
verified APIs. Design refinements to carry into C6:

1. `EinoCheckpointAdapter.Get/Set/Delete` is a thin wrapper; durability =
   `VersionedCheckpointStore` commit (new generation + atomic pointer flip),
   because the Runner treats `Set` return as durable.
2. Runtime must pass `adk.WithCheckPointID(vivyCheckpointID)` on every
   `Query`/`Run`/resume; allocate the id BEFORE starting the run.
3. On interrupt event: verify generation/checksum, then append ONE journal
   commit (`run.suspended` + `approval.requested`), then expose to UI —
   exactly D-029 steps 4–6; no extra synchronization needed.
4. Resume target key = `InterruptContexts[IsRootCause].ID`; persist it in the
   Approval record alongside `run_id` + `tool_call_id` at suspend time.
5. Record eino version per checkpoint generation; fail closed on
   cross-version resume.

## 4. Deltas vs IMPLEMENTATION-PLAN (minor refinements only)

| Plan statement | Verified reality | Action |
|---|---|---|
| §4.1 `ProviderRef.ChatModel` returns a `model.ChatModel`-like shape | `ChatModelAgentConfig.Model` is `model.BaseModel[*schema.Message]`; tools require `model.ToolCallingChatModel` (`WithTools`) | ProviderRef returns `model.ToolCallingChatModel`; finalize seam in C2 |
| §6 mapping assumes explicit run-start/tool-requested engine events | Engine emits message/tool-result events + interrupt/cancel actions; `run.started` and `tool.requested` are synthesized by Vivy runtime from call start and assistant tool-call messages | Keep the mapping table; note synthesis in C4 |
| §6 `ResumeWithParams(ctx, checkpointID, &ResumeParams{Targets})` | Exact match | none |

## 5. Reproduce

```powershell
cd diva-go/agent-vivy
& "C:\Program Files\Go\bin\go.exe" run ./spike/einoverify
# expect: "ALL VERIFICATIONS PASSED", exit 0
```

No API keys, no network at runtime (module already in cache via goproxy.cn).
