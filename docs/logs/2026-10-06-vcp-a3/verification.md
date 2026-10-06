# A3 verification

## Tests (all green)

```
go test ./internal/app/ -run 'TestCodeFace' -count=1 -v
=== RUN   TestCodeFacePrintModeStreamsAssistantText        PASS
=== RUN   TestCodeFaceJSONModeEmitsOrderedRecords          PASS
=== RUN   TestCodeFacePrintModeCancelsBlockedRunLoudly     PASS
=== RUN   TestCodeFaceJSONModeContinuesNewestSession       PASS
=== RUN   TestCodeFaceRPCModeGoldenTranscript              PASS
=== RUN   TestCodeFaceRPCModeQueuesFollowUpWhileRunning    PASS
ok  agent-vivy/internal/app  0.601s
```

`TestCodeFaceRPCModeGoldenTranscript` drives a real composed kernel
(`RunFaceWithAppOptions` + scripted deepseek SSE mock) over an `io.Pipe`
stdin/stdout pair and asserts the golden transcript:

- `get_state` → session exists, `isStreaming:false`
- `prompt "hi"` → `{disposition:"started", run_id}` + ordered event tail
  session → turn_start → agent_start → message_start → message_update →
  message_usage → message_end → turn_end → agent_end → agent_settled
- `get_last_assistant_text` → `"rpc answer"` (the mock's stream reassembled)
- `set_session_name`, `switch_session` round-trip
- `steer`, `get_tree`, `bash` → `success:false` with reasons
- `bogus` → `success:false`; session stays healthy afterwards
- session id stable across the whole transcript

`TestCodeFaceRPCModeQueuesFollowUpWhileRunning` covers the follow-up path
(disposition ∈ {started, queued}) and settle ordering.

## Conformance

Internal-suite source digest re-pinned (sdk/tui/face + facerun changes):
`60521960…` → `9e024d18bbf65aa3470b9d07a9e7704999adddb3b9995125c656ae38d936b39d`
in `sdk/internal/assembly/conformance_results.json` (5 internal suites).

```
go test ./sdk/internal/conformance/ -run TestCheckedInProviderConformance -count=1
ok  agent-vivy/sdk/internal/conformance  65.201s
```

## Build/vet

`go build ./sdk/tui/face ./cmd/vivy-code` clean; `go vet` clean;
gofmt applied.

## Not run

- `just ci` full sweep — deferred per plan (35m+); per-story focused gates
  are the evidence above.
- Live `vivy-code --mode rpc` binary smoke — covered in-process via
  `RunFaceWithAppOptions`; A4's codeclient will add the real-subprocess gate.
