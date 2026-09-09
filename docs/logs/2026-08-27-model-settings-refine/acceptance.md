# Acceptance guide — 2026-08-27 model-settings interaction refactor

Confirm from the user's perspective (development environment: `just dev` →
`http://127.0.0.1:3015` → Settings → Model, or hard-refresh directly in a Vivy Studio
session):

1. **Bottom form removed**: the Provider / default model / Base URL three-input form and
   「Save real settings」 button are gone; the page still has 「Selected models」 chips at
   the top, with the provider list on the left and model list on the right below.
2. **Provider-row edit button**: every custom-provider row has a persistent small pencil
   button on the right; clicking it opens the edit dialog, where 「Display name (alias)」 and
   「Base URL (address)」 plus model list/API Key can be changed; catalog (official) providers
   have no edit button.
3. **Two new model-list-header buttons**:
   - Refresh (literal label "Sync from official"): after clicking, the list area
     briefly shows the literal "Model list reloaded (static catalog snapshot)" feedback;
   - Add (+): an inline input appears at the top of the list; entering a model id and
     pressing Enter or the checkmark **applies it immediately** (becomes runtime config +
     enters the selected list); custom-provider models are also persisted in their registry
     and remain in that provider's list after refresh.
4. **API Key above the model list**: when a custom provider is selected, the key is editable
   (password box, saved to the registry on blur and applied with a model click, with the
   notice "stored only in local runtime data"); for a catalog provider, the input is disabled
   with the notice "catalog-provider keys are injected by the runtime environment".
5. **Read-only deployment**: the card displays a read-only notice at the top; switch-like
   operations are disabled, but registry editing/deletion and bookmark management remain
   available.

If items 1–4 differ from your three suggestions (especially the static-snapshot behavior of
"Sync from official"), report it and I will adjust it to your expectation.
