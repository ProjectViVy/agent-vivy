# E1-BML-02: Actual model-request consumption Implementation Plan

> **For agentic workers:** After separate execution authorization, use `superpowers:executing-plans` task by task. Do not launch child sessions. Steps use checkbox syntax.

**Goal:** Prove default VIVY consumes reopened BML state in a subsequent real model HTTP request and uses that material in a deterministic response.
**Architecture:** Ordinary App composition, generated BML source, real ContextHost and Service.Run, existing EinoExt provider aimed at a local scripted HTTP server. The server derives its answer solely from received BML context; neither expected answers nor database reads are available to it.
**Tech Stack:** Go, httptest, existing App/DialControl JSON-RPC, Eino v0.9.13 and EinoExt OpenAI v0.1.13.
**Spec:** [Architecture](architecture.md), A1–A4/A7–A9 and Eino capability check; BML-R2/R3/R6. Predecessor/status: [index](index.md).

## Global constraints

- Main's current ordinary BML admission is the target. Do not enable LIFE, Recap, DIVA or a coding resident-Recap policy to make the test pass.
- No Eino imports in App tests, no custom runtime, no production ScriptedModel modification.
- Use loopback only, dummy credentials and bounded requests; never invoke a paid endpoint.
- Each scenario owns its own real disk data root and fresh session; no earlier inference can contaminate recall with a duplicate history record.
- Public long-term correction/deletion changes future reads, not prior Journal/model inputs.

## Files and interfaces

- Create (proposed): `internal/app/bml_closeout_test.go`, package `app`. Keep the fixture local and small.
- Reuse patterns/read-only helpers from `internal/app/facehost_test.go`: `newDeepSeekTestConfig`, `callControl`, `waitFor`, `runEvents`; reuse App `New`, `WithoutEars`, `WithoutGateway`, `DialControl`, `Close`.
- Read only: `internal/app/{app,assembly_sources,default_generation_test}.go`, `internal/runtime/{service,contextadapter,scriptedmodel}.go`, `internal/provider/openai.go`.
- Do not reuse `newMemoryLoopFixture`'s DIVA overlay or its Mentle card parser.

Consumes 01's verified public outcomes and existing cfg fields:
`Storage.DataDir`, `Storage.SQLite.Path`, `Runtime.WorkspaceRoot`,
`Governance.Profile`. Set the three paths under one scenario root; set
`Governance.Profile="full_auto"` only in this disposable test config so
authenticated module writes use the existing allowed policy.

Proposed private helper signatures, all test-only:

```go
func newBMLCloseoutModel(t *testing.T) (*httptest.Server, func() [][]byte)
func bmlCloseoutReply(raw []byte) (string, error)
```

The first helper captures deep copies under a mutex and calls the second to
produce the reply. It must not accept a marker/expected-answer parameter. Output
is a valid streamed/nonstreamed chat.completions response using the existing
`scriptedDeepSeekServer` wire pattern, with JSON encoding rather than unsafe
string interpolation. Support exactly the tested text/string or text-part input;
return an explicit fixture error on an unexpected shape.

Produces `TestBMLCloseoutActualModelConsumption`, subtests `absent`, `present`,
`corrected`, `removed`, plus captured request/assistant evidence for each.

## Review focus

1. Fixed output passes without reading BML: request-derived reply and absent/removed controls must fail that shortcut.
2. Secret appears in user query, prior history or server closure: forbid all three; use a separate fresh root/session per subtest.
3. Schema parser misses native text parts: decode both string content and text arrays at the real provider wire boundary.
4. Candidate body gains instruction authority: require BML marker and payload in user-role data and absent from system messages.
5. Wrong assembly or a skipped DIVA fixture is called acceptance: assert the generated `vivy.memory.bml` provider exists and fail, never skip.

## Ordered tasks

- [ ] **1. Revalidate the current composition and baseline.** Read `TestDefaultGenerationComposesEstablishedOptionalHosts` and `TestServiceRunProjectsGenericContextHostIntoModelInput`. Confirm `VIVY_API_BASE` selects the local URL through the existing resolver and source ID is `vivy.memory.bml`. Do not transplant #79's pins or bound API. Run `go test ./internal/app -run '^TestLoopbackControlCompletesApprovedConversation$' -count=1`; expected one executed pass. Failure at provider setup blocks this Story; do not start a Provider repair lane.

