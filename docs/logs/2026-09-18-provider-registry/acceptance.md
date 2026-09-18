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

## Later phases

- `PROV-P4`: the browser shows all 45 vendors at `http://127.0.0.1:3015` with no
  provider data in the UI bundle.
- `PROV-P5`: `just ci` green and the board closed.