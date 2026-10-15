# Verification

## 1. How the fault was found

Reported symptom: a turn failed with the generic UI message while the backend log
said only

```text
WARN run failed err="engine event error: [NodeRunError] error, status code: 422,
status: 422 Unprocessable Entity, message: openai_error
node path: [node_1, ChatModel]"
```

The upstream body is not preserved by the provider error, so the fault was
isolated by replaying the **real** tool set through the **real** provider path
against the registered gateway (a temporary `toolwireprobe`-tagged test, deleted
after use; no local relay — the process talked to the gateway directly):

```text
family=openai-completions     ... ref_err=<nil>
family=openai-responses       ref_err=provider adapter is DEFERRED-INDEFINITE
family=anthropic-messages     ... ref_err=<nil>
POST /v1/chat/completions     status=200
POST /v1/responses            status=404  {"error":{"message":"openai_error",...}}
POST /v1/messages             status=200
```

Then every tool the default generation enables was bound to the model one
request at a time:

```text
ALL TOOLS -> error, status code: 422, status: 422 Unprocessable Entity, message: openai_error
  tool multiedit       -> ok        tool task_list       -> 422
  tool execute         -> ok        tool skills_list     -> 422
  tool list_dir        -> ok        tool get_goal        -> 422
  tool grep            -> ok        tool enter_plan_mode -> 422
  tool write_file      -> ok        ... every other tool -> ok
```

The four rejected tools share one property: their `domain.ToolSpec` declares
neither `Schema` nor `Params`, so `toolAdapter.Info` left `ParamsOneOf` nil.

## 2. The accepted wire shape (measured, not assumed)

Direct `POST /v1/chat/completions` with a single tool, varying only the
parameter encoding:

| `function.parameters` | Status |
|---|---|
| key omitted | 422 |
| `null` | 422 |
| `{}` | 200 |
| `{"type":"object","properties":{}}` | 200 |
| `{"type":"object","properties":{},"additionalProperties":false}` | 200 |

Root cause pinned in the pinned dependency: Eino documents nil `ParamsOneOf` as
"no input parameters", and `eino-ext/libs/acl/openai@v0.1.17/tool.go` has no
`omitempty` on the field, so nil is serialized as `"parameters": null`.

## 3. After the fix

Same replay, real tool set, real provider path, direct to the gateway:

```text
schema task_list       {"properties":{},"additionalProperties":false,"type":"object"}
schema skills_list     {"properties":{},"additionalProperties":false,"type":"object"}
schema enter_plan_mode {"properties":{},"additionalProperties":false,"type":"object"}
schema get_goal        {"properties":{},"additionalProperties":false,"type":"object"}
ALL TOOLS -> ok
```

## 4. End-to-end, real run

A backend built from this tree (`go build -tags vivy_headless -o
.workspace/e2e/vivy-backend.exe ./cmd/vivy`, 2026-10-15 14:50) was started on a
throwaway home with the user's own registry entry and talked straight to the
gateway (no relay), then driven over the control-plane WebSocket exactly as the
browser does:

```text
[session] sess_68e4c45e7c69494d
[run] {"run_id":"run_20aca7454c9923ac","status":"accepted"}
[messages] [{"role":"user","content":"Reply with exactly: OK"},
            {"role":"assistant","content":"OK"}]
```

The backend log for that run carries no `run failed`. Before the fix the same
path produced the 422 above.

## 5. Gates

```text
go vet ./...                                     exit 0
gofmt -l (both changed files)                    clean
go test ./internal/runtime  -count=1             ok 330.463s
go test ./internal/tools    -count=1             ok   3.983s
go test ./internal/provider -count=1             ok  11.300s
go test ./internal/app      -count=1             FAIL — environment, see §6
```

The new regression test `TestToolAdapterDeclaresArgumentObjectForArgumentlessTool`
runs in the untagged build (`internal/runtime`), so it guards the invariant in
the ordinary gate.

## 6. Attribution of the `internal/app` failures

Every failure is `testMemoryLoop*` failing at fixture setup with

```text
symlink C:\Users\com01\Desktop\laputa\mentle\models
        ...\.workspace\tmp\TestMemoryLoopActivityContinuity...\garden\models:
A required privilege is not held by the client.
```

This Windows token has no `SeCreateSymbolicLinkPrivilege`. Re-run with the
change stashed (`git stash push -u -- internal/runtime`):

```text
--- FAIL: TestMemoryLoopActivityContinuity (0.02s)
    memory_loop_activity_test.go:22: symlink ... A required privilege is not held by the client.
FAIL	agent-vivy/internal/app	1.862s
```

Identical on the clean tree, so the failures are environmental and not caused by
this change. The same host limitation is why the SDK pack/UI-seal suites fail
(see `docs/logs/2026-10-15-custom-provider-endpoint/verification.md`).

## 7. Not run

- `just ci` in full: it stops at `go test ./...` on the symlink fixtures above,
  and every non-environmental package relevant to this change
  (`internal/runtime`, `internal/tools`, `internal/provider`) was run directly
  and passes. `go vet ./...` covers the whole module.

## 8. Temporary artifacts

All diagnostic material was deleted: the tagged probe tests
(`zz_toolwire_probe_test.go`, `zz_wirevariant_probe_test.go`), the recorder and
driver scripts, the throwaway homes, and the built probe binary. Nothing under
`.workspace/` is tracked; `git status` shows only the two changed files plus this
log directory.
