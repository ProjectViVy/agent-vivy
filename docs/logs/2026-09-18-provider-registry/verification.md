# Verification

Commands were run from the lane `.worktrees/provider-sot` (branch
`feat/provider-registry`) on 2026-09-18, Go 1.26.4 / pnpm 10.33.0.

## `PROV-P5`

| Command | Result |
|---|---|
| `just ci` (once, on the final tree) | **exit 0** — `fmt-check`, `ui-ci`, `vet`, `test`, `headless-compile`, `plugin-ci` all passed. The long poles: `sdk/internal` 506.8s, `sdk/internal/conformance` 195.1s, `internal/rpc` 115.6s, `sdk/internal/assembly` 20.2s, `internal/studiocore` 16.9s, `sdk/tui/live` 1.4s, `internal/provider` 3.7s |
| digest verification inside that run | `sdk/internal/conformance` byte-compares the checked-in `conformance_results.json` against the live tree, so `just ci` is also the proof that `5e386f84…` is the correct final value |
| browser smoke re-run on the committed tree (`http://127.0.0.1:3015`) | 14/14 checks; `settings.yaml` ended at `provider: openai-completions`, `default_model: deepseek-chat`, `base_url: https://api.deepseek.com` |
| zero-data greps | `rg "gen-provider-catalog|PROVIDER_CATALOG|agent-diva-source" ui/src ui/scripts ui/vitest.config.ts ui/package.json` → no hits; the only provider-shaped strings left under `ui/src` are the read-side alias table, the demo fixtures, and placeholder copy (`summary.md` deviations) |
| canary experiment: add a vendor to the data only, restart the control plane, reload the untouched Vite bundle | 9/9 checks — row count 94 → 96, `provider-row-p5-canary-openai-completions` present with display name `P5 Canary`, selectable, `aria-pressed=true` after the click, its `p5-canary-model` offered as a model button, search finds it, no page errors |
| canary without its `provenance` block (the fail-closed half) | startup aborts: `composition failed: ... provenance must carry source, entry and derived_at (D-025)` — no catalog is served |
| revert the canary | tree clean, digest back to `5e386f84…` (bit-identical content → identical hash), and the 14-check smoke passes again at 94 rows / 45 vendors |
| `git merge --ff-only feat/provider-registry` in the root checkout | fast-forward from `ff8a47d`; `main` and the branch share one tree, so the CI evidence above covers what landed |
| `git push origin main` | `origin/main` advanced `fe60b18` → `ff8a47d` → the branch tip; the push also published eight previously-unpushed `main` commits (the DeepSeek-default change and seven docs/scheduling commits), which is inherent to publishing the branch |
| docs-only tail after `8c62886` | `git diff --stat 8c62886..HEAD` lists `AGENTS.md` and `docs/` only, so no code path changed after the `just ci` run and no re-run was required |
| CI on the pushed tip (run `35361729156`, `77be9c5`) | `ui ci` passed (1m35s) and `backend ci` passed (19m46s — Go tests, conformance suite and the `internal` digest on the pushed tree). `full UI browser smoke` failed with `Error: Timed out waiting 30000ms from config.webServer.`, and the `just ci` job failed in 4s only because it asserts `BROWSER_RESULT = success`. Pre-existing and unrelated: the same signature is on the previous `main` tip `fe60b18` (run `34944037203`). Mechanism and fix direction: `CI-BROWSER-SMOKE-WEBSERVER` in `docs/TODO.md` §0.1 |

Docs-only commits after this run (this log, the board row, the plan status table)
do not re-enter the build: nothing under `internal/` changes, so the digest and
the CI evidence stand.

## `PROV-P4`

