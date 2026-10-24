# E3 — `tool_search` meta-tool (coding-tools module)

**Goal:** model-invokable search over the deferred tool catalog; returning matches activates them for the session (pi `tool_search` equivalent).
**Epic:** E. **Requirements:** RQ-TOOL. **Predecessor:** E2 (exposure + activation API).
**Spec:** VCP-D1 §5.6.

## Scope

**Files:** `plugins/coding/tools/` — `vivy-module.yaml`, `module.go` (Descriptor: `std/tool@v1` id `tool_search`), `search.go` (BM25-ish scoring over name+description+namespace; tiny hand-rolled scorer — no external index dep). Consumes ToolWorld catalog through the Host facade at query time (never a cached copy that can drift — schema-hash rule).

## Tasks

- [ ] Tool schema: `{query: string, limit?: int}` → ranked `[{id, namespace, description, activated:bool}]`; side effect: top-N results activate via `tools/activate` (Journal event carries which).
- [ ] Namespace-aware display (mcp.*, tools.*).
- [ ] Direct-exposure tools are always searchable but never need activation.
- [ ] Module descriptor + digest pin + conformance.
- [ ] Tests: query ranking on a fixture catalog; activation actually flips model-visible catalog next turn; hidden tools never appear.
- [ ] `go test ./plugins/coding/tools`; `just ci`.
- [ ] Commit `feat(plugins): tool_search module`.

## Boundary

No codemode (O5). Activation is per-session; module cannot widen its own grants.

## Acceptance

Model asks tool_search "file versions" → deferred tool activates → next turn calls it directly.
