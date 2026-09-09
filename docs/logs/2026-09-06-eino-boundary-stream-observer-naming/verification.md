# Verification

Date: 2026-09-06 ｜ worktree `agent-vivy-eino-boundary` (branch `feat/eino-boundary-5-5-5-6`)

```
go test ./internal/runtime/ -count=1 -timeout 60s -run 'TestObservingChatModel'
    → ok  (Begin-before-return, live tee, chunk fail-closed, missing observer,
      inner Stream error, EOF, upstream recv error, WithTools tee, Pipe(8)
      backpressure)

just ci
    → CI_EXIT=0  (~405s)
      fmt-check empty
      ui typecheck + vitest 201 passed + vite build
      go vet ./...
      go test ./...  (internal/runtime 246.695s; internal/app 66.787s;
      internal/rpc 94.218s)
      headless-compile ok
      plugin-ci plugins/* + faces/* ok
```

Post-review: comments narrowed to producer-path / backpressure / fail-closed /
tool-barrier gaps; added recv-error, WithTools, and Pipe(8) tests.

```
go test ./internal/runtime/ -count=1 -timeout 360s
    → ok  137.958s
```

No browser smoke: no user-visible UI/RPC contract change.
