# Provider Registry (Single Source of Truth) Program Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement one scheduled phase task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give Vivy exactly one write point for provider configuration data, and
make the sealed unit a **protocol adapter** rather than a vendor, so that adding a
vendor is a data edit and a vendor speaking two protocols is ordinary data rather
than a special case.

**Architecture:** A three-row adapter table in `internal/provider` is the sealed
capability set. One embedded vendor document (46 vendors, each with one or more
protocol endpoints) is the single source of provider data. The running process
never reads provider data from disk. The frontend holds no provider data at all
and receives the catalog over RPC.

**Tech Stack:** Go 1.26, `//go:embed`, `gopkg.in/yaml.v3`, repository-pinned
`eino v0.9.13`, `eino-ext openai v0.1.13`, `eino-ext claude v0.1.25`,
JSON-RPC over WebSocket, React/TypeScript/Vite, generated Assembly wiring,
`just ci`.

**Spec:** this directory — `DESIGN.md`, `EINO-CAPABILITY.md`, `MIGRATION.md`, and
the five phase files.

**Provenance:** this program was designed in a single owner session on
2026-09-18. It is documentation-only until a human schedules a phase.

---

## Global constraints (standing orders)

- **Eino-native first.** For provider/model capability, adapt a concrete pinned
  Eino/EinoExt API. If it is absent, mark the capability `DEFERRED-INDEFINITE` and
  add no custom substitute (root `AGENTS.md`).
- **Import quarantine.** Only `internal/runtime/` and `internal/provider/` may
  import `github.com/cloudwego/eino*`.
- **One data write point.** All provider configuration data lives in
  `internal/provider/data/`. No provider YAML lives anywhere else.
- **No runtime disk reads.** Provider data is embedded at build time; there is no
  directory setting and no working-directory dependency.
- **Data supplies data, never capability.** A vendor entry cannot introduce an
  adapter, a transport, or executable behaviour.
- **Secrets never leave the environment boundary.** Config and data carry
  `env_key` names only (D-010).
- **Human scheduling only.** A human changes an `UNSCHEDULED` phase to scheduled.
- **One deliverable per commit**, with its iteration log under `docs/logs/` and
  `just ci` as the gate.
- **No push** without explicit owner authorization.

---

## Relationship to PLG-P5 (no second source of truth)

The plugin-platform program (`docs/plans/plugin-platform/`) completed P0–P9. Its
**PLG-P5** (`DONE · 2026-09-10`) built the machinery this program reuses:
`sdk/port/providerprofile` (the public declarative Profile),
`internal/modelhost` (capability states and status projection),
`internal/modules/defaults/providers.go` (first-party Profiles), and the UI status
projection.

This program is a **new epic** and does not change the PLG status board. It
**revises PLG-P5 Task 4's output** (three vendor-named Profiles become three
protocol-named adapters) and **PLG-P5 Task 5's data source** (the UI catalog comes
from the backend rather than from a generated TypeScript array). Everything
PLG-P5 established about atomicity — one ModelHost, no public executable
Provider, declarative Profiles only, `DEFERRED-INDEFINITE` for anything the pinned
Eino surface cannot do — is preserved unchanged.

The Port evidence anchors that PLG-P5 established must stay valid:
`sdk/internal/assembly/evidence.go:77-80` and
`sdk/internal/conformance/reproduction_test.go:456` cite
`internal/modules/defaults/providers.go#ProviderProfiles` and
`TestDefaultProviderProfilesMatchExistingRuntimeFamilies`. Keep those symbol
names; change only their values (`PROV-P2` Global constraints).

---

## Decision ledger

Every decision below is closed. A phase may not re-open one; if implementation
shows a decision is wrong, stop and amend this ledger first.

| # | Decision |
|---|---|
| **D1** | The single write point is `internal/provider/data/`; `fixtures/` is deleted. |
| **D2** | The data is `vendors.yaml` plus `provider.schema.json` in that directory; both are embedded. |
| **D3** | Data is embedded with `//go:embed` and parsed from the embedded FS at startup. The running process never reads it from disk; `providers.bundle_dir` is deleted. |
| **D4** | Four layers: Adapter (code, sealed) / Vendor (identity + key) / Endpoint (protocol + address + default model + models) / Model (metadata, inline). |
| **D5** | Three adapters, named after the real wire APIs: `openai-completions`, `openai-responses`, `anthropic-messages`. |
| **D6** | `openai-responses` is `DEFERRED-INDEFINITE`, with the explicit owner commitment to migrate onto `AgenticModel` as its only lift path. |
| **D7** | Default is vendor `deepseek` on adapter `openai-completions` (OpenAI-standard streaming), overridable by user configuration. |
| **D8** | Endpoint identity is `(adapter, base_url)`. The selection payload `{provider, base_url, default_model}` keeps its exact shape; only its vocabulary changes. |
| **D9** | `settings.yaml` migration map: `deepseek`→`openai-completions`, `openai`→`openai-completions`, `anthropic`→`anthropic-messages`. |
| **D10** | The schema keeps 8 fields; 10 zero-consumer fields and the redundant `backend` are deleted. |
| **D11** | Model metadata slots exist in phase 1 with values left empty; `context_window: 0` means unknown and preserves the existing `128000` fallback. |
| **D12** | The DeepSeek reasoning request shape is an endpoint capability (`deepseek-thinking`), not a fourth adapter. |
| **D13** | The vendor catalog is derived once from the Diva registry (47 entries; `custom` dropped because it is not a vendor, `cherryin` dropped because it declares no models and no default model → **45**), gateway prefixes stripped, and the repository then stands alone; `ui/agent-diva-source/` is deleted. Amended in `PROV-P1` with the as-built count (`MIGRATION.md` §7.2). |
| **D14** | The frontend holds zero provider data; the catalog arrives over RPC; a first-paint loading state is accepted. |
| **D15** | A startup consistency gate compares the embedded data against the sealed adapter set bidirectionally and fails closed. |
| **D16** | `env_key` becoming a data field makes "which environment variables the model module may read" data-driven. Because the data is embedded and reviewed in-repo, its trust level equals code; if it ever becomes user-editable, the boundary must be re-evaluated. |
| **D17** | A user registry entry may declare metadata/dialect for unknown models. Out of phase-1 scope. |
| **D18** | The `/models` refresh gate is `adapter == openai-completions`, replacing the hard-coded 2-of-3 vendor list. |
| **D19** | The `reasoning_content` response-decode gap is **not** part of this program; it stays with `docs/TODO.md` `DEEPSEEK-REASONING-CONTENT` and no Vivy-owned decoder is built. |

