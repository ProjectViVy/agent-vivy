# PR #20 integration acceptance

- [x] P6 composes one selected UI root and ordered extensions from an explicit,
  sealed Recipe without runtime discovery.
- [x] A selected trusted UI Module can replace the visible Web Face, register a
  route/style/theme, and observe live Face state without a UI permission path.
- [x] The fixture's backend action crosses only `module.action.invoke`; forged
  browser approval and Trust fields are rejected server-side.
- [x] Action execution re-enters the existing `Service.Run` and ToolHost seams,
  and host/per-action timeouts have deterministic public classification.
- [x] Pack and Inspect preserve source, lock, SDK, catalog, Generation, and final
  asset provenance.
- [x] The default Generation omits the fixture and retains the already-landed
  P4/P5 governed hosts.
- [x] `just ci`, independent review, and the real split-pair browser smoke pass.
