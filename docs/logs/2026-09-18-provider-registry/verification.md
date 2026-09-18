# Verification

Commands were run from the lane `.worktrees/provider-sot` (branch
`feat/provider-registry`) on 2026-09-18, Go 1.26.4 / pnpm 10.33.0.

## `PROV-P1`

| Command | Result |
|---|---|
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 (after the test sweep; the first run reported every broken bundle call site) |
| `go test ./internal/provider/ -count=1` | `ok` — data validation (18 rejection cases), the gate in both directions, the embedded catalog shape (45 vendors / 47 endpoints / 168 models), metadata anchors, the DeepSeek default chain, the deferred `openai-responses` endpoint, and the unchanged request bodies |
| `go test ./... -count=1` | every package `ok` except `sdk/internal/conformance`, which failed **only** on the stale `internal` source digest (expected derived data, not a defect); see below |
| digest refresh + `go test ./sdk/internal/conformance/ -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1` | `ok` in 182.9s |
| `just fmt-check` | exit 0 (after `gofmt -w` on the 19 changed Go files) |
| `just ui-ci` | exit 0 (typecheck, lint, unit tests, i18n completeness, 6.7s build) |
| `rg -n "bundle_dir\|BundleDir" --glob '!docs/**'` | no hits (the only remaining mention is `schemas/README.md`'s note that the setting no longer exists) |

Not run in this phase, with reasons:

- `just ci` as a whole: scheduled once at `PROV-P5` per the program plan; the
  individual recipes that this phase can affect were run above
  (`fmt-check`, `ui-ci`, and the Go build/vet/test trio).
- `headless-compile` and `plugin-ci`: no plugin, assembly, or embedded-UI build
  input changed. `PROV-P5` runs them.
- Browser smoke at `http://127.0.0.1:3015`: this phase deliberately changes no
  browser-observable payload (the `settings/providers` response is byte-identical
  and the UI still reads its generated catalog until `PROV-P4`).

## Failures found and their resolution

1. **The startup gate rejected real upstream identifiers.** `302ai` and
   `302AI_API_KEY` do not match `^[a-z][a-z0-9_-]*$` / `^[A-Z][A-Z0-9_]*$`.
   Resolved by relaxing both rules to allow a leading digit — in the data
   validator and in `config.ValidEnvKey` / the settings `auth_env` pattern, which
   the credential allowlist uses on `Profile.SecretRefs`. Recorded in
   `MIGRATION.md` §7.2.
2. **Nine call sites the plan's file inventory did not list** broke the build or
   the tests (studiolifecycle defaults, the eval runner in `app.go`, the `sdk`
   module's eval users, `codeface`, five `internal/app` test configs, the
   Playwright child config, and a config test asserting the Dockerfile *must*
   copy fixtures). All fixed; listed in `MIGRATION.md` §7.1.
3. **The conformance digest moved twice.** It is derived from every file under
   `internal/`, so it changed once when the data and adapter files landed and
   again when `gofmt` rewrote them. `sdk/internal/assembly/conformance_results.json`
   now carries `087b41ac…` and the producer gate passes.

   The phase spec defers this refresh to `PROV-P5`. It was done here instead,
   because leaving it stale would make every intermediate commit of this branch
   fail `go test ./sdk/internal/conformance/`, and the value is mechanical to
   recompute (`HashSourceTree("internal", "")`). `PROV-P2`..`PROV-P5` must
   refresh it again whenever they touch a file under `internal/`; `PROV-P5`
   still owns the final value.

## Failure-first evidence

The data tests were written before the loader was wired, and the boundary
rewrite was driven to green by `go vet ./...` reporting the compile sites one
package at a time. The vendor/endpoint parser's rejection table (unknown key,
missing `env_key`, unsealed adapter, relative `base_url`, `default_model`
outside `models`, vendor-prefixed and duplicate model ids, unknown capability,
negative metadata, duplicate vendor and duplicate endpoint identity, joined
multi-error output) is the contract for future data edits.