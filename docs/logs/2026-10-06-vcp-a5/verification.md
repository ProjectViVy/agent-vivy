# A5 verification

```
go test ./cmd/vivy-code -count=1        ok (all parse-args + 5 new subcommand tests)
go test ./internal/app/settings ./internal/config ./internal/mcphost -count=1
  ok settings 0.579s / config 0.011s / mcphost 0.009s
go build ./...   clean; go vet clean; gofmt applied
```

Coverage: add/list/remove round-trip persists to settings.yaml (verified via
settings.Load on the real path), `--command npx -y foo` keeps trailing args,
endpoint with `?api_key=` is rejected by schema validation and NOT persisted,
remove-missing exits 1, `mcp status` probes without crashing, `mcp login`
prints deferral, `config set`+`get` round-trips through real schema
validation (provider prerequisite honored), unknown paths exit 1, secret
paths exit 2 with a D-010 message.

`just ci` deferred per plan.
