# A4 verification

```
go test ./sdk/codeclient -count=1 -v
=== RUN   TestClientCorrelatesResponsesAndStreamsEvents   PASS
=== RUN   TestClientPropagatesProtocolErrors              PASS
=== RUN   TestClientRejectsModeOverrideAndMissingBinary   PASS
=== RUN   TestClientAgainstRealVivyCode                   PASS (4.58s)
ok  agent-vivy/sdk/codeclient  4.594s
```

- Fake-binary tests (bash script speaking canned wire lines) cover id
  correlation, event ordering (agent_start → message_update×2 →
  agent_settled), protocol-error propagation, and spawn validation.
- `TestClientAgainstRealVivyCode` is the acceptance gate: builds
  `cmd/vivy-code`, spawns it `--mode rpc` against an httptest SSE mock via
  the frozen-env provider trick, and drives prompt → event tail →
  `LastAssistantText` ("codeclient e2e") → `GetState` → `SetSessionName` →
  `Steer` (fails as designed until B1).
- `go vet` clean, gofmt applied.
- Conformance reproduction: `go test ./sdk/internal/conformance -run
  TestCheckedInProviderConformance -count=1` — ok (63s). codeclient is not
  yet a declared module so no descriptor digest needed re-pinning.
- `just ci` deferred per plan.
