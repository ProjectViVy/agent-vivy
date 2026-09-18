# PROV-P1 — Provider Data Source and Embedding

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `internal/provider/data/` the single write point for all provider
configuration data, embedded into the binary at build time, validated strictly at
startup, and reconciled against the sealed adapter set.

**Architecture:** One vendor document (46 vendors, each with one or more protocol
endpoints) plus a JSON Schema in the same directory, declared with `//go:embed`
and parsed from an `embed.FS`. No runtime path, no working-directory dependency.

**Tech Stack:** Go 1.26, `//go:embed`, `gopkg.in/yaml.v3` strict decoding,
`just ci`.

**Spec:** `docs/plans/provider-registry/DESIGN.md` §2–§3, §5;
`docs/plans/provider-registry/MIGRATION.md` §1.2, §1.3, §1.4, §4, §5.

**Depends on:** nothing. `PROV-P2` depends on this phase.

---

## Global constraints

- No hand-written provider YAML remains outside `internal/provider/data/`.
- The running process never opens a provider data file from disk.
- Data cannot introduce an adapter, a transport, or executable behaviour.
- Strict decoding: an unknown key is a hard error, matching
  `internal/provider/bundle.go:87-88` today.
- Validation collects **all** failures with `errors.Join`, matching
  `internal/provider/bundle.go:98-150` today.
- One focused commit; `internal/sourcehash` digest update is deferred to
  `PROV-P5` but must be listed in the phase's own verification notes.

---

### Task 1: Author the vendor data

**Files:**

- Create: `internal/provider/data/vendors.yaml`
- Create: `internal/provider/data/README.md`
- Modify: phase iteration `verification.md`

**Interfaces:**

- Consumes: `ui/agent-diva-source/agent-diva-providers/src/providers.yaml`
  (47 entries, 1064 lines) as the **one-time derivation source**, and
  `fixtures/provider/{deepseek,openai,anthropic}.yaml` for the three first-party
  values already normalized by Vivy.
- Produces: the 46-vendor document in the Vivy schema.

- [ ] Derive entries mechanically, then review by hand. Rules:
  - drop `custom` (a placeholder for local endpoints; the user registry plus the
    UI's "Add custom provider" already covers it, and its empty `default_model`
    would fail validation);
  - drop the 10 fields listed in `DESIGN.md` §3.2 and the `api_type`/`backend`
    duplication, mapping `api_type: openai` → `adapter: openai-completions` and
    `api_type: anthropic` → `adapter: anthropic-messages`;
  - strip the gateway prefix from every `default_model` (11 of the 13 non-empty
    values carry one upstream, e.g. `openai/gpt-4o`, `zhipu/glm-4-flash`);
  - keep the three first-party values already in use: `deepseek` →
    `deepseek-flash` + `https://api.deepseek.com`; `anthropic` →
    `claude-sonnet-4-5` + `https://api.anthropic.com`;
  - give `deepseek` a second endpoint (`anthropic-messages`); the exact
    `base_url` comes from the DeepSeek Anthropic-API documentation and is a data
    value, not a design decision;
  - declare `openai-responses` endpoints for `openai` so the deferred capability
    is visible;
  - carry `provenance{source, entry, derived_at}` on every vendor (D-025 is
    unchanged);
  - leave every model metadata field absent. Unknown means `context_window: 0`,
    which preserves today's `128000` fallback
    (`internal/runtime/compaction_policy.go:17`).
- [ ] Populate model metadata **only** where the current Go tables already assert
  it: `internal/provider/openai.go:48-68` and `internal/provider/claude.go:88-103`.
- [ ] Write `internal/provider/data/README.md` (English): what the file is, who
  edits it, how to add a vendor, how to add an endpoint variant, why unknown
  metadata is absent rather than guessed, and the rule that the running process
  never reads it from disk.
- [ ] Record the row counts (47 upstream → 46 vendors; endpoint and model counts)
  in `verification.md`.

### Task 2: Author the schema

**Files:**

- Create: `internal/provider/data/provider.schema.json`
- Delete: `schemas/providers.bundle.schema.json` (in this phase, since the old
  bundle shape stops existing here)

**Interfaces:**

- Produces: the review surface for edits, and the documented contract that
  `internal/provider/vendor.go` enforces in Go.

- [ ] Model the document as a top-level array of vendors with the field set in
  `DESIGN.md` §3.1; `additionalProperties: false` at every level.
- [ ] Encode the constraints from `DESIGN.md` §3.4 rules 1–11 (patterns, required
  fields, `default_model ∈ models`, unique `(adapter, base_url)`, capability
  vocabulary per adapter).
- [ ] Update `schemas/README.md` so it no longer points at `../fixtures/provider/`
  and states that the provider document lives with the code that consumes it.

### Task 3: Types, embedded loading, and validation

**Files:**

