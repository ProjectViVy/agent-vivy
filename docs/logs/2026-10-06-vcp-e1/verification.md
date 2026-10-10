# VCP-E1 verification

Focused plan command plus the touched-package sweep, 2026-10-06.

## Commands and results

```
go test ./internal/tools -run 'Spill|JobRegistry' -count=1          ok 0.442s
go test ./internal/runtime -run 'CommandEnv|ShellPrefix|CommandOutputSpills|NoContextKeeps|CommandBackend' -count=1
                                                                    ok 0.210s
go test ./internal/tools ./internal/runtime -run 'Bash|Execute|Command|Spill|Shell' -count=1
                                                                    ok 0.019s / 2.342s
go test ./internal/tools ./internal/config ./internal/domain ./internal/modules/sandbox -count=1
                                                                    ok all
go test ./internal/runtime -count=1                                 ok 50.114s (full package)
go test ./internal/rpc ./sdk/... -count=1                           ok all
go test ./sdk/tui/... -count=1                                      ok all (after fixture fix)
go test ./sdk/internal/conformance/ -count=1                        ok 83.911s (re-pinned digest)
go vet ./sdk/tui/... ./internal/tools ./internal/runtime ./internal/rpc ./internal/config ./internal/modules/sandbox
                                                                    clean
gofmt -w (all touched files)                                        clean
```

## Behavior evidence (test-level)

- Env injection: `bash -c 'printf "$VIVY_SESSION_ID|$VIVY_RUN_ID|…"'`
  returns `sess-env|run-env|prov-x|model-y|on`; non-allowlisted request env
  (`API_KEY`) still rejected.
- Prefix: bash stdout starts with `PRE\n`; commandline argv `{"a b","c"}`
  renders `PRE\na b c\n` verbatim through `bash -c '<prefix> "$@"'`;
  `execute` without `ApplyShellPrefix` prints `hi` unprefixed.
- Spill: 200 KB `printf` → inline `Stdout` bounded tail, `StdoutTrunc=true`,
  `StdoutTotalBytes=200000`, spill file under
  `<workspace>/.vivy/tool-output/` holds all 200000 bytes.
- `!!`: `Parse("!!x")` → Shell{NoContext}; `shell/start` accepts
  `no_context`; `run.started` journals `"no_context":true`; the run's tool
  rows project for `!` (2 messages) but not for `!!` (0 messages) while the
  journal keeps its full lifecycle for the transcript.
- Strict `shell/start` param validation still rejects unknown fields.

## Known gaps / follow-ups

- `tool_output_spill_bytes` is a global bound; there is no per-tool
  override (plan did not require one).
- GUI has no shell surface; `!!` is TUI-only this story, as planned.
- Spill cleanup is implicit via run-workspace teardown, per plan wording
  ("cleaned with instance teardown").
