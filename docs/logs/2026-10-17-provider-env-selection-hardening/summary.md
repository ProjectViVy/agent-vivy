# Provider environment/selection seam hardening

Date: 2026-10-17 · Branch: `fix/provider-env-selection-hardening` (from `origin/main`
`5715950d`) · Scope: kernel only (`internal/`), no UI source change.

## What this fixes

Four defects on the seam between the process environment, the settings document,
and the embedded provider data. Every one was reproduced on a pristine tree
before the change; the reproductions are recorded in `verification.md` and the
per-fix test names below.

| ID | Defect | Fix | Tests |
|---|---|---|---|
| A | A process with one shared credential name was frozen to a guessed vendor, and settings became read-only | `freezeFromEnv` decides or abstains; a warning names the ambiguity | `TestResolverSharedEnvKeyDoesNotGuessAVendor`, `TestResolverAmbiguousEnvHonoursVIVYProvider`, `TestResolverAmbiguousEnvHonoursConfiguredVendor`, `TestResolverAmbiguousEnvRejectsAmbiguousAdapterOverride`, `TestResolverSingleCredentialStillFreezesTheSession` |
| B1 | A user-registered provider's key was copied into an embedded vendor's environment variable when the entry's label matched a vendor name | the key write skips a registry-resolved selection | `TestApplySettingsEnvKeepsRegisteredProviderKeyInTheRegistry` |
| B2 | `VIVY_API_BASE` was never cleared, even when this process had published the value | publish/withdraw only this process's own value | `TestApplyBaseURLEnvWithdrawsOnlyItsOwnValue` |
| C | `default_model` could sit outside `models`, so a selection could adopt a model the Settings list does not render | reconcile the default into the list on read and on upsert | `TestProviderRegistryReconcilesDefaultModelIntoModels`, `TestUpsertProviderReconcilesDefaultModelIntoModels` |
| D | `/models` discovery had no entry bound and no id-length bound | `MaxModelListEntries = 2000` (inclusive) is an error; ids over 256 bytes are dropped | `TestModelListClientRejectsAnUnboundedCatalogue`, `TestModelListClientKeepsACatalogueAtTheLimit`, `TestModelListClientDropsOversizedModelIDs` |
| E | The `Bundle` field comment claimed "openai or anthropic"; `applySettingsEnv`'s comments claimed a startup overlay that does not exist | comments corrected to the code | — |

## Intentional contract change (Fix A)

Before: `freezeFromEnv` picked `hits[0]` — the alphabetically first embedded
vendor whose credential variable is set — so `OPENAI_API_KEY` alone froze the
process to `aihubmix` (a gateway) even when `config.providers.active` named
`deepseek`, and the settings document was then unwritable for that session.

After, the environment must identify exactly one candidate. Precedence:

1. `VIVY_PROVIDER` naming a candidate vendor;
2. `VIVY_PROVIDER` naming a sealed adapter that exactly one candidate speaks;
3. `config.providers.active` naming a candidate vendor;
4. exactly one candidate;
5. otherwise **no freeze**: the settings document stays in effect, the session
   stays writable, and one `logger.Warn` reports the variables and vendors
   involved (names only, never values).

This is deliberate: guessing which provider owns `OPENAI_API_KEY` can route a run
to an endpoint the operator did not choose, and freezing made that guess
unrecoverable from the UI. A single-credential startup (the ordinary case) is
unchanged.

## Explicitly not done

- Not making `env_key` unique in `vendors.yaml`: `openai` and `aihubmix`
  legitimately both read `OPENAI_API_KEY`, and the shared name is real data, not
  a data error to paper over.
- Not rejecting `default_model ∉ models` in `validateProviderEntries`: a legacy
  document that already has the mismatch would stop loading. The read
  reconciles instead.
- No environment fallback for an embedded vendor selected through Settings —
  that is a product decision tracked as **O2** in `docs/TODO.md` §0.1, not a
  bug fix.
- No UI source change. Fix A/B keep Settings editable and Fix C renders through
  the existing list, so the Face needs no new code.
- Observation O3 (not fixed, not a defect): `VIVY_PROVIDER` naming a vendor
  whose variable is unset falls back to the single-candidate rule exactly as
  before.
