---
name: vivy-plugin-five
description: Redirect a request to implement the obsolete plugin five-step/v0 pack workflow to current Vivy v1 development. Current hello-fs work, historical discussion and prose mentions alone do not trigger this redirect.
---

# Legacy plugin workflow redirect

The five-step workflow targeted the rejected, unreleased v0 API. Do not use,
extend, emulate, or preserve it.

For actual Module/Port implementation, use the applicable `vivy-plugin` method.
Historical explanation or instruction edits do not start that workflow.

`vivy.plugin/v0`, `vivy-plugin.json`, `Seam`, `sdk/plugin.Plugin`, and the old
`pack --with` contract are not valid targets and have no migration path. If the
v1 phase required by the request is still `UNSCHEDULED` or only `SPECIFIED`,
stop and report that Gate instead of implementing with v0.

Do not edit `internal/runtime/engine.go`, hand-edit generated registration, or
scan plugin directories at runtime. The v1 source of truth is under
`docs/architecture/VIVY-*-STANDARD.md`, `VIVY-PORT-CATALOG.md`,
`VIVY-PLUGIN-SPEC.md`, `VIVY-ASSEMBLY.md`, and
`docs/plans/plugin-platform/`.
