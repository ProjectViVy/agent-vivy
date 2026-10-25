# Verification

Host: macOS (arm64), `GOTOOLCHAIN=go1.26.4` (the pinned toolchain; ambient
Go 1.27.1 cannot build grpc in this tree).

## Commands

```
go test ./internal/runtime -run 'TestParentFeedMessagesDropsChildTranscriptsOnly|TestContextStatusCountsOnlyParentFeed|TestRunFeedExcludesChildTranscripts' -count=1
ok  agent-vivy/internal/runtime

go test ./internal/runtime/... ./internal/app/... ./internal/provider/... -count=1
ok  agent-vivy/internal/runtime            68.602s
ok  agent-vivy/internal/runtime/compaction  0.301s
ok  agent-vivy/internal/app                19.070s
ok  agent-vivy/internal/app/settings        1.035s
ok  agent-vivy/internal/provider            0.652s
```

New tests:

- `TestParentFeedMessagesDropsChildTranscriptsOnly` — child rows leave the
  feed, parent and legacy (empty run id) rows stay, store keeps all five rows.
- `TestContextStatusCountsOnlyParentFeed` — `total_messages` still 5 while
  `feed_messages` is 3 and `feed_tokens` is below the unfiltered estimate.
- `TestRunFeedExcludesChildTranscripts` — a real `Service.Run` with a seeded
  child transcript; the captured model input contains the parent history
  markers and neither child marker.

## Evidence read (not modified)

Read-only queries against `.workspace/dev/home/vivy.db`:

- `sess_a3e51e9c26f6d4a4`: 206 messages / 280,905 bytes; child runs 164 /
  275,289 (94 `tool` rows, 277,181 bytes); primary `run_b67af7138e043698` 42 /
  5,616.
- `model.call.finished` usage for that run peaks at `prompt_tokens=18788`.

## Unrun gates

`just wiring-ci`, `just kernel-ci` and `just d02-ci` require a Linux-filesystem
WSL/Linux checkout and were not run on this macOS host. No UI, schema, storage
or provider code changed, so the UI and protocol lanes are unaffected.

## Provider 400 evidence captured during the same investigation

Two `agent` children failed with `invalid params, 400 (2013)`
(`child_664aad8af1baae3b`, `child_fabef09d165b1a41`). Their journaled `wire`
summaries are byte-identical in shape: 3 messages (system `string`, user
`array`, user `array`), 17 tools, 12,463 body bytes, no assistant row. A sibling
child in the same minute (`child_b6003d6c6f0e57c8`) carried the same 17 tools
and the same system/contract rows and succeeded; the only journal difference is
the final user text (58 bytes vs 1,215 bytes).

Consequence: the assistant-content-array hypothesis in
`WIRE-ASSISTANT-CONTENT-NORMALIZATION` does not apply to this failure — there is
no assistant row. The failing call is not retried by the new classifier (no
`auto_retry.*` event), which is the intended behavior.
