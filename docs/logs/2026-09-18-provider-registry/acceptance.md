# Acceptance

How a human can tell the provider registry works. Written for the whole
program; each phase adds the checks it makes possible.

## `PROV-P1` — provider metadata is part of the binary

1. **There is nothing to configure or ship beside the executable.**
   ```text
   rg -n "bundle_dir" --glob '!docs/**' .        # no hits
   ls fixtures                                    # does not exist
   ```
   A fresh checkout runs with a config that names only `providers.active`; no
   directory of provider YAML has to be present, and the working directory does
   not matter.

2. **The catalog is complete and validated before first use.**
   ```text
   go test ./internal/provider/ -run 'Embedded|Reconcile' -count=1 -v
   ```
   Expect 45 vendors / 47 endpoints / 168 models, and both gate directions
   (a data adapter that is not sealed, a sealed adapter with no endpoint) to
   fail closed.

3. **Editing the data is reviewed like code.** Change a number in
   `internal/provider/data/vendors.yaml`, rebuild, and the metadata anchors and
   the conformance digest change — the data cannot drift silently because it is
   inside the binary and inside the source hash.

4. **A bad data edit fails the build, loudly.** Add a vendor with an unknown
   key, a model id prefixed by its vendor name, a non-`http(s)` `base_url`, or a
   capability no adapter implements: startup reports every violation at once
   instead of the first.

5. **Behaviour is unchanged for the three shipped providers.** With a real key:
   ```text
   just run        # 127.0.0.1:8787
   ```
   DeepSeek still streams on `https://api.deepseek.com/chat/completions` with
   the same thinking controls; the Settings provider list still shows
   DeepSeek/OpenAI/Anthropic with the same models.

## `PROV-P2` — the sealed unit is the protocol, not the vendor

1. **An OpenAI reasoning model now asks for reasoning.**
   ```text
   go test ./internal/provider/ -run 'ReasoningEffort|DecideThinking' -count=1 -v
   ```
   The outbound body carries `reasoning_effort: high` and no DeepSeek
   `thinking` object; a non-reasoning model on the same endpoint is untouched.

2. **Two vendors can share one protocol without a synthetic vendor id.** The
   DeepSeek Anthropic endpoint and the Anthropic endpoint resolve to the same
   sealed `anthropic-messages` adapter while keeping their own `env_key` and
   `base_url`; a missing key names the right environment variable:
   ```text
   go test ./internal/provider/ -run 'CarriesVendorIdentity' -count=1 -v
   ```

3. **The deferred protocol is visible and not executable.**
   ```text
   go test ./internal/provider/ ./internal/modules/defaults/ -run 'Deferred|Adapters' -count=1 -v
   ```
   `openai-responses` is in the table, in the compiled Profile set and in the
   sealed manifest, and every attempt to construct through it fails closed.

4. **Data cannot widen the executable set.** A vendor entry naming an adapter
   outside the sealed three fails the startup gate; a Profile naming an unsealed
   family fails to compile.

## `PROV-P3` — configuration and credentials come from the data

1. **`config.yaml` names one thing: the vendor to fall back to.**
   ```text
   rg -n "env_key|default_model" config.example.yaml     # no hits under providers
   go test ./internal/config/ -run 'LoadValid|InvalidValuesRejected' -count=1 -v
   ```
   A document that still carries `deepseek:`/`openai:`/`anthropic:` fails to
   start with a message naming the removed key; deleting those three blocks (and
   keeping `active:`) is the whole migration.

2. **An existing `settings.yaml` selects exactly what it selected before.**
   ```text
   go test ./internal/app/ -run 'LegacyDocumentKeepsItsVendorAndEndpoint' -count=1 -v
   ```
   `provider: deepseek` with no `base_url` resolves to vendor `deepseek`,
   adapter `openai-completions`, endpoint `https://api.deepseek.com` and model
   `deepseek-flash` — the same pair the pre-migration build used. In the browser
   at `http://127.0.0.1:3015`, Settings still shows the same active provider and
   model, and the `settings/providers` payload is unchanged.

3. **Any vendor in the catalog works with no configuration block.**
   ```text
   MINIMAX_API_KEY=sk-... go test ./internal/app/ -run 'ThirdPartyVendor' -count=1 -v
   ```
   A MiniMax endpoint selected through the openai-compatible adapter is Ready
   from `MINIMAX_API_KEY` alone. Paste that key into Settings instead and
   `applySettingsEnv` writes it into `MINIMAX_API_KEY`, never into
   `OPENAI_API_KEY`.

