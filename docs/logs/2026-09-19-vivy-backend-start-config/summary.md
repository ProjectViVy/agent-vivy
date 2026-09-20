# Vivy backend cannot start — stale providers config from the Studio console

## Symptom

总控台 reported that the managed backend had exited before listening, and the
gateway log ended with:

```text
startup aborted: parse config .../data/studio-home/vivy-console/config.yaml:
  line 9: field bundle_dir not found in type config.Providers
  line 10: field openai not found in type config.Providers
  line 13: field anthropic not found in type config.Providers;
  providers.openai was removed in PROV-P3 — provider metadata is embedded in
  the binary, so keep providers.active only
```

The console's start endpoint returned `ok:false` with that tail, so the dev loop
had no backend on `127.0.0.1:8787`.

## Root cause

`studio/dsh-vivy-console/index.js` → `prepare()` writes the managed backend's
`config.yaml` on every start. That template was still the pre-PROV shape:

```yaml
providers:
  active: openai
  bundle_dir: ".../fixtures/provider"   # deleted in PROV-P1
  openai:    { env_key, default_model } # deleted in PROV-P3
  anthropic: { env_key, default_model } # deleted in PROV-P3
```

PROV-P1/P3 deleted `providers.bundle_dir` and the per-vendor blocks and
deliberately made decoding **strict**, so every other producer was migrated
(`config.example.yaml`, `internal/eval/isolator.go`, `ui/e2e/global-setup.ts`,
the Docker configs). The console template lives in the `studio/` submodule and
was not in that inventory, so the kernel rejected its file at load time and the
process exited 1 before `ListenAndServe`.

The kernel behaved correctly: the removal hint in the error is the intended
product contract, and `providers.active` is the only key that survives. The
defect was the producer, not the schema.

## What changed

| Path | Change |
|---|---|
| `studio/dsh-vivy-console/index.js` | `prepare()` now writes `providers.active` only. The template moved into an exported `backendConfig(port, dir)` so the producer's shape is testable in one place. |
| `studio/dsh-vivy-console/config.test.mjs` | New regression test: the config names only sections the kernel declares, `providers` carries `active` alone, no removed key is emitted, and the journal/roots still point at Studio scratch. |
| `studio/dsh-vivy-console/package.json`, `README.md` | Test file added to the shipped set; README documents the config producer, the strict decoder, and the new test command. |
| `docs/plans/provider-registry/MIGRATION.md` | §7.1 records this as a site the P1 inventory missed, and why it surfaced at runtime instead of at build/test time. |
| `data/studio-home/profiles/{vivy-studio,vivy-studio-next}/node_modules/dsh-vivy-console/` | Installed copies re-synced (Studio scratch, not committed). |

## Explicitly not done

- **No kernel change.** Strict decoding and the PROV-P3 removal hint are kept;
  tolerating the removed keys would undo the single-source-of-truth decision.
- **No `bundle_dir`/per-vendor fallback** anywhere in the console. A duplicated
  provider table is what caused the drift.
- **Not committed.** The shared working tree carries another lane's uncommitted
  work (`docs/TODO.md`, `internal/workflow/`, `internal/domain/workflow_test_support.go`,
  `docs/logs/2026-09-18-console-log-panel/`) and the `studio/` submodule sits on
  another lane's branch (`feat/hub-delete-source-autocommit`). Landing this
  change needs a decision about that lane, so it is left staged for review.