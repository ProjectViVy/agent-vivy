# UI-E2E-STALE — Realign e2e assertions with the current UI (including two related defect fixes)

## What changed

The 2026-09-01 e2e rerun found six assertions across four specs still targeting
the old UI. All were realigned with the current UI, and two real defects exposed
by the rerun were fixed as well.

### 1. Resynchronize stale assertions in four specs

- `ui/e2e/runtime.spec.ts` — the attachment entry changed from a button to
  `label[aria-label="附件"]` (embedded file input), and two locations now use
  `[aria-label="附件"]` for targeting. These are executable selectors, so the
  Chinese `aria-label` value remains unchanged. The Settings → Model tab
  assertion changed from the removed "The API key is managed only by the runtime
  environment" to the current card title "Selected model".
- `ui/e2e/welcome-wizard.spec.ts` — the model-step key hint now uses the current
  `welcome.secretNote` copy; after the completed-card deep link, the Model-tab
  assertion uses the "Selected model" title + "OpenAI current" provider row
  (the Model tab was redesigned as a provider registry UI with no Provider input
  field).
- `ui/e2e/language-setting.spec.ts` — the zh/EN language options are narrowed to
  the LanguagePicker's `role=group` (the group is named Language after the
  switch), avoiding the "en" substring in the top-bar model switcher's
  aria-label and the resulting strict-mode collision.
- `ui/e2e/model-refresh.spec.ts` — gpt-4o / gpt-4o-mini assertions changed from
  `getByText` to exact full-name button matching (model rows are buttons; the
  current-model span in the top bar makes `getByText` for gpt-4o-mini hit strict
  mode); after reload, the provider row is selected again before asserting.

### 2. Related defect fix 1: concurrent double-write from Model-tab Add model (UI)

`ModelSettingsCard.confirmAddModel` previously called `applyModelNow` without
waiting via `void saveProvider(...)` (and it has an internal `settings.save`), so
two RPCs concurrently read-modified-wrote the same settings document. It now
awaits the registry write before applying the model.

### 3. Related defect fix 2: missing wizard i18n keys and copy/form mismatch (UI)

The wizard's model step referenced `welcome.provider` /
`welcome.providerPlaceholder`, which did not exist in the zh/en dictionaries, so
the UI rendered raw keys. At the same time, `welcome.modelTitle/modelBody/introBody`
described a "display name + API Key" registry form, while the form actually
collected the Provider runtime bundle + Base URL + default model (D-010: the
wizard does not collect keys). The fix adds the two keys, restores the title to
"Configure model", aligns the copy with the form, and removes five unused dead
keys (displayName/apiKey/fieldsRequired, etc.).

### 4. Related defect fix 3 (kernel): concurrent settings-document reads/writes

See `docs/logs/2026-09-01-settings-save-race/` (separate commit): `settings.Save`
used a fixed `path+".tmp"` without a lock; during the full e2e run, the Add-model
concurrent double-write caused the temporary files to overwrite one another, and
Windows rename collided with concurrent reads, producing Access denied and later
`Load` failures surfaced as `providersError "internal error"`. The fix uses a
unique CreateTemp file plus `fileMu` serialization around the file-I/O window,
with a concurrency regression test.

## Explicitly not done

- Settings read-modify-write (Load→modify→Save) is still not fully serialized at
  the handler level; cross-request lost-update semantics remain last-writer-wins.
  Transactional behavior would require an API evolution such as
  `settings.Update(path, fn)` (tracked in TODO §0.1).
- The runtime.spec no-provider assertion (`Unable to connect; check provider
  configuration!`) was not stale—the earlier failure was solely caused by
  model-refresh polluting global settings, and the spec did not change that
  assertion.
- File-version history still awaits the O1..O6 user ruling (RB-1) and is outside
  this slice.
