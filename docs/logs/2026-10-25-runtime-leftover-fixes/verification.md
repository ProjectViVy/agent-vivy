# Verification

Host: macOS arm64, `GOTOOLCHAIN=go1.26.4`, serial package runs (`-p 1`).

## Per-commit targeted runs

| Commit | Command (pattern within `./internal/runtime/`) | Result |
| --- | --- | --- |
| rebind on reload | `TestScheduleEngineReload`, `TestChildEngineInheritsModelRetryDecider` | `ok` |
| run-scoped budget | `TestChildModelRetry`, `TestOverflowRetryConsumesBudget`, `TestToolResultOrder`, `TestOverflow`, `TestBare2013` | `ok` |
| compaction folds | `TestCompactSessionExcludesChildTranscript`, `TestCompactForOverflowExcludesChildTranscript` (verbose) | both `PASS` |
| 2013 classifier | `TestBare2013`, `TestIsContextOverflowClassifier`, `TestUnambiguous`, `TestToolResultOrder` | `ok` |
| dead describer | `TestDescribeModelRequest`, `TestServiceFeedsToolTraceAndRequestMetadata` | `ok` |

## Full gates (at HEAD after all five commits)

```text
GOTOOLCHAIN=go1.26.4 go test -p 1 -count=1 ./internal/runtime/...
ok   agent-vivy/internal/runtime               68.976s
ok   agent-vivy/internal/runtime/compaction     0.369s

GOTOOLCHAIN=go1.26.4 go test -p 1 -count=1 ./internal/config/... ./internal/provider/... ./internal/app/...
ok   agent-vivy/internal/config     0.329s
ok   agent-vivy/internal/provider   0.655s
ok   agent-vivy/internal/app       19.881s
ok   agent-vivy/internal/app/settings 1.033s

GOTOOLCHAIN=go1.26.4 go build ./...
(no output)

GOTOOLCHAIN=go1.26.4 go vet ./internal/runtime/...
(no output)

gofmt -l <changed files>
(no output)

git diff --check
(no output)
```

`rg 'overflowRecovered|deterministic2013Phrases|describeModelRequest\b' internal`
returns no matches, confirming the superseded names are gone rather than
kept in step.

## Not run

- Race detector, Windows-native checks, kernel CI and WSL lanes: not available
  on this host; no ownership or pack change is involved.
- Browser smoke: no UI or Face surface changed.
- Live provider replay: the retry paths are covered by scripted models; the
  owner-visible checks are in `acceptance.md`.
