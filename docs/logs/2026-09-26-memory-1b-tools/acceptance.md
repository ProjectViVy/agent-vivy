# Acceptance: how a human can tell MEM-1B is real

Product/maintainer view: an agent run can now read and write long-term
memory through the ordinary governed tool path — six `memory_*` tools ride
`std/tool@v1` off the same composition-owned `Active()` service the control
actions and sync providers use, writes pass the runtime approval gate, and
the six cheap MEM-1A review defects are closed.

## 1 — The wiring is in the artifact

```text
rg -n 'vivy/memory-bml-tools|ToolProviders' internal/modules/defaults/catalog.go recipes/default.vivy.yml
rg -n 'memory_(add|get|list|search|update|remove)' internal/modules/memory/tools.go
rg -n 'memory-bml-tools' internal/generated/assembly/zz_default.go
```

A third `boundRecord` (`vivy/memory-bml-tools`) alongside `vivy/memory-bml`
and `vivy/memory-bml-sync`, all pointing at `internal/modules/memory`; its
`ProviderCollection` emits one `std/tool@v1` provider per declared PortRef
id into the generated manifest's `Tools` set.

## 2 — Write authority goes through the approval gate

`Effect: tool.EffectWrite` on `memory_add`/`memory_update`/`memory_remove`
yields `ToolSpec.Readonly=false` in `bindGeneratedTools`, so a write tool
cannot bypass the existing runtime approval gate — that is the MEM-0D
mutation-authority surface. The three reads declare `EffectRead` and
auto-execute. CAS discipline carries over: `memory_update` and
`memory_remove` require `base_revision` and return `conflict` on a stale
token; closed-enum fields (kind/trust/provenance/scope) reject anything
outside profile §6.

## 3 — Behavior is test-backed

```text
go test ./internal/modules/memory -count=1
```

`tools_test.go` proves the tool surface (ids, effects, schemas, invoke
paths on a temp store, `bml_unavailable` when the service is down);
`provider_test.go`/`module_test.go`/`actions_test.go` prove the six
defect fixes — poison events ack without wedging, evidence preserved on
omission, colliding ids dedupe separately, absurd cursors rejected,
snake_case output-cap reason, idempotent `Open`.

## 4 — Trackers agree

```text
rg -n 'MEM-1B' docs/superpowers/plans/memory/index.md
```

`index.md` shows MEM-1B Done 2026-09-26 pointing here. MEM-2 stays
Blocked until its plan is written on G1 evidence.

## Known limits

- `memory_distill` and the `actmem_*` family are deferred: upstream
  `memory_distill` only files a pending governed Skill request and Vivy
  has no governed-request seam yet (MEM-3 territory).
- Four MEM-1A review nits remain deferred as latent or unreachable:
  WriteRules check-then-act atomicity, `memory-bml-sync`-only recipe
  asymmetry, `AppendHistory` content-equality on same-ID redelivery, and
  the shared `ownerModule.Descriptor()` id.
- `defaultRecallLimit = 100` bounds agent read fan-out as a service
  constant; not yet a contract-pinned profile number.
- Agents see only the `machine-memory-home` scope and `long_term` kind —
  host-assigned identity, not caller-selected.
