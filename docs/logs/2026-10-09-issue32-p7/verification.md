# P7 verification record

## VIVY producer

Passed on `feat/issue32-remediation` with the integrated source state:

- `go test -timeout 35m ./...`
- `go vet ./...`
- `go test ./sdk/internal/conformance -run '^TestCheckedInProviderConformanceMatchesExecutedSuites$' -count=1 -timeout=35m`
- `go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui`
- TypeScript `tsc --noEmit -p tsconfig.json`
- `vitest run`: 80 files, 620 tests passed
- Vite production build
- `node scripts/check-i18n-completeness.js`
- `node --test scripts/check-i18n-cross-face.test.js`: 8 tests passed
- `node scripts/check-i18n-cross-face.js`: 13 shared units passed
- `go vet ./... && go test ./...` for each Go module under `plugins/` and `faces/`
- Split UI browser smoke: initialize the temporary Persona, create a fresh
  session, create/save/validate/publish r1, hold an active parent Run on a local
  mock provider, start the published workflow, and inspect the terminal Run,
  completed node, and journal events; no page errors.
- Source-closure verification: Laputa lock, Go module pseudo-versions and the
  selected Laputa checkout all resolve to
  `30fa208e3cded4af43f8cb226d80b225b1910854`; checkout is clean and its module
  declarations match the locked identities.

`just ci` itself was attempted and exited before running recipes because
`powershell.exe` is missing. All Linux-executable recipe components were run
directly; this environment-specific wrapper limitation is documented in
`summary.md`.

## DIVA consumer

Still in progress. The canonical lock repin and consumer-side tests will be
recorded after the final VIVY source commit exists. GTK/WebKit and Windows native
candidate validation are unavailable in this environment.
