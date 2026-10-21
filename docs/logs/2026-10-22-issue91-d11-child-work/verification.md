# Verification

Verification ran in `/home/mastwet/work/vivy-issue91-d11-child/agent-vivy` on Debian WSL, an independent Linux-filesystem clone with its own sibling Laputa at `9acc999301d5d575da2cea902a1c6d213763f381`. Candidate source is dependency checkpoint `ecaf7943dd98e036127f5b4e66a97dbad744eb60` plus this lane's changes. The shared service field/forwarding rename was applied in that disposable test checkout only; the integration owner owns its durable edit. No production Journal was accessed.

Environment: `PATH=/home/mastwet/.local/go/bin:/home/mastwet/.nvm/versions/node/v24.21.0/bin:/home/mastwet/.local/bin:$PATH`, `GOFLAGS=-p=2`.

- `gofmt -w` on affected Go files: passed.
- `go test ./internal/domain ./internal/storage -count=1`: passed, including exact child input/authority and private Work receipt regressions.
- `go test ./internal/storage/sqlite -run 'TestWorkExactInputRestartAndUnavailableHistory|TestChildExactInputReplayConflictAndUnavailableMailbox' -count=1`: blocked at `Open`, as the manifest correctly refuses the missing 042..045 sequence before 046. Tests compiled and were selected; assertions beyond `Open` have not run in this lane.
- `go test ./internal/runtime ./internal/rpc -run 'TestWorkPublication|TestModelWorkIdentity' -count=1`: blocked by parallel dependency adaptation, including removed upstream `evolution.RunBinding.StrategyDigest` and INOFY `ProgramMeta.DefinitionDigest`/definition draft digest fields. No API stubs or fake migrations were added.
- Focused code search across all edited production child/Work owners found no `RequestHash`, `RequestDigest`, `AuthorityCeilingDigest`, `ActivationRequestDigest`, `sha256`, or old hash helper producer.
- `git diff --check`: passed before commit.

`just wiring-ci`, `just kernel-ci`, affected Face gates, browser smoke at 3015, race, native Windows and real PostgreSQL execution were not run in this incomplete lane. The parent integration candidate must run the required scoped gates after all D11 owners land; these lanes are not reported green here. Browser code is unchanged. The paired SQL and PostgreSQL adapter changes remain unverified against a live server locally.