| Command | Result |
|---|---|
| `go vet ./internal/... ./sdk/... ./cmd/...` | exit 0 |
| `go test ./internal/rpc ./internal/app ./internal/provider ./internal/config ./internal/eval -count=1 -timeout 20m` | all `ok` (`rpc` 105.6s, `app` 73.5s, `provider` 0.25s, `config` 1.3s, `eval` 41.1s) |
| `go test ./sdk/tui/live -count=1` | `ok` |
| `just headless-compile` | exit 0 |
| `just plugin-ci` | exit 0 |
| digest refresh (`go run ./sdk/internal/cmd/source-hash <lane>/internal ""`) | `5e386f84…`, written to the five `internal`-rooted `sourceSha256` entries of `sdk/internal/assembly/conformance_results.json` (the value computed before the last `internal/app/app.go` edit, `0d24ebe4…`, was stale — see `MIGRATION.md` §8.7) |
| `cd ui; pnpm typecheck` | exit 0 (38 files) |
| `cd ui; pnpm test` | exit 0 — 38 files / 332 tests, including the new `ModelSettingsCard.test.tsx` (5 mounted cases) and the rewritten `provider-catalog` (11) / `custom-providers` (18) / `saved-models` (19) specs |
| `cd ui; node ../scripts/check-i18n-completeness.js` | PASS — en=1409 / zh=1409 keys, 138 placeholders, runtime copy audit clean (two locale keys were dropped with the contract entries that described them) |
| `cd ui; node ../scripts/check-i18n-cross-face.js` | PASS — 13 shared semantic units |
| `cd ui; pnpm build` | exit 0 |
| `cd ui; pnpm e2e` | not runnable as written here: Playwright 1.62.1 wants Chromium revision 1234, this workstation has neither it nor CDN access to fetch it. The suite was run with `playwright test --config playwright.system-chrome.config.ts` (a scratch config pointing `launchOptions.executablePath` at the system Chrome; never committed and deleted before the commit) with a lane-local `.env` (`VIVY_DEFAULT_LOCALE=zh`, gitignored): **14 passed, 10 failed, 2 skipped** — including both specs this phase changed (`model-refresh.spec.ts`, `welcome-wizard.spec.ts`). The 10 failures are outside this phase's file set (see below). |
| browser smoke at `http://127.0.0.1:3015` | 14/14 checks, see below |

Not run in this phase, with reasons:

- `just ci` as a whole: scheduled once at `PROV-P5` per the program plan.
- `just test` (the full Go sweep): the packages this phase changes were run
  directly above; the full sweep costs ~13 minutes and `PROV-P5` runs it inside
  `just ci`.

### The four failures this phase produced, and what they changed

1. **The face contract had to move with the UI.** `ui-sdk-face-compat` and
   `ui-build-provenance` assert `sdk/ui/src/module.ts` and the store's state
   *exactly*, in both directions, so the UI could not change without the
   published contract. `FaceProviderEntry.bundle` was literally
   `"openai" | "anthropic" | "deepseek"` and `FaceStoreState` had no `catalog`.
   Two non-obvious constraints came out of fixing it: `ui/node_modules/@vivy/ui-sdk`
   is a **hard-linked copy** that pnpm materializes from `sdk/ui`, so an edit to
   `module.ts` is invisible to `tsc` until `pnpm install --frozen-lockfile` runs
   in `ui/`; and `FaceStoreState` must stay *mutually* assignable with the
   store's own state type, because zustand's `subscribe` is a property with call
   signatures rather than a method, so its listener parameter is checked
   contravariantly. `catalog` and `FaceProviderCatalogEntry.endpoints` are
   therefore mutable arrays and `state` is the exact union, not `string`.
2. **The e2e suite still wrote the config shape PROV-P3 removed.**
   `ui/e2e/global-setup.ts` wrote `providers.deepseek.{env_key,default_model}`;
   strict decoding rejects that field now, so the suite's own server could not
   start. The e2e suite is not part of `just ci`, which is why P3's gate did not
   catch it. The block is gone and the config default is now the catalog's
   DeepSeek `default_model`, so the wizard's prefill assertion still reads
   `deepseek-flash`.
3. **The welcome wizard still hard-coded a provider.** `SUGGESTED_DEFAULTS =
   {provider: 'deepseek', model: 'deepseek-flash', baseUrl: ''}` and a free-text
   provider field were the last provider data in the UI, and the wizard wrote the
   legacy bundle name back on every first run. The prefill now comes only from
   the backend's settings document and the write normalizes to the sealed
   adapter; the copy and its two locale strings say "protocol adapter".
4. **Two locale keys became dead** (`settingsModel.bundleDeepseek`,
   `settingsModel.moreProviders`) once the write side dropped DeepSeek bundles
   and the fold was removed, and the dialog label `settingsModel.bundle`
   ("Runtime bundle") no longer described its adapter values. All three are gone
   from `scripts/i18n-cross-face-contract.json` and both locales; the label is
   now `settingsModel.adapter`. The i18n gates stay green at 1409 keys each.

