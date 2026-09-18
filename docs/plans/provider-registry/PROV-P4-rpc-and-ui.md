# PROV-P4 — Catalog RPC and Zero-Data UI

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task.

**Goal:** The frontend holds no provider data of its own. The vendor catalog,
endpoint variants, adapter states, and model lists all arrive from the backend;
the repository no longer depends on a gitignored external checkout to regenerate
them.

**Architecture:** The existing `settings/providers` RPC already returns
`entries`, `bundles`, and `profiles`; `bundles` is currently declared on the wire
type and consumed by nobody (`ui/src/lib/api.ts:349`). It becomes the catalog
payload. The UI keeps only projection helpers and gains a loading state.

**Tech Stack:** Go, JSON-RPC over WebSocket, React/TypeScript/Vite, Vitest,
`just ci`.

**Spec:** `docs/plans/provider-registry/DESIGN.md` §4, §7;
`docs/plans/provider-registry/MIGRATION.md` §2, §4.

**Depends on:** `PROV-P1`..`PROV-P3`.

---

## Global constraints

- The UI must have no bundled provider/vendor/model data left. Grep proof at the
  end of the phase: no vendor name literal array remains under `ui/src`.
- Deleting `ui/agent-diva-source/` is intended. It is gitignored
  (`ui/.gitignore:31`) and untracked, so `git status` will not show it; verify by
  filesystem listing.
- Secrets stay out of the wire: `api_key_set` booleans only (existing rule).
- A `DEFERRED-INDEFINITE` adapter is visible and not selectable. The machinery
  already exists (`ui/src/components/settings/provider-catalog.ts:35-45`).
- One focused commit.

---

### Task 1: The catalog payload

**Files:**

- Modify: `internal/rpc/control.go:110-117,4443-4493`
- Modify: `internal/app/app.go:866-868`
- Modify: `ui/src/lib/api.ts:13-14,115,327-375`
- Modify: `internal/rpc/control_test.go`

**Interfaces:**

```go
// catalogEntryResult is one vendor with its endpoint variants. No secret value
// crosses this boundary.
type catalogEntryResult struct {
    Vendor      string                 `json:"vendor"`
    DisplayName string                 `json:"display_name"`
    Endpoints   []catalogEndpointResult `json:"endpoints"`
}

type catalogEndpointResult struct {
    Adapter      string   `json:"adapter"`
    BaseURL      string   `json:"base_url"`
    DefaultModel string   `json:"default_model"`
    Models       []string `json:"models"`
    Executable   bool     `json:"executable"`
    State        string   `json:"state"`   // the adapter's capability state
}
```

- [ ] RED: the RPC returns every embedded vendor exactly once, with all of its
  endpoint variants, and no `env_key` value (name only is acceptable; the current
  wire carries no env key at all, so adding the *name* is optional — decide in the
  task and assert it either way).
- [ ] RED: `openai-responses` appears with `executable: false` and
  `state: "DEFERRED-INDEFINITE"`.
- [ ] RED: the payload contains no `api_key` field and no credential value.
- [ ] Replace `Deps.ProviderBundles []provider.Bundle` with the catalog snapshot.
- [ ] Decide whether the catalog rides on `settings/providers` or on
  `settings/get`; prefer `settings/providers` (it already returns entries and
  profiles, and the UI already loads it on demand — `ui/src/lib/store.ts:635`).
  Record the choice here before implementing.

### Task 2: UI consumes the catalog

**Files:**

- Modify: `ui/src/components/settings/provider-catalog.ts`
- Modify: `ui/src/components/settings/custom-providers.ts:26-59,129-247`
- Modify: `ui/src/components/settings/ModelSettingsCard.tsx:330-470,555-582,758-850`
- Modify: `ui/src/components/settings/GenerationParamsCard.tsx:45`
- Modify: `ui/src/lib/store.ts` (catalog state + loading phase)
- Modify: the corresponding `*.test.ts` files

- [ ] RED: with no catalog loaded, the settings card renders an explicit loading
  state and no vendor rows (previously the rows were synchronously available).
- [ ] RED: with the catalog loaded, a vendor with two endpoint variants renders
  two selectable protocol rows that share one `display_name`.
- [ ] RED: a vendor whose only endpoint is deferred renders disabled with a
  capability badge.
- [ ] RED: the registry merge still prefers catalog matches before registry
  entries and still excludes `catalog-*` overlay entries from the custom list
  (`ui/src/components/settings/custom-providers.ts:194-204`).
- [ ] Keep `providerSelection` emitting `{provider: <adapter>, base_url,
  default_model}`; the wire shape does not change.
- [ ] Keep the per-message thinking control (`ui/src/components/chat/ChatInput.tsx:64-66,244`)
  driven by `context.thinking_supported`; this phase does not change it.

### Task 3: Refresh gate and vocabulary

**Files:**

- Modify: `internal/rpc/control.go:4780-4825`
- Modify: `ui/src/components/settings/custom-providers.ts:47-59`
- Modify: `ui/src/components/settings/ModelSettingsCard.tsx:562-582`

- [ ] RED: model-list refresh is allowed exactly when the endpoint's adapter is
  `openai-completions`; today it is a hard-coded 2-of-3 vendor list
  (`control.go:4793,4821`), and the UI mirrors it with its own whitelist
  (`custom-providers.ts:55-59`).
- [ ] Keep the `http(s)` base-URL requirement and its error message.
- [ ] Single source for the rule: derive the UI gate from the same adapter
  property the backend uses, so the two cannot disagree.

### Task 4: Delete the vendored generation path

**Files:**

- Delete: `ui/scripts/gen-provider-catalog.py`
- Delete: `ui/agent-diva-source/` (gitignored, untracked)
- Modify: `ui/src/components/settings/provider-catalog.ts:1-14` (drop the
  AUTO-GENERATED header)

- [ ] Confirm the 46 vendor entries in `internal/provider/data/vendors.yaml` are
  byte-for-byte equivalent to the 47 upstream entries minus `custom`, before
  deleting the source checkout.
- [ ] Delete the generator script and the vendored checkout.
- [ ] Grep proof: no remaining reference to `agent-diva-source` or
  `gen-provider-catalog` anywhere in the repository (excluding `.worktrees`).

### Task 5: Focused verification

- [ ] `go build ./...`, `go vet ./...`
- [ ] `go test ./internal/rpc ./internal/app -count=1`
- [ ] `cd ui; pnpm typecheck; pnpm test` (or the `just ui-ci` recipe)
- [ ] Real browser smoke at `http://127.0.0.1:3015` (split pair: `just run` +
  `cd ui; pnpm dev`): the Model settings page loads, shows the loading state,
  renders vendor rows with protocol variants, keeps the deferred row disabled, and
  a model selection round-trips to `settings.yaml`.
- [ ] Record commands, outcomes, and a screenshot path (or the reason a
  screenshot was not taken) in `verification.md`.

## Phase exit

Exit requires: zero provider data under `ui/src`; the catalog arriving over RPC;
the deferred adapter visible and disabled; the generator script and vendored
checkout gone; the browser path exercised at `:3015`.

## Commit

```
feat(provider): serve the provider catalog from the backend
```
