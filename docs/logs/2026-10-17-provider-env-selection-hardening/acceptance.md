# Acceptance — provider environment/selection seam hardening

A human can confirm each item below without reading the diff. Command-line
checks assume the repository root; `SESSION` means the running kernel.

## 1. An environment that names no single provider no longer guesses

Start the kernel with one shared credential name and no explicit choice:

```
OPENAI_API_KEY=sk-…  (no VIVY_PROVIDER, config.providers.active: deepseek)
```

Expected: startup logs one warning —

> environment provider selection is ambiguous … OPENAI_API_KEY are set for
> several providers (aihubmix, openai); set config.providers.active or
> VIVY_PROVIDER to choose one, so the settings document stays in effect

— and the kernel starts normally. Settings stay editable for that session, and
the run uses the selection from `settings.yaml`. Before the change the same
environment silently froze the session to `aihubmix` and Settings could no
longer correct it.

Choosing explicitly removes the warning and freezes to the named vendor:

```
OPENAI_API_KEY=sk-… VIVY_PROVIDER=aihubmix   → starts with no warning
```

A single credential (the ordinary case) behaves exactly as before:
`DEEPSEEK_API_KEY=sk-…` alone freezes to `deepseek`.

## 2. A registered gateway's key stays in the registry

In `Settings → 模型`, register a provider whose display name happens to match an
embedded vendor — for example display name `openai`, address
`https://gateway.example/v1`, key `sk-my-gateway` — and make it active. Save,
then check the process environment of the running kernel: `OPENAI_API_KEY` is
**not** set to `sk-my-gateway`, and the run still authenticates through the
registry entry. Before the change the gateway key was written into
`OPENAI_API_KEY`, where the embedded `openai` vendor would have picked it up.

## 3. An operator's `VIVY_API_BASE` survives a settings write

- Export `VIVY_API_BASE=https://operator.example/v1` in the environment, clear
  the base URL in Settings, save: the variable keeps the operator's value.
- Conversely, set a base URL in Settings (it appears in the environment), then
  clear it and save: the variable is removed, because this process published it.

## 4. `Settings → 模型` can always show the active model

Register a provider with `default_model: ghost-model` and `models: [real-model]`
(an older document could contain exactly this). Reopen the page: `ghost-model`
is listed alongside `real-model`, and it is the selected default — no state
where the active model is invisible in the list.

## 5. A huge upstream catalogue fails loudly

Point a provider entry at an upstream `/models` returning more than 2000 ids and
press refresh: the refresh reports an error naming the limit, and the stored
list is left untouched. Returning 2000 ids succeeds; a single id longer than
256 bytes is ignored instead of being stored.

## Evidence and limits

Tests: `TestResolverSharedEnvKeyDoesNotGuessAVendor`,
`TestResolverAmbiguousEnvHonoursVIVYProvider`,
`TestResolverAmbiguousEnvHonoursConfiguredVendor`,
`TestResolverAmbiguousEnvRejectsAmbiguousAdapterOverride`,
`TestResolverSingleCredentialStillFreezesTheSession`,
`TestApplySettingsEnvKeepsRegisteredProviderKeyInTheRegistry`,
`TestApplyBaseURLEnvWithdrawsOnlyItsOwnValue`,
`TestProviderRegistryReconcilesDefaultModelIntoModels`,
`TestUpsertProviderReconcilesDefaultModelIntoModels`,
`TestModelListClientRejectsAnUnboundedCatalogue`,
`TestModelListClientKeepsACatalogueAtTheLimit`,
`TestModelListClientDropsOversizedModelIDs`.

Not claimed by this record: `just ci` as a whole (blocked here by the UI
production build — see `verification.md`), a browser pass over the Settings
page, and any live-provider run. Items 2–5 are verified by focused unit tests
and the code path they cover; items 1 and 3 were additionally exercised against
the real binary / real functions as recorded in `verification.md`.
