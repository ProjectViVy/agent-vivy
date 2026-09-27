# PROV-P3 — Configuration, Credentials, and Selection

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task.

**Goal:** Remove the three per-vendor configuration blocks and every hard-coded
vendor switch, so the default chain and the credential source both derive from
the embedded data — while existing `settings.yaml` documents keep working.

**Architecture:** `config.providers.active` names a vendor. The endpoint, adapter,
and model come from the embedded data plus the user's overlay. Credential
allowlists are derived from the vendor data rather than from three config fields.

**Tech Stack:** Go 1.26, `gopkg.in/yaml.v3`, existing `internal/app/settings`,
`internal/modules/credential`, `just ci`.

**Spec:** `docs/plans/provider-registry/DESIGN.md` §5, §6;
`docs/plans/provider-registry/MIGRATION.md` §1.3–§1.6, §3.

**Depends on:** `PROV-P1` (data and embed), `PROV-P2` (adapter table).

---

## Global constraints

- No secret ever enters the data file or config: `env_key` names only (D-010).
- Reading an old document must never fail because of the vocabulary change
  (`MIGRATION.md` §3).
- The `env_key` field becomes data-driven, which widens what the model module is
  allowed to read from the environment. The data is embedded and reviewed in the
  repository, so its trust level equals code — **record this explicitly** and
  keep `docs/TODO.md` `PROVIDER-DATA-CONFIG-EDIT` current. Do not add any
  permission UI.
- One focused commit; `just ci` blocked by unrelated lanes is recorded, not
  worked around.

---

### Task 1: Shrink `config.Providers`

**Files:**

- Modify: `internal/config/config.go:214-226,605-611,723-741`
- Modify: `internal/config/config_test.go`
- Modify: `config.example.yaml`

**Interfaces:**

```go
type Providers struct {
    // Active selects the vendor whose default endpoint is used when the
    // settings overlay names none. It must exist in the embedded vendor data.
    Active string `yaml:"active"`
}
```

- [ ] RED: `Validate` rejects an `active` value that is not an embedded vendor.
- [ ] RED: `Default()` still yields `Active: "deepseek"`.
- [ ] RED: a config document that still carries `deepseek:`/`openai:`/
  `anthropic:` blocks fails strictly (unknown field), and the error message names
  the removed key so an operator can fix it in one step.
- [ ] Delete the three `Provider` blocks, their `env_key`/`default_model`
  validation, and the `Provider` struct if nothing else uses it.
- [ ] Update `config.example.yaml` and the gitignored root `config.yaml`.
- [ ] Note in the phase log that `bundle_dir` was already removed in `PROV-P1`.

### Task 2: Credential allowlist from data

**Files:**

- Modify: `internal/app/app.go:277-283`
- Modify: `internal/app/model.go:45-60`
- Modify: `internal/modules/credential/module.go` (only if the signature must change)

**Interfaces:**

`credentialmodule.CompileScopes(profiles, channels, modelRefs...)` currently
receives three explicit env keys from config. The env-key set becomes the union
of the embedded vendors' `env_key` values (plus the adapter profiles'
`SecretRefs`, unchanged), so a third-party vendor's environment variable works
exactly like the first-party ones do today.

- [ ] RED: with `MINIMAX_API_KEY` set in the process environment and a MiniMax
  endpoint selected, the model is `Ready` without any per-vendor config block.
  Today this is impossible: `ProviderCatalogEntry` carries no `env_key` at all, so
  a third-party key can only arrive by pasting it into the UI registry.
- [ ] RED: a vendor with no environment variable set stays unconfigured and
  inactive; nothing is network-probed during construction or status inspection
  (preserve the existing guarantee).
- [ ] Keep the resolver itself unchanged: `Resolver.Resolve` still reads
  `os.LookupEnv` and still denies anything outside the allowlist.

### Task 3: Remove the hard-coded vendor switches

**Files:**

- Modify: `internal/app/app.go:1221-1248` (`applySettingsEnv`)
- Modify: `internal/app/app.go:1271-1287` (settings overlay)
- Modify: `internal/app/app.go:1527-1541` (`defaultModelFor`, `providerConfigBaseline`)
- Modify: `internal/app/model.go:62-116` (`freezeFromEnv`)
- Modify: `internal/app/model.go:135-170` (`currentLocked`)
- Modify: `internal/app/settings/settings.go:52-54` and its two validation switches

**Interfaces:**

```go
// EndpointForVendor resolves a vendor's default endpoint, or the endpoint
// addressed by an explicit base URL.
func (c *Catalog) EndpointForVendor(vendor, baseURL string) (Endpoint, Vendor, error)
```

- [ ] RED: with a legacy `settings.yaml` (`provider: deepseek`, no `base_url`),
  the resolved selection is vendor `deepseek`, adapter `openai-completions`,
  endpoint `https://api.deepseek.com`, model `deepseek-flash`.
- [ ] RED: `applySettingsEnv` writes the key into the **vendor's** env var. Today
  it uses a 3-way switch (`app.go:1233-1240`) over the config env keys, so
  selecting a MiniMax model through the `openai` bundle writes a MiniMax key into
  `OPENAI_API_KEY`. Add an assertion that this no longer happens.
- [ ] RED: `VIVY_PROVIDER` names a vendor; a frozen ENV session for a
  non-first-party vendor resolves correctly.
- [ ] Replace the three switches with data lookups. Delete `defaultModelFor` and
  `providerConfigBaseline` if the endpoint lookup subsumes them.

### Task 4: `settings.yaml` vocabulary aliases

**Files:**

- Modify: `internal/app/settings/settings.go:460-466,742-766`
- Create: `internal/app/settings/provider_migration.go` (normalization + its test)
- Modify: `internal/app/settings/settings_test.go`
- Modify: `internal/app/model.go:135-170`

- [ ] Implement the three-row normalization from `MIGRATION.md` §3, in one place,
  and call it from every read/validate path.
- [ ] RED: loading a document with `provider: anthropic` yields
  `anthropic-messages`; with `provider: openai` or `deepseek` yields
  `openai-completions`.
- [ ] RED: the same for `providers[].bundle`.
- [ ] RED: saving after a legacy read writes the normalized value.
- [ ] RED: an unknown provider value is still rejected on write and still does not
  crash a read.
- [ ] Keep the YAML key `bundle` (renaming it is optional and out of scope); keep
  the `(adapter, base_url)` uniqueness rule and its error message.

### Task 5: Focused verification

- [ ] `go build ./...`, `go vet ./...`
- [ ] `go test ./internal/config ./internal/app/... ./internal/modules/credential -count=1`
- [ ] `go test ./... -count=1`
- [ ] Manual real-path smoke (no network): start `go run ./cmd/vivy`, open the
  split UI at `http://127.0.0.1:3015` per `AGENTS.md`, and confirm the selected
  model still reports the same vendor/adapter/endpoint as before the migration
  when the legacy `settings.yaml` is present.
- [ ] Record commands and outcomes in `verification.md`.

## Phase exit

Exit requires: no production switch on a vendor name; a legacy `settings.yaml`
resolving to the same effective selection; `MINIMAX_API_KEY`-style environment
variables working for third-party vendors; and no secret in config, data, logs,
or the wire.

## Commit

```
refactor(provider): derive configuration and credentials from provider data
```
