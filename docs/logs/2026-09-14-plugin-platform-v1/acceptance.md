# Plugin Platform v1 release acceptance

Date: 2026-09-14

PLG-P9 is accepted locally when all of the following remain true:

- [x] Support state comes only from complete, exact Provider/Port/source
  evidence and all required Port artifacts.
- [x] The 23-case producer independently regenerates canonical release
  evidence and byte-compares it with the checked-in artifact.
- [x] The 25-case failure matrix executes real build/lifecycle paths, compares
  exact normalized diagnostics, and emits no formal artifact on rejection.
- [x] Default behavior remains present, while the minimal Generation
  physically omits unselected Modules, constructors, assets, edges, Grants,
  catalogs, and conformance records.
- [x] Default, minimal, full-UI, and SCX artifacts pack and inspect from final
  branch state.
- [x] Canonical catalog formatting is identity-neutral and semantic catalog
  changes alter Generation identity.
- [x] Whole-Generation rollback restores the prior executable, Manifest,
  catalog/locale identity, and next-launch behavior without Journal mutation.
- [x] VIVY CODE, Channel, MCP, and packed full-UI HTTP real paths pass without
  tenant data or live credentials.
- [x] Independent review reports READY with no remaining findings.
- [ ] GitHub Actions `backend-ci`, `ui-ci`, and Windows Playwright browser jobs
  pass on the published branch.
- [ ] PLG-1 is removed from the open board and issue #14 is closed only after
  the complete GitHub gate passes.

The phase remains `REVIEW READY · CI/BROWSER PENDING` until the last two items
are checked. No skipped browser slice is silently converted into acceptance.
