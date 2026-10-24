# Verification

Host: macOS (arm64), Go `1.26.4` via `GOTOOLCHAIN=go1.26.4`,
`GOPROXY=https://goproxy.cn,direct`. Tests ran serially (`-p 1`) on a 16 GiB
host.

## Reproduction (before the fix)

```text
go test -p 1 ./internal/app -run TestAgentChildOnTextOnlyModelOmitsImages -count=1 -v
--- FAIL: TestAgentChildOnTextOnlyModelOmitsImages (0.28s)
    WARN child run failed run=child_213f964a90d6bb14
    err="engine event error: [NodeRunError] error, status code: 400, status: 400 Bad Request,
    message: invalid params, 400 (2013) ... node path: [node_1, ChatModel]"
    child run failed with an attachable image in a text-only model request:
    "sub-agent failed: 模型服务拒绝了本次请求（HTTP 400）..."
```

This matches the operator's report byte for byte. The test drives a real app
(`New` + control plane), `VIVY_MODEL=deepseek-chat`, a child `read_file` of a
PNG, and a fake provider that rejects any request carrying an image part with
the DeepSeek 2013 body.

## After the fix

```text
go build ./...                                             ok
go vet ./internal/...                                      ok
go test -p 1 ./internal/provider ./internal/tools          ok  0.6s / 2.8s
go test -p 1 ./internal/runtime -count=1                   ok  67.9s
go test -p 1 ./internal/app -count=1                       ok  17.7s
    (includes TestAgentChildOnTextOnlyModelOmitsImages, PASS)
go test -p 1 ./internal/rpc ./internal/storage/...         ok  19.1s (all storage packages)
git diff --check                                           clean
```

The E2E asserts: parent completes, the agent tool result carries the child's
completed answer (not `sub-agent failed`), the provider saw zero image requests,
and the child model input contains the `read_file` text degradation.

## Unit coverage added

- `internal/provider/catalog_test.go`: declared `true`, declared `false`, and
  absent image capability project to `SupportsImages`/`ImagesKnown` correctly.
- `internal/provider/vendor_test.go`: embedded `deepseek-chat` declares
  text-only input; an unannotated model stays unknown.
- `internal/tools/read_image_test.go`: a text-only context produces no parts
  envelope and an honest text description; the unbound context still attaches.

## Not run

- `just wiring-ci` + `just kernel-ci` from an independent Linux-filesystem
  checkout: not possible on this macOS host (AGENTS.md gate; unchanged from the
  prior sub-agent deliveries).
- Split-pair browser smoke at `http://127.0.0.1:3015` with a live provider key:
  owner-run. The deterministic app E2E is the automated evidence.