4. **A vendor with no key is inactive, not probed.** Selecting a vendor whose
   environment variable is unset leaves the model not-ready and issues no
   network request during startup or status inspection.

## `PROV-P4` — the catalog is served, and the UI holds none of it

1. **Every embedded vendor is on screen, one row per endpoint.**
   ```text
   cd ui; pnpm dev        # then open http://127.0.0.1:3015/settings?tab=model
   ```
   The settings card lists 45 vendors / 47 endpoint rows, all loaded from
   `settings/providers`. A vendor with two protocols contributes two rows:
   `provider-row-deepseek-openai-completions` and
   `provider-row-deepseek-anthropic-messages` share the display name `DeepSeek`
   and print the adapter inline.

2. **A deferred protocol is visible, explained, and not selectable.**
   ```text
   rg -n "provider-row-openai-openai-responses" ui/src/components/settings/ModelSettingsCard.tsx
   ```
   The `openai-responses` row renders with `disabled` and a
   `DEFERRED-INDEFINITE` badge (`provider-capability-openai-openai-responses`),
   because this Generation seals the adapter but cannot construct it. Clicking
   it selects nothing.

3. **Selecting a model writes the adapter and the declared address.**
   ```text
   cat data/agent-home/settings.yaml     # after clicking a row and then a model
   ```
   The document reads `provider: openai-completions`, `base_url:
   https://api.deepseek.com`, `default_model: <model>` — the same
   `(adapter, base_url)` endpoint the resolver reads back, with no vendor data
   invented by the browser.

4. **The UI bundle cannot serve a provider the backend does not know.**
   ```text
   rg -n "openrouter|aihubmix|PROVIDER_CATALOG|gen-provider-catalog" ui/src ui/scripts
   ```
   No hits: the vendor array and its generator are gone. Searching the card for a
   vendor the data does not declare (`cherryin`) returns no row, while a declared
   one does, so the rows provably come from the payload.

5. **Both faces read the same list.** `settings/providers` is also what the TUI
   consumes (`sdk/tui/live/rpc.go`), so a vendor added to the embedded data shows
   up in the terminal switcher and the browser without a frontend change.

## `PROV-P5` — the program closed, and the promise checked end to end

1. **One command is the gate.**
   ```text
   just ci        # fmt-check, ui-ci, vet, test, headless-compile, plugin-ci
   ```
   Exit 0. `test` includes `sdk/internal` (~8.5 min) and
   `sdk/internal/conformance`, which recomputes the `internal` source hash and
   byte-compares the checked-in `conformance_results.json`, so this run is also
   the digest's proof.

2. **The whole promise in one experiment: data in, both faces out.**
   This program's claim is that a vendor is data. The check is to add one that
   no frontend code mentions, and watch both faces serve it — no rebuild of the
   UI, no Go table, no generated file.
   ```text
   # append a vendor to internal/provider/data/vendors.yaml (provenance is
   # required: source, entry, derived_at — D-025)
   - name: p5-canary
     display_name: P5 Canary
     env_key: P5_CANARY_API_KEY
     endpoints:
       - adapter: openai-completions
         base_url: https://p5-canary.invalid/v1
         default_model: p5-canary-model
         models:
           - id: p5-canary-model
             context_window: 8192
     provenance:
       source: p5-acceptance-check
       entry: p5-canary
       derived_at: "2026-09-18"
   ```
   Restart the control plane and reload `http://127.0.0.1:3015/settings?tab=model`
   (the Vite bundle is untouched — the page is the one already running). Observed
   on the final tree: the row count rises 94 → 96,
   `provider-row-p5-canary-openai-completions` renders with the display name
   `P5 Canary`, it is selectable, taking it sets `aria-pressed=true`, its model
   id `p5-canary-model` appears as a model button, and the search box finds it.
   Remove the block again and everything returns to 45 vendors / 94 rows — and
   the `internal` digest returns to `5e386f84…`, which is why the digest is
   trustworthy rather than a ritual.

3. **A half-added vendor is rejected, not half-served.** The same experiment
   without the `provenance` block fails startup:
   ```text
   composition failed: ... provenance must carry source, entry and derived_at (D-025)
   ```
   The process does not start; it never serves a catalog with a vendor the
   runtime cannot describe. Data is a build input with a schema, not a place to
   paste a URL.

4. **Nothing is left running or left behind.** `just ci` green, the lane's
   scratch (`.env` with the e2e locale, `data/`, the system-Chrome Playwright
   config) is not committed, and the branch is the deliverable: no push, no
   merge. `MIGRATION.md` §8.9 is the landing proposal for the owner's call.