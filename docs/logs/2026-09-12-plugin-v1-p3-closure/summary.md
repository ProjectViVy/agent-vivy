# PLG-P3 closure summary

## Outcome

PLG-P3 is complete. Its implementation landed with pull request #18, but the
phase plan remained unscheduled and Task 7 had no whole-envelope conformance
proof. This closure adds that proof and fixes the defect it exposed.

## Changes

- Added one end-to-end matrix for protected T1 Tools, public static Tools,
  public dynamic ToolWorld Tools, and MCP-derived dynamic Tools.
- Proved every class uses the same schema, pre-tool Middleware, Kernel Policy,
  approval, execution, bounded-result, redaction, Observer, and Journal path.
- Bound approval authority to a canonical SHA-256 digest of the Tool identity
  and exact post-Middleware arguments. The raw snapshot lives only in protected
  checkpoint state; resume re-evaluates governance and fails closed as stale if
  the resulting arguments differ.
- Fixed approval projection so `tool.approval_required` carries a recursively
  Secret-redacted view of the post-Middleware arguments. Before this change,
  the Provider received rewritten arguments while approval showed the original
  request, and Middleware-injected Secrets could enter the Journal.
- Redacted Provider error messages at the runtime Tool boundary while
  preserving the original Go error cause chain for `errors.Is`/`errors.As`.
- Reused pinned Eino `StatefulInterrupt` checkpoint state and the existing
  approval row's protected proposal payload instead of adding another approval
  or execution path.
- Updated the build-owned Tool and pre-tool Middleware conformance evidence to
  point at the complete matrix.
- Reconciled the PLG-P3 phase checklist and program status with merged reality.

## Architecture record

- Modules: existing `vivy/tool-host`, `vivy/protected-tools`, and selected
  Tool/ToolWorld Providers; no new Module was introduced.
- Trust: protected Providers remain T1; public static and dynamic Providers
  remain T2/T3 according to their selected source and process boundary.
- Ports: `std/tool@v1`, `std/tool-world@v1`, and
  `std/middleware/pre-tool@v1`.
- Sole consumers and authority: ToolHost owns catalog/dispatch, while Runtime
  retains final schema, Policy, approval, result, and Journal authority.
- Grants, lifecycle, timeouts, cleanup, failure state, and Inspect evidence
  remain those of the sealed Generation and existing Hosts.

## Explicitly not changed

- PLG-P7 internal Module composition.
- SCX integration gates in PLG-P8.
- Release conformance and rollback in PLG-P9.
- Public Port versions, Recipe selection, generated Assembly wiring, or Eino
  import quarantine.
