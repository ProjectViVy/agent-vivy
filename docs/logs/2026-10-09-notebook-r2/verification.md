# R2 verification

- `pnpm --dir ui exec vitest run ../plugins/vivy-notebook/ui/vivy-notebook/src/` — 27/27 pass (6 new client wire-pin tests + 6 new view behavior tests: capability absent, admission envelope pin, duplicate click, busy, lost-ack same-key retry, cancel by run id).
- `pnpm --dir ui typecheck` — clean (stage-ui regenerates `src/generated` from the default recipe).
- `pnpm exec playwright test --config playwright.notebook.config.ts notebook-reports.spec.ts` — 3/3 pass against packed `default` and `no-reports` binaries over real `/rpc` WebSocket (12.3s): lifecycle+provenance, restart resilience, capability absence.
- Live probe (packed default binary): `vivy.reports.generate` → `run_id` admission; `vivy.reports.get` → `completed` + full `GenerationProvenance` (promoted, "no facts in window") — caught the PascalCase admission serialization bug, fixed at the source.
- Module/source pins: `plugins/vivy-notebook/vivy-module.yaml` → fixed-point `source.sha256`; `conformance_results.json` internal digest → `e20a31fd…` after the `reportcontract` tag fix.
- Pending at log time: `just ci` (running); N3 notebook.spec.ts regression included in the same config.