### Real-path browser smoke (`http://127.0.0.1:3015`, split Vite + `go run ./cmd/vivy`)

Unlike P1–P3, this phase has browser-observable changes, so the smoke drives a
real browser (Playwright's Chromium against the Vite server on `:3015`, whose
`/rpc` proxy talks to the control plane on `:8787`) and asserts the DOM rather
than a screenshot. Scratch config and data directory under the OS temp dir
(`server.addr: 127.0.0.1:8787`, `providers.active: deepseek`), a `settings.yaml`
holding the pre-migration shape `provider: deepseek`.

| Check | Observed |
|---|---|
| catalog rows render | 94 `provider-row-*` elements (47 rows × row + name) |
| two DeepSeek protocol rows | `provider-row-deepseek-openai-completions` and `-anthropic-messages` both present, sharing the name `DeepSeek`, 70 buttons of which 9 name DeepSeek |
| deferred endpoint | `provider-row-openai-openai-responses` present, `disabled`, carrying the `DEFERRED-INDEFINITE` badge |
| selection round-trip | clicking the `deepseek-chat` model button left the row `aria-pressed=true`, and `settings.yaml` then read `provider: openai-completions`, `base_url: https://api.deepseek.com`, `default_model: deepseek-chat` |
| rows come from the backend | searching the card for `cherryin` (a vendor the old frontend array carried and the data does not declare) returns 0 rows, while `302` returns 2 — so the list is the payload, not a bundled array |
| D-010 | no `sk-` material, no uncaught page error, no failing request |

The page rendered in English because a fresh scratch document carries no
language preference; the app's copy follows its own setting, so the smoke
matches the search box in either language.

### The e2e suite: two environment preconditions, and why ten specs still fail

The suite is not part of `just ci`, and it needed two setup steps that the
repository does not record:

1. **Locale.** the app's language comes from the *backend* developer locale
   (`<launch root>/.env`, `VIVY_DEFAULT_LOCALE`), not from the browser context's
   `locale`, because `ui/src/i18n/index.ts` only reads `vivy.language` from
   localStorage and otherwise defaults to English. Without a lane-local `.env`
   with `VIVY_DEFAULT_LOCALE=zh`, the first run scored 2 passed / 22 failed and
   every failure was a Chinese label the page never rendered — the app was
   correct, the environment was not.
2. **Browser.** the pinned revision cannot be downloaded here, so the run used
   the system Chrome through a scratch config.

With both, 14 passed / 10 failed / 2 skipped, and the two specs this phase
changed pass. The ten failures are `approvals-nav`, `chat-act`,
`compaction-setting` (×2), `files-panel`, `lifecycle-readonly`,
`mcp-settings:100`, `runtime`, `thinking-gate`, and `trajectory-panel`. They are
**not** this phase's regressions, and the evidence is the change set itself:
their failures are Playwright strict-mode ambiguity on `新建会话` (three matching
buttons), a missing `聊天` link, an `en`-only `Compaction history` expectation in
a `zh` run, and a `Generations` tab that no longer exists — assertions against
`layout/` navigation and session controls, the compaction card, the MCP page and
the lifecycle page, none of which this phase's diff touches (`git diff --stat`
lists only `MaskAndModelSwitcher`, `WelcomeWizard`, `GenerationParamsCard`,
`ModelSettingsCard`, the provider-catalog/custom-providers/saved-models modules,
`lib/api.ts`, `lib/store.ts`, and the two locales). Suite drift is recorded on the
board as `CI-E2E-NOT-IN-GATE` rather than fixed here.

### Failure-first evidence

- `TestProvidersCatalogServesEmbeddedData` walks the payload: 45 vendors, 47
  endpoints, exactly one row per vendor, the deferred `openai-responses`
  endpoint with `executable:false` and `state:DEFERRED-INDEFINITE`, and a
  payload-wide audit that no `api_key`, `env_key`, `sk-` literal or credential
  variable name can appear.
- `TestSelectModelUsesCatalogAndPreservesUnrelatedSettings` selects a model
  through the catalog and asserts the unrelated `network_search` preference and
  the execute ceiling survive a `settings/update` that replaces the document.
- `ModelSettingsCard.test.tsx` is the DOM-level red list: no rows while the
  catalog is idle or loading, two DeepSeek rows distinguishable by their inline
  adapter, the deferred row `disabled` with its badge next to an enabled sibling,
  a catalog endpoint beating a same-endpoint registry clone, `catalog-*` overlay
  rows never rendering, and a row+model click writing
  `{provider, base_url, default_model}`.
