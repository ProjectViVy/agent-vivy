# Welcome wizard: catalog-driven model step with write-only API key

Date: 2026-10-20
Lane: `feat/welcome-wizard-model` (`.worktrees/welcome-wizard-model`)
Base: the clean `feat/issue-91-base` line (`94e449ba`: vivy-code parity plus
the 2026-10-20 provider-catalog prune and provider-agentic migration), not the
discarded bloated `main`.

## What changed

The first-run wizard's model step is now the same selection the Settings → Model
face uses, instead of three free-text fields:

- **Vendor → protocol (endpoint) → model** are selects fed by the backend
  `settings/providers` catalog (`internal/provider/data/vendors.yaml` is still
  the only source of provider/model data). Only endpoint variants whose compiled
  Profile is executable are listed; an adapter without a Profile (the sealed set
  now has `openai-completions` and `anthropic-messages`) does not appear.
- The selected endpoint's address is displayed read-only. The free-text
  protocol-adapter and Base-URL inputs are gone, and the wizard never accepts a
  custom gateway (Settings → Model keeps that path).
- The select menus portal above the wizard overlay. The wizard overlay uses
  `z-[200]`; the shared `SelectContent` default is `z-50`, so the options were
  rendered in the DOM but behind the modal. The three wizard selects now pass
  `className="z-[250]"`, and the e2e spec clicks a real option to keep that
  stacking contract tested.
- **API Key** is a password input. A typed key is written through
  `settings/providers/upsert` with `api_key` (write-only); the value is never
  read back, logged, or sent through `settings/update`.
- Key policy: when the selected endpoint already has a key (`api_key_set` on a
  registry entry, or a legacy `Settings.ApiKey` overlay matching the current
  selection) an empty field keeps it. Otherwise a key is required unless the
  user explicitly checks "the runtime environment already provides the key".
  The backend does not expose env-var presence, so this is a deliberate opt-in
  checkbox rather than auto-detection.
- Prefill comes from the current settings document: stored provider/base-url
  exact match, legacy bundle-name fallback, then `config_provider`'s catalog
  default endpoint. A stored custom gateway (provider + base URL with no
  catalog match) pre-fills nothing, so the wizard cannot silently overwrite it.
- Read-only/frozen deployments render the existing notices, disable the fields,
  and advance without writing.
- Removed the last vendor-specific UI constant: `DEEPSEEK_PLATFORM_URL` and the
  "Open in browser" button are gone. Capturing that link as catalog data is
  filed as `UI-PROVIDER-CONSOLE-LINK` (open).

Files:

- `ui/src/components/layout/welcome-model.ts` (new pure projection layer)
- `ui/src/components/layout/WelcomeWizard.tsx`
- `ui/src/i18n/{zh,en}.ts` (`welcome.*` copy)
- `scripts/i18n-cross-face-contract.json` (Web face-specific key list: six
  removed keys retired, sixteen new keys classified)
- `ui/src/components/layout/welcome-model.test.ts` (new)
- `ui/src/components/layout/WelcomeWizard.test.tsx` (new)
- `ui/e2e/welcome-wizard.spec.ts` (rewritten for the new step and the default
  `en` locale)

## Scope

UI-only over existing control-plane RPCs (`settings/providers`,
`settings/providers/upsert`, `settings/update`). No backend, wire, schema,
migration, or dependency change. No hard-coded provider/endpoint/model list was
added; all displayed truth is the backend catalog.

Eino capability check: this change touches no agent loop, model adapter,
prompting, tool orchestration, streaming, or context management. The model
call path is untouched, so there was no Eino surface to inspect or reuse.

## Explicitly not done

- Custom provider creation inside the wizard (still Settings → Model only).
- A catalog data field for vendor console/documentation links.
- Folding the browser e2e suite into `just ci` (`CI-E2E-NOT-IN-GATE` remains
  open; the rewritten spec was run manually).
- Any live provider request; verification never uses a real key.
