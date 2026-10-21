# D05: inert warming and steering factories

Baseline: accepted D04 `d098d4e2` on stable. Extend its same typed seam with pure
SDK warming/steering policy DTOs, runtime bound Host/opaque prefix contracts,
closed T1 factory Ports (`vivy/warming`, `vivy/steering`), named build bindings,
generated absent/selected accessors and typed App unwrapping shared by startup
and settings/tools/MCP/compaction rebuilds. A fixed compiler
table reuses D04's validator for the three loop factories. Factories remain lazy
metadata in EngineConfig; none is constructed or executed.

Core remains the sole owner of real off/streaming/idle warming, savings gate,
maintenance timeout/cancellation/accounting and cache.warmed, and same-Run
steering, durable dual-track queue, cancellation/checkpoint resume. No production
warmer/queue implementation changes, provider, adapter, settings read, Recipe,
second scheduler or behavior retirement is introduced. #93 owns actual ordinary
plugin extraction and same-change core-body deletion. No physical-omission claim.

Fresh residue scan: no hash in cachewarmer.go or queue.go; turn_queue.go is absent.
No duplicate steering setup was proven. Service continuityInputHash/source
expectations are D09; Work/workflow hashes are D11; migration metadata is D12.
Notebook digest is a text summary. No unrelated owner is renamed or deleted.

Eino capability check at pinned v0.9.21: maintenance uses model.BaseModel.Generate;
steering uses adk.WithAgentCancelMode(adk.CancelAfterChatModel), WithCheckPointID,
TypedRunner.ResumeWithParams and existing typed middleware. The later Host seam
retains Vivy accounting/admitted Run ownership without replacing these APIs.