- Create: `internal/provider/embed.go`
- Create: `internal/provider/vendor.go`
- Create: `internal/provider/vendor_test.go`
- Delete: `internal/provider/bundle.go`

**Interfaces:**

```go
//go:embed data/*.yaml
var dataFS embed.FS

type Model struct {
    ID              string
    ContextWindow   int
    InputPerMTok    float64
    OutputPerMTok   float64
    SupportsImages  bool
    SupportsThinking bool
}

type Endpoint struct {
    Adapter               string   // an adapter family id
    BaseURL               string
    DefaultModel          string
    Models                []Model
    Capabilities          []string
    SupportsPromptCaching bool
}

type Vendor struct {
    Name        string
    DisplayName string
    EnvKey      string
    Endpoints   []Endpoint
    Provenance  Provenance
}

// LoadEmbedded parses and validates every embedded vendor document.
// It returns an error joining every validation failure.
func LoadEmbedded() ([]Vendor, error)
```

- [ ] Write the failure-first tests in `vendor_test.go` before the loader:
  unknown key rejected; missing required field rejected; `default_model` outside
  `models` rejected; duplicate `(adapter, base_url)` rejected; a model id carrying
  a `vendor/` prefix rejected; an unknown capability rejected; a capability
  declared on an adapter that does not implement it rejected; two vendors with the
  same name rejected; every error joined rather than returned one at a time.
- [ ] Implement `LoadEmbedded` with `yaml.Decoder` + `KnownFields(true)` over
  `dataFS`, then validate.
- [ ] Keep `Provenance` and its three required fields unchanged (D-025).
- [ ] Delete `internal/provider/bundle.go` in the same commit that removes its
  last caller (`PROV-P2` Task 3 / this phase's Task 4 use the new loader only).

### Task 4: Startup consistency gate

**Files:**

- Create: `internal/provider/reconcile.go`
- Create: `internal/provider/reconcile_test.go`
- Modify: `internal/app/app.go` (call site)

**Interfaces:**

```go
// ReconcileAdapters proves the embedded data and the sealed adapter set agree
// in both directions. A mismatch is a startup failure.
func ReconcileAdapters(vendors []Vendor, sealedAdapters []string) error
```

- [ ] Write the RED tests: data naming an unsealed adapter fails; a sealed
  `SUPPORTED` adapter with no endpoint fails; a sealed
  `DEFERRED-INDEFINITE` adapter with no endpoint passes.
- [ ] Implement the bidirectional comparison.
- [ ] Call it from `internal/app/app.go` immediately after the catalog and the
  runtime assembly are both available, before the model resolver is constructed.

### Task 5: Replace the disk loader at the app boundary

**Files:**

- Modify: `internal/app/app.go:254-272`
- Modify: `internal/eval/isolator.go:74-93`
- Modify: `internal/config/config.go:214-226,605-611,723-741`
- Modify: `config.example.yaml`, `config.yaml`
- Modify: `Dockerfile:36-40`
- Delete: `fixtures/README.md`, `fixtures/provider/*.yaml`

- [ ] Replace the `bundlePath` closure and the three `LoadBundle` calls with one
  `provider.LoadEmbedded()` and catalog construction.
- [ ] Delete `providers.bundle_dir` from the config struct, `Default()`,
  `Validate()`, `config.example.yaml`, the gitignored root `config.yaml`, and the
  eval child config (drop the `filepath.Abs` block entirely).
- [ ] Add no new dependency in this phase. In particular do **not** pull
  `eino-ext/components/model/agenticopenai`: the Responses API stays deferred
  (`EINO-CAPABILITY.md` §4–§5) and this phase changes data structure only.
- [ ] Remove `COPY fixtures/provider /app/fixtures/provider` from the Dockerfile
  and confirm `WORKDIR /app` no longer participates in provider resolution.
- [ ] Delete the `fixtures/` directory. Confirm no remaining reference:
  `git grep -n fixtures -- ':!.worktrees' ':!docs'`.

### Task 6: Focused verification

- [ ] `go build ./...`
- [ ] `go test ./internal/provider ./internal/config ./internal/eval ./internal/app -count=1`
- [ ] `go test ./... -count=1` (the full-suite proxy for this phase; `just ci`
  also runs the UI and plugin lanes, which this phase does not touch)
- [ ] Start the split pair (`just dev` is unnecessary for this phase; a plain
  `go run ./cmd/vivy` from the repository root is enough) and confirm the process
  starting from a **different** working directory still resolves DeepSeek — the
  working-directory trap is the regression this phase removes.
- [ ] Record every command and outcome in `verification.md`.

## Phase exit

Exit requires: no provider YAML outside `internal/provider/data/`; no
`bundle_dir` anywhere; `LoadEmbedded` reachable with no filesystem path; the
consistency gate aborting startup on a deliberate mismatch; and the repository
building from any working directory.

## Commit

```
refactor(provider): embed provider data as the single source
```
