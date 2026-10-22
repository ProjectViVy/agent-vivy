# D14 action audit and active contract residue

Removed ActionHost input/output SHA-256 fields, slog keys, payload hashing and
the early-denial placeholder calculation. Mandatory pre-effect admission and
completion audit remain on the existing Host/sink path. The recorder receives
byte counts rather than payloads; identity, effect, outcome, safe error class,
size limits and duration remain bounded metadata.

Updated only the affected active paragraphs in LOGGING, the Port Catalog,
Memory Host Contracts, Face Pack and Channel Pack. They now describe actual
schema comparison, optional explicit Skill revisions and exact edit bytes,
explicit Module/source and opaque Generation identity, and the existing
cognitive capture provenance and durable watermark. Session export remains a
proposed seam; removing its digest requirement does not implement or retire it.

Owner inspection: `internal/mcphost/host.go` (`definitionsChanged`),
`sdk/port/skillsource/skillsource.go`, `internal/skillhost/host.go`,
`internal/runtime/skills_backend.go`, `sdk/internal/assembly/{source,manifest}.go`,
`internal/cognitivecontract/ports.go`,
`internal/modules/diva-cognitive/factory.go`, and
`internal/runtime/cognitive_service.go`. The pinned Laputa
`garden/agentapi/{contract,service}.go` confirms session-bound original
Run/event provenance and committed receipt sequence.

Eino capability check: this cut changes Host audit metadata and documentation,
not model/tool orchestration. The repository pin remains Eino v0.9.21 and
EinoExt MCP v0.0.9; existing schema/Skill/capture owners remain authoritative.
No Eino replacement, additional runtime, replay registry, content token or new
source of truth was added.

Scope excludes production data, historical changelog rewrites, packaging,
upstream changes, UI behavior and D14 acceptance. No release was produced, so
there is no release record. This is a focused successor to base
`a12748a7114bb54fd0bdf663bfb1f01fcec90802`.
