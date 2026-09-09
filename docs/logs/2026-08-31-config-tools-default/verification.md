# Verification — 2026-08-31 config tools default surface retained

## Commands and results

```text
go test ./internal/config/ -count=1
  -> ok  agent-vivy/internal/config  1.150s

just ci
  -> EXIT=0 (fmt/vet, all Go tests, 175 UI tests, and UI build all passed)
```

## Added test

`TestToolsSectionWithoutEnabledKeepsDefault`（internal/config/config_test.go）：

- The `tools:` section omits the `enabled` key (all other fields are retained) → loading succeeds, and `cfg.Tools.Enabled` has the same length (26) as `Default().Tools.Enabled`.
- An explicit `enabled: []` → `Load` returns a validation error and does not silently allow it through.

## Real-path evidence (before the fix)

After deleting the two `enabled:` lines from the local config.yaml, `just run`:

```text
startup aborted: invalid config config.yaml:
  tools.enabled must list at least one tool
```

After the fix, the same config.yaml starts successfully (see the re-verification steps in this log's acceptance document).
