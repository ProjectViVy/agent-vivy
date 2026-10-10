# Verification — headless form mask arming

## Go gates

| Command | Result |
|---|---|
| `go build ./...` | pass |
| `go vet ./internal/app/... ./internal/generated/assembly/... ./sdk/internal/assembly/... ./sdk/internal/cmd/generate-default/...` | pass |
| `go test ./sdk/internal/assembly/...` (incl. new `TestGenerateRuntimeAssemblyFormIdentity`) | pass |
| `go test -timeout 35m ./internal/app/...` (full suite, incl. new `TestHeadlessFormArmsMasksAndAdmissionWithoutInjectedIdentity`) | pass (46.1s) |
| `go test ./internal/runtime/... ./internal/rpc/... ./internal/storage/...` | pass |
| `go test ./sdk/...` | pass except two packages — see "SDK suite notes" |

## SDK suite notes

- `sdk/internal/conformance`: first run failed
  `TestCheckedInProviderConformanceMatchesExecutedSuites` — expected: the
  test hashes the live `internal/` tree (zz_default.go excluded) against the
  checked-in canonical digest, and this change edits `internal/app` comments
  and adds a test file. Per the documented procedure
  (`docs/plans/provider-registry/MIGRATION.md` §8.7) the five
  internal-rooted `sourceSha256` entries in
  `sdk/internal/assembly/conformance_results.json` were refreshed in this
  same change to the recomputed canonical value
  `6e3c6e54…` (= `HashSourceTree(<repo>/internal, "")`), and the full
  conformance suite re-ran green (`go test ./sdk/internal/conformance/ -count=1`).
- `sdk/internal`: `TestPackAndInspectSharedTarget` fails locally with
  "`-buildmode=c-shared requires external (cgo) linking, but cgo is not
  enabled`". Environment limitation: this machine has no C toolchain
  (`CGO_ENABLED=0`); CI runs windows-latest where MinGW is preinstalled.
  The test's build inputs are untouched by this change.

Existing tests that pin the old dormant behavior
(`TestPrimaryAdmissionGateKeepsUnsealedEmbedderCompatibility`,
`TestMaskManagerForAssemblyIsDormantWhenUnsealed`) pass unchanged: they call
the composition functions directly with an empty identity, which is exactly
the custom-embedder case the legacy branch still serves.

## Artifact regeneration

- `go run ./sdk/internal/cmd/generate-default --repo . --output internal/generated/assembly/zz_default.go`
  succeeds; the diff is exactly the `HeadlessGenerationID` const plus the
  `GenerationID:` assignment (8 added lines, no other drift).
- `go run ./sdk verify plugins/vivy-masks-ui` → `ok vivy/masks-ui` after the
  source-pin repair.

## Real-path smoke (split pair, Windows)

Environment: fresh `VIVY_USER_HOME` scratch (no repo `data/` touch), frozen
env provider (`VIVY_PROVIDER=deepseek`, base URL = a local hanging mock on
`127.0.0.1:9911`, no real key), Vite dev server on `127.0.0.1:3015` proxying
`/rpc` to the Go backend on `127.0.0.1:8787`.

1. Backend startup log contains no "control actions disabled" warning.
2. In the browser, the mask picker lists the built-in catalog (Programmer /
   Researcher / Writer / Just me) — `vivy.masks.catalog.list` works.
3. Selecting Writer succeeds with no error; the banner shows
   "Session mask: Writer".
4. The selection survives a page reload and a full backend restart.
5. Sending a message with the mask selected produces an `active` run held by
   the hanging mock — admission happened before the model call.
6. Read-only sqlite inspection of the scratch journal:
   - `run_prompt_snapshots`: one row, `generation_id = 'vivy-headless/1'`,
     composer `mask-prompt/1`, payload contains `builtin/writer`.
   - `session_mask_selections`: the session row `builtin/writer`, revision 1.

## Policy posture note

Under the repository-default governance profile (`default`), a write module
action has no policy rule, so the engine's default yields "prompt", and
module actions have no approval row — the UI surfaces
"module action is not authorized". This is the shipped policy semantics (the
masks e2e config documents it and uses `profile: full_auto` for the same
reason); the smoke therefore ran with `governance.profile: full_auto`,
matching `ui/playwright.masks.config.ts`.

## Follow-up (same day): default posture allow rule — owner approved

The owner approved allowing mask operations in the default posture:

- `go test ./internal/runtime/ -run TestPolicy -count=1` — pass, including
  new `TestPolicyToolPrefixRuleMatchesModuleActions` (trailing-`*` Tool
  prefix match; `vivy.masksx.*` namespace-boundary case does not match).
- `go test ./internal/config/ -run 'TestDefaultGovernance|TestGovernance'
  -count=1` — pass, including new `TestDefaultGovernanceAllowsMaskActionsInDefaultProfile`.
- `go test ./internal/config/... ./internal/runtime/...` — pass in full.
- `go test ./internal/app/ -run
  TestHeadlessFormArmsMasksAndAdmissionWithoutInjectedIdentity -count=1` —
  pass with `cfg.Governance = config.Default().Governance` (pure product
  default profile, no full_auto escape hatch).
- Browser re-smoke under the pure default posture (no config file): mask
  picker opens, switching the selection Writer → Researcher succeeds with
  zero error alerts, and the new selection survives a page reload.
- The full_auto e2e config comment (`ui/playwright.masks.config.ts`) remains
  accurate for deployments that pin stricter profiles.

## Not run

- `just ci` full gate (UI lane + plugin-ci): this delivery touches no UI or
  plugin source; the Go gates above plus the browser smoke cover the change.
  The vivy/masks-ui source-pin repair is covered by `sdk verify`.
- Packed-binary masks e2e (`ui/playwright.masks.config.ts`): requires a
  packed candidate; the packed path is unaffected by construction (pack
  replaces `zz_default.go` via overlay and never passes `WithFormIdentity`).
- PostgreSQL conformance: no schema or storage change in this delivery.
