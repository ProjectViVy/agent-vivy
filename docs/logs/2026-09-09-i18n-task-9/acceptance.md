# Acceptance

Source and documentation checks completed in Task 9:

1. README states exact en/zh support, English fallback, process-over-`.env`
   defaults, immutable sealed defaults, and the global workspace override.
2. Both faces retain the same backend locale authority; localStorage is only
   a cache. Failed/read-only writes do not establish a client-only preference.
3. Completeness rejects the old hard-coded reveal diagnostic; the runtime
   emits the selected catalog message while keeping content visible on failure.
4. CI wiring retains all existing gates and includes completeness after the
   UI dependencies are installed. `.env` remains ignored and untracked.
5. Task 8 is visibly deferred, not implemented or normative. The v1 Module ID
   and selected-Module artifact implementation is an explicit prerequisite.

Still required in a Go/just-enabled environment; not executed here:

- Run `go test ./...` and `just ci`, including TUI catalog/render/hydration,
  pack embedding, headless, and plugin-module checks.
- Start the normal split backend/Vite pair and exercise Settings → Language
  in both locales, persistence across restart, and read-only/failure behavior.
- Launch both in-process TUI and `--live`; verify backend hydration, fallback
  warning sanitization, bilingual chrome, and unchanged user/model/tool data.
- Pack with process/file defaults, change the source environment, and confirm
  the sealed artifact retains its default while the global override wins.
- Establish the approved canonical cross-face catalog-shape conformance; do
  not substitute independent en/zh parity checks for that requirement.

These pending checks are not waived by Task 8's plugin-only deferral.