- `provider-catalog.test.ts` covers the read-side alias table, the
  `(vendor, adapter)` row identity, `EXECUTABLE_STATES`, and the address-less
  fallback that mirrors `NormalizeProviderSelection(...).LegacyVendor`.

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
| `go test ./... -count=1` (`just test`, `-timeout 20m`) | first run: one failure, `internal/provider`'s source-level secret audit (`TestSecretEnvReadsStayOutOfProvider`) flagged `os.Getenv` in the new **test** file; fixed (see below) and the second full run exited 0, including `sdk/internal` 780.2s and `sdk/internal/conformance` 209.9s |
| digest refresh (`go run ./sdk/internal/cmd/source-hash internal ""`) | `13f4ee33…` after the first sweep, then `a0811ff1…` after the audit-test edit — both written to the five `internal`-rooted entries of `sdk/internal/assembly/conformance_results.json` |
| `go test ./sdk/internal/conformance/ -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1` | `ok` in the full sweep (`sdk/internal/conformance` 209.9s) with `a0811ff1…` checked in |
| `just fmt-check` | exit 0 |
| real-path smoke (below) | runs A, B, C as recorded |

### The one failure, and what it changed

`TestSecretEnvReadsStayOutOfProvider` (`internal/provider/secret_audit_test.go`) greps
every `.go` file under `cmd/` and `internal/` for `os.Getenv("<secret-shaped
name>")` and exempts only `internal/app/model.go`. The new
`internal/app/provider_selection_test.go` reads `OPENAI_API_KEY` and
`MINIMAX_API_KEY` to assert that `applySettingsEnv` wrote the key into the right
variable, which the audit flagged.

The fix keeps the audit's production guarantee and makes it stronger, rather
than evading it: test files are exempt (a test must be able to arrange and
observe the environment; the walk still covers every production source), and the
pattern now also matches `os.LookupEnv` and `os.Setenv`, because PROV-P3 made
`applySettingsEnv` write a **data-derived** variable name. The audit passes with
the wider rule, which is the evidence that no production source outside
`internal/app/model.go` reads or writes a literal secret-shaped name.

### Real-path smoke (`go run ./cmd/vivy`, legacy and new-style documents)

A scratch config and data directory under the OS temp dir (never in the repo),
`server.addr: 127.0.0.1:3015`, `providers.active: deepseek`, and the browser's
own transport: `GET /rpc/bootstrap`, then `initialize` and `settings/providers`
over the `/rpc` WebSocket. No browser was driven: this session has no browser
automation tool, and no browser-observable payload changes in P3 (the
`settings/providers` response is byte-identical; `PROV-P4` rebuilds the UI).

| Run | `settings.yaml` | Observed |
|---|---|---|
| A | legacy: `provider: deepseek`, `default_model: deepseek-flash`, no address | `active_provider=deepseek`, `active_model=deepseek-flash`, `active_base_url=''`, `config_provider=deepseek`, `config_model=deepseek-flash`, `profiles: anthropic-messages:COMPILED, openai-completions:UNCONFIGURED, openai-responses:DEFERRED-INDEFINITE`, `bundles=3` (deepseek/gpt-4o/claude) |
| B | A plus `api_key: sk-legacy-overlay` | identical, except `openai-completions:READY` — the legacy value resolved to adapter `openai-completions` and the legacy overlay reached it |
| C | new-style: `provider: openai-completions`, `base_url: https://api.minimaxi.com/v1`, one registry entry with `bundle: openai-completions` and a key | `active_provider=minimax`, `active_model=MiniMax-M2.1`, `active_base_url='https://api.minimaxi.com/v1'`, `openai-completions:READY`, `entries: custom-1/openai-completions@https://api.minimaxi.com/v1 key=True`, while `config_provider` stays `deepseek` |

Run C is the phase's product claim on the real binary: a document that names no
vendor resolves to the third-party vendor the address belongs to, with no
per-vendor config block anywhere.

Startup logged `settings overlay applied provider=deepseek model=deepseek-flash`
(run A) and `provider=deepseek model=MiniMax-M2.1` (run C) — the first field is
`config.providers.active`, the second is the resolved model.

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