- [ ] **2. Implement the local HTTP script and capture.** Set `DEEPSEEK_API_KEY` to a dummy fixture key, `VIVY_PROVIDER=deepseek`, `VIVY_API_BASE` to the httptest URL, and preserve the existing pinned engine-version override/cleanup pattern. The request handler extracts only a user-role block starting `[context: vivy.memory.bml/`, checks its identity/revision framing, and returns that block's exact body; absent block returns literal `BML_UNKNOWN`. The handler only has raw request bytes. Capture request path/body and verify the model field equals the selected raw model ID; never capture auth headers. No arbitrary “expected body” argument.

- [ ] **3. Build four isolated scenario preparations.** Generate a per-scenario ASCII nonce in the test body. Use `query := "bmlprobe"+nonce` as the entire turn text and content `query+" secret="+otherRandomNonce`. Retain old/current content only in test assertions, never in the HTTP handler closure. Compose App with the default assembly; initialize DialControl; add/update/remove through `module.action.invoke` with `module_id="vivy/memory-bml"` and the `vivy.memory.*` action IDs. Decode outer `status="ok"` and inner mutation outcomes separately. No call to `memory.NewProvider().Query` may substitute for model invocation.

  | Subtest | Preparation before close/reopen | Expected request / answer |
  |---|---|---|
  | absent | No record; no model turn | No BML body or secret; `BML_UNKNOWN` |
  | present | Add one long-term record | Exact ID/revision 1 label + full content; answer derived from that content |
  | corrected | Add, then CAS update with a new random secret at revision 1 | Exact same ID/revision 2 and new secret; no old secret; answer is new body |
  | removed | Add, update, remove at revision 2 | No selected ID or either secret; `BML_UNKNOWN` |

- [ ] **4. Cross the actual restart boundary and run.** Explicitly close the DialControl peer/context and App, require Close success, then create a new App with the same config/root and open a new persistent session. Do not rewrite/re-add memory after restart. Recheck public get/search state, invoke `turn/start` with the query-only text and wait boundedly for `run/get.status="completed"`. Inspect captured HTTP messages and `session/messages` assistant content. Require at least one actual inference and all relevant requests to meet the scenario; do not insist on a fixed incidental request count. Preserve failed run log for diagnosis. No previous inference is allowed during setup: default completion sync could create a second history copy of the secret and invalidate the deletion control.

- [ ] **5. Assert authority and sensitivity.** For present/corrected require context framing to contain the selected native record ID and decimal revision, full body only in user content and never in system content. Negative controls must succeed with `BML_UNKNOWN`, not merely lack an output. A fixture that returns a precomputed success string must be caught by these assertions. Preserve actual raw request snapshots with synthetic values; this proves transport/admission/consumption, not natural-language reasoning quality.

- [ ] **6. Run checks and independent review.**

  ```sh
  go test ./internal/app -run '^TestBMLCloseoutActualModelConsumption$' -count=1 -v
  go test -race ./internal/app -run '^TestBMLCloseoutActualModelConsumption$' -count=1
  go test ./internal/app -run '^TestEinoImportsQuarantined$' -count=1
  ```

  Expected: four executed subtests, no skip/race, quarantine pass. Retain existing loopback regression. Obtain independent review of the handler's only inputs, fresh-session isolation, raw requests and deletion semantics; commit only the new test and owned evidence with the responsible human identity. The integrator runs the final full `just ci` gate.

## Stop conditions and acceptance

Stop for outside-loopback traffic, unavailable default source, wrong storage root,
unclosed first App, unexpected wire shape, missing body/revision, stale secret,
policy denial or a required-test skip. Block 02 only and name the failing seam;
01/03/04 can still progress. Preserve evidence; do not bypass ContextHost, change
policy defaults or repair production code under this task.

Accept with one exact candidate showing disk preparation → full App close/reopen
→ new session → captured request → request-derived assistant result in every
scenario. No live-model quality, Recap, DIVA or cross-store erasure claim follows.
Rollback stops only fixture-owned servers/Apps and removes their disposable roots
after evidence is captured.
