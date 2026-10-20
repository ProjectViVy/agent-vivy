# D02 acceptance repair

The owner required D02 acceptance before D03 implementation. This follow-up
starts from `codex/issue91@7f9eca45` and repairs the observed verification
blockers rather than treating focused D02 checks as full acceptance.

- `App.Run` now uses the existing Close owner on cancellation and listener
  failure. The shared close path also shuts down HTTP, and releases cognitive,
  observer, action, memory and diagnostics owners before core storage.
- Subprocess protocol fixtures use a native Go executable. The real CODE
  conversation owns an isolated user home and initializes its private Persona.
  Command prefix/spill fixtures exercise portable executable/printf behavior.
- UI packing stages sources under ignored `ui/.cache`, retaining separate
  source/output directories while using Node ancestor dependency resolution.
  It no longer requires privileged Windows symlinks or copied dependencies.
- The existing conformance producer has an explicit update mode. Source
  identities derive from the current source and declarations, owning tests
  must actually execute, and only complete passing results can replace the
  artifact. Failed or incomplete runs preserve the previous artifact bytes.
- The shared truncation conformance fixture creates its fork session before
  writing that session's provenance marker, satisfying the existing PostgreSQL
  foreign key. The complete real PostgreSQL gate passes on a disposable WSL
  Docker instance.

No runtime loop, approval, database format or D03 operation contract is changed
by this prerequisite cut. No dependency or Port is added. D03 implementation
and final release conversion remain separate deliverables.

Release: no release or push is requested; this is a focused source delivery.
