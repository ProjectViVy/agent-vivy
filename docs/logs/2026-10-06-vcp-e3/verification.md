# VCP E3 verification

Date: 2026-10-06 (UTC). Branch `feat/vivy-code-parity`.

## Focused suite

```
go test ./internal/runtime -run 'TestToolSearchActivatesDeferredTool' -count=1 -v
--- PASS: TestToolSearchActivatesDeferredTool (0.07s)
```

## Acceptance mapping

| Plan acceptance | Evidence |
| --- | --- |
| Model asks tool_search → deferred tool activates | Journal `tool.finished(tool_search)` → `noteToolSearchMatches` → `svc.ToolActivation` = `[echo_info]` |
| Next turn calls it directly | Post-search `model.request` payloads list `echo_info` in `SelectedTools`; `echo_info` executes and returns "parity" |
| Hidden tools never appear | Hidden tools are bound in `staticTools` + `suppressedTools`, never in `DynamicTools` the search indexes (engine.go) |
| Direct tools need no activation | `ResolveToolExposure` fixed core → direct; they are statically disclosed |
| Journal records activation | `tools.exposure_changed` (tools/activate path) + `tool.finished` fold (search path), replayed by `sessionToolActivation` |

## Notes

- The plan's custom BM25 module was not built: Eino
  `dynamictool/toolsearch` (pinned @v0.9.13) already implements
  query/select matching, `max_results`, and forward selection. Recorded
  as a deliberate Eino-native decision in `summary.md`.
- `tool_search` result vocabulary is `{"matches":[names]}` — the exact
  JSON the E2 fold parses, verified end-to-end here.
