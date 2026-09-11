# PLG-P6 UI conformance acceptance

- [x] Browser fixtures prove selected root/extension behavior and isolate
  `default`, `extension`, `replacement-root`, and `minimal` generations.
- [x] Duplicate/missing Provider and contradictory order inputs fail closed.
- [x] TypeScript build failure is observable and cannot publish a Generation.
- [x] Runtime install failure rolls back registrations; successful cleanup is
  reverse-order and idempotent.
- [x] Source, dependency-lock, asset, SDK, Generation, and UI provenance are
  sealed and inspected through the real Go seams.
- [x] Typed `module.action.invoke` reaches the client seam and displays an
  Action failure without a UI permission path.
- [x] Minimal output has no selected UI imports/provenance and explicitly
  rejects old v0 UI, route, and permission markers.
- [x] Seven-artifact evidence is complete for both UI Ports and Control Action.
- [ ] `just ci` and split-pair Playwright smoke remain release-conformance
  work for PLG-P9 and are intentionally not claimed here.