---

## Program status

| Phase | Deliverable | State | Depends on |
|---|---|---|---|
| PROV-P1 | Provider data source and embedding | `DONE` | — |
| PROV-P2 | Adapter table and sealed adapter set | `DONE` | P1 |
| PROV-P3 | Configuration, credentials, and selection | `DONE` | P1, P2 |
| PROV-P4 | Catalog RPC and zero-data UI | `DONE` | P1–P3 |
| PROV-P5 | Conformance, evidence, and closeout | `DONE` | P1–P4 |

The owner scheduled the whole sequence on 2026-09-18: all five phases run in
order on one branch, `feat/provider-registry`.

## Critical path

```text
P1 data + embed + consistency gate
  -> P2 adapters + sealed manifest + thinking capabilities
  -> P3 config + credentials + settings aliases
  -> P4 catalog RPC + UI zero-data
  -> P5 conformance + evidence + closeout
```

The phases touch the same files in a fixed order and must land as one sequence on
one branch: `PROV-P3` changes the meaning of stored `settings.yaml` values, and
`PROV-P4` removes the UI's only data source, so splitting the sequence across a
release boundary would leave a broken intermediate state (`MIGRATION.md` §6).

---

## Phase index

| File | Contents |
|---|---|
| `DESIGN.md` | The problem with evidence, the four-layer target model, the full data schema with per-field consumers, endpoint identity and data flow, the default chain, unknown-metadata semantics, availability projection, worked examples, and rejected alternatives. |
| `EINO-CAPABILITY.md` | The mandatory Eino capability check: pin-by-pin evidence for all three adapters, why `openai-responses` is deferred, the committed migration path and its blast radius, and the out-of-scope `reasoning_content` gap. |
| `MIGRATION.md` | Per-file change and delete inventory, the `settings.yaml` migration map, the conformance digest step, the WF-1 WIP warning, and rollback per phase. |
| `PROV-P1-data-source-and-embed.md` | Author the vendor data and schema, embed it, validate strictly, add the consistency gate, delete `fixtures/` and `bundle_dir`. |
| `PROV-P2-adapters-and-assembly.md` | The three-row adapter table, resolution by family, capability-driven thinking, re-sealed Generation. |
| `PROV-P3-config-and-credentials.md` | Shrink config to `providers.active`, derive credential allowlists from data, remove every vendor switch, add the settings aliases. |
| `PROV-P4-rpc-and-ui.md` | Catalog RPC, UI loading state and vocabulary, refresh gate, deletion of the generator and the vendored checkout. |
| `PROV-P5-conformance-and-closeout.md` | Conformance digest, failure-path coverage, `just ci`, browser smoke, iteration log, backlog rows. |

---

## Expected net effect

- **Deletions:** `fixtures/` (4 files), `providers.bundle_dir` (5 sites),
  `ui/scripts/gen-provider-catalog.py`, `ui/agent-diva-source/` (a whole vendored
  Rust checkout), `internal/provider/bundle.go`'s disk loader, 10 dead schema
  fields, the redundant `backend` field, `ProfileFromBundle`'s YAML projection,
  Docker's fixture `COPY`, and `schemas/providers.bundle.schema.json`.
- **Additions:** one data directory (2 files plus a README), one embed file, one
  three-row adapter table, one consistency gate, one catalog RPC payload, and a UI
  loading state.
- **Problems removed:** four duplicated facts with no guard; a lazy-only family
  check; a hard-coded `bundle.Name != "deepseek"`; an impossible-to-express
  two-protocol vendor; a launch failure when the working directory is not the
  repository root; and a UI catalog whose generation source is not in the
  repository.
- **Problems explicitly retained:** the deferred Responses adapter (with a
  committed path), the `reasoning_content` response gap, empty model metadata for
  44 of 46 vendors, the hand-maintained conformance digest, and the
  `env_key`-as-data trust boundary.
