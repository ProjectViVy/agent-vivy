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

## `PROV-P3`

| Command | Result |
|---|---|
| `go build ./...` | exit 0 (after the config shrink, `internal/app` failed on the three deleted `cfg.Providers.*` reads and the two deleted helpers; both removed) |
| `go vet ./...` | exit 0 (vet named the test call sites: `EndpointForVendor`'s new arity, `config.Provider`, `Catalog.AdapterFamily`) |
| `go test ./internal/config ./internal/provider ./internal/app/settings ./internal/app -count=1` | `ok` |
| `go test ./internal/... -count=1` | every package `ok` (after the eval isolator stopped writing per-vendor blocks and the registry uniqueness key was normalized) |
| `go test ./... -count=1` (`just test`, `-timeout 20m`) | see the job log below |
| digest refresh (`go run ./sdk/internal/cmd/source-hash internal ""`) | `13f4ee33…`, written to the five `internal`-rooted entries of `sdk/internal/assembly/conformance_results.json` |
| `go test ./sdk/internal/conformance/ -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1` | see the job log below |
| `just fmt-check` | exit 0 |

### Failure-first evidence

- `TestResolveStoredSelectionChain` pins all five rungs of the resolution chain
  (declared address, legacy vendor, adapter alone, legacy vendor with an
  undeclared gateway, address no vendor declares), and
  `TestResolverLegacyDocumentKeepsItsVendorAndEndpoint` is the phase's headline
  case: `provider: deepseek` with no address still resolves to vendor `deepseek`,
  adapter `openai-completions`, model `deepseek-flash`.
- `TestResolverThirdPartyVendorNeedsNoConfigurationBlock` is the `MINIMAX_API_KEY`
  RED: before P3 the key was outside the allowlist and there was no per-vendor
  block to add it to, so a MiniMax selection could never be Ready. It now is, and
  the same test asserts the unset-key case stays not-ready.
- `TestApplySettingsEnvWritesTheVendorsOwnVariable` is the third-way-switch RED:
  a MiniMax key selected through `openai-completions` used to be written into
  `OPENAI_API_KEY`; the test asserts the vendor's own variable receives it and
  `OPENAI_API_KEY` is byte-identical afterwards.
- `TestProviderRegistryUniquenessIgnoresVocabularySpelling` is the normalization
  RED at the registry boundary: `bundle: openai` and `bundle: openai-completions`
  on one address are one endpoint identity and must collide.
- `TestUnknownProviderValueIsRejectedNotInvented` and
  `TestResolverIsNonFatalForAnUnusableStoredValue` are the two halves of rule 3
  (rejected on write, non-fatal on read).
- `TestCatalogVendorForEndpointIdentifiesTheVendor` covers the address→vendor
  direction the resolution chain depends on, including the "user gateway" case
  that must not match.

## `PROV-P2`

| Command | Result |
|---|---|
| `go build ./...` / `go vet ./...` | exit 0 (vet found the four bundle-era helpers the boundary rewrite had to re-point, and then the test scaffolding) |
| `go test ./internal/provider ./internal/modelhost ./internal/modules/defaults ./internal/app -count=1` | `ok` |
| `go test ./internal/... -count=1` | every package `ok` |
| `go test ./... -count=1` | every package `ok` except `sdk/internal`, which hit Go's **default 10-minute per-package timeout** (601.5s); see below |
| `go run ./sdk/internal/cmd/generate-default --repo . --output internal/generated/assembly/zz_default.go` (twice, second to a temp path) | one line changed (the sealed `ProviderProfiles` list); both runs byte-identical, SHA-256 `156D7436…` |
| digest refresh (`HashSourceTree(internal, "")`) + `go test ./sdk/internal/conformance/ -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1` | `978e0d42…` written to the five entries; re-run `ok` in 375.0s |
| `go test ./sdk/internal -count=1 -timeout 25m` | `ok`, 768.6s, every test listed pass |
| `just fmt-check` | exit 0 |

### The one non-passing result, and why it is not a defect

`sdk/internal` is the SDK's real pack/eval suite: ~20 tests each build a
temporary repository module end to end. Measured per test, the heavy ones are
119.6s, 89.3s, 59.9s, 54.8s, 50.4s, 48.7s, 47.1s, 44.6s, … — a 13 minute total on
this host, which is why `go test ./...` with Go's default 10-minute per-package
timeout kills it. The product gate does not use that default: `just test` passes
`-timeout 20m` for exactly this reason (the recipe already documents it for
`internal/runtime`; `sdk/internal` is the second such package).

To rule out a P2 regression, the same test was measured on both revisions:

| Revision | `TestPackAndInspectSealUIAssemblyIdentity` |
|---|---|
| `PROV-P1` (stashed working tree, `3025405`) | 39.50s |
| `PROV-P2` | 37.78s |

So the phase is not slower; the earlier 48.7s reading was taken while another
test job was running.

Not run in this phase, with reasons:

- `just ci` as a whole: scheduled once at `PROV-P5`.
- `just ui-ci`: no UI file changed. P2 deliberately keeps the
  `settings/providers` payload byte-identical (`app.transitionalVendorNames`),
  so there is nothing for the UI gate to see until `PROV-P3`/`PROV-P4`.
- `headless-compile` and `plugin-ci`: `PROV-P5`.
- Browser smoke at `http://127.0.0.1:3015`: the payload is unchanged, and the
  provider list still shows three vendors. `PROV-P4` owns the browser check.

### Failure-first evidence

`decideThinking` was extracted as a pure function specifically so the rule table
could be asserted without constructing a model; the table has 13 cases including
the deferred and unknown adapters. The two API-level tests then assert the
outbound body, which is what makes the OpenAI reasoning fix a behaviour change
rather than a refactor: `reasoning_effort: high` with no `thinking` object on a
non-DeepSeek endpoint, both fields on DeepSeek, and nothing on a model without
the metadata.

## Failure-first evidence

The data tests were written before the loader was wired, and the boundary
rewrite was driven to green by `go vet ./...` reporting the compile sites one
package at a time. The vendor/endpoint parser's rejection table (unknown key,
missing `env_key`, unsealed adapter, relative `base_url`, `default_model`
outside `models`, vendor-prefixed and duplicate model ids, unknown capability,
negative metadata, duplicate vendor and duplicate endpoint identity, joined
multi-error output) is the contract for future data edits.