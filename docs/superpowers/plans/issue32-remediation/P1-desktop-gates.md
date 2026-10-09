# P1 Desktop Authorization and Release Gates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. The owner authorized plan execution on 2026-10-09.

**Goal:** Close H1/H2/H3 with negative authorization, CI, and publication gates on the active desktop.

**Architecture:** Reuse the existing native capability and sealed SDK host. Keep candidate production separate from aggregate acceptance and publication. Extend existing guards while preserving transitional coverage.

**Tech Stack:** Go 1.26.4 floor, Wails `v3.0.0-beta.27`, Vue/TypeScript/Vitest, Python standard library, GitHub Actions, pnpm packageManager pin.

**Spec:** [Issue #32 remediation design](../../specs/2026-10-09-issue32-remediation-design.md#p1-desktop-authorization-and-release-gates).

## Global Constraints

- One `Service.Run`/Journal/policy path; DIVA imports VIVY only through `sdk/host/v1`.
- Use native `application.WindowKey`; JavaScript parameters or request headers are not a bound-method identity.
- CloseBudget is `5 * time.Second`; Wails module, CLI, and `@wailsio/runtime` stay `v3.0.0-beta.27` / `3.0.0-beta.27`.
- Pin full Git commits and SHA-256 digests; never accept a substring/prefix or dirty release source.
- Preserve Tauri/C ABI until approved W6 prerequisites; preserve historical v1 acceptance evidence.
- No real voice/Windows pass is inferred from scripted, headless, or source-level checks.
- All P1 implementation files below belong to the DIVA repository unless explicitly marked as a VIVY or read-only SDK dependency. In particular, H1's `internal/desktop/**` is DIVA code.
- Execution-order correction (2026-10-09): the canonical DIVA test wrapper currently stops in SDK consumer-module resolution because DIVA's host `go.mod` replacements under `./deps/laputa/*` are not redirected to the already locked Laputa snapshot. Repair this H2 packaging prerequisite in VIVY SDK first, with a focused SDK regression, then resume H1's canonical red/green sequence. This changes task order only; it does not broaden product acceptance.

## Review Focus

- Unauthorized empty-method calls still fail authorization before revealing validation or invoking the host: Task P1.1.
- A packaging-script or generated-binding-only change starts the same active-host CI: Task P1.2.
- Removing a canonical row or subcase cannot turn incomplete acceptance into a pass: Task P1.3.
- One platform's success cannot publish while another is pending, missing, or failed: Task P1.3.
- A signing, rename, rebuild, or evidence-source commit change cannot silently substitute accepted bytes: Task P1.3.

---

### Task P1.1: Share native main-window authorization across all bound methods (H1)

**Files:** DIVA — modify `internal/desktop/runtime_service.go` (`VivyCall`, `DesktopDispatch`, `MediaToken`); test `internal/desktop/runtime_service_test.go`. Reuse DIVA `internal/desktop/media.go` (`tokenFor`, `revoke`) without adding a second capability owner.

**Interfaces:** Consumes `(*mediaCapability).tokenFor(windowID uint) (string, bool)` and `application.WindowKey`. Produces proposed `(*RuntimeService).authorizeMainWindow(ctx context.Context) (string, *hostv1.Error)`. Preserve existing public signatures `VivyCall(context.Context, CallRequest) CallReply`, `DesktopDispatch(context.Context, DispatchRequest) CallReply`, and `MediaToken(context.Context) (string, error)`.

- [ ] **Step 1: Write failing authorization tests.** Add `TestVivyCallNativeAuthorization`, `TestPrivilegedMethodsRejectRevokedCapability`, and `TestAuthorizationPrecedesPayloadValidation`; update `TestVivyCallEnvelope` to bind capability ID `42` and use a context containing `fakeWindow{id: 42}`. Cases are missing identity, foreign ID `7`, nil capability, unbound capability, and revoked capability. Each unauthorized reply is `OK == false`, `Error.Kind == "closed"`, `Error.Code == -32081`, host `Call` count `0`, dispatch count `0`, and token empty/error present. For a valid main context assert one host call, unchanged JSON method/params, preserved typed upstream error, and honored timeout; valid empty method remains `invalid_input/-32602`.

```go
if calls != 0 || reply.OK || reply.Error.Kind != "closed" || reply.Error.Code != -32081 { t.Fatal(reply, calls) }
if token != "" || err == nil { t.Fatal("revoked identity received a token") }
```

- [ ] **Step 2: Run the canonical wrapper to prove failure.** `python scripts/build-desktop.py --mode test` stages tracked source and generates the resolved consumer.mod, then executes all Go race tests (including the new focused cases). Expected pre-fix failure: missing/foreign VivyCall dispatch increments the fake host call count. Do not replace this with plain `go test` against development-only local replacements.
- [ ] **Step 3: Implement the shared helper and invoke it first in all three methods.** Return the existing closed authorization envelope consistently, convert its typed error for `MediaToken`, and preserve authorized call/result semantics. Do not use the asset-server sender headers as authorization for Go bindings.
- [ ] **Step 4: Verify passing focused and canonical host gates.** Re-run the focused command, then `python scripts/build-desktop.py --mode test`; require passing race tests. Re-run `pnpm --dir agent-diva-gui test src/api/vivy/client.test.ts` to preserve the bound error envelope. No binding regeneration is needed for an unexported helper/publicly unchanged signatures.
- [ ] **Step 5: Commit the focused patch.** `git add internal/desktop/runtime_service.go internal/desktop/runtime_service_test.go` then `git commit -m "fix(desktop): authorize every privileged native binding"`.

### Task P1.2: Gate active Go host changes and sealed packaging (H2)

**Files:** DIVA — modify `.github/workflows/ci.yml`, `justfile`, `scripts/build-desktop.py`, and sole canonical `build/vivy-sources.lock.json` for target declarations; create `scripts/ci/check_desktop_boundary.py` and `scripts/ci/test_desktop_ci_contract.py`. Retain `scripts/ci/check_vivy_backend_boundary.py` for the transition. Test new `scripts/ci/test_desktop_boundary.py`; add negative fixtures under existing `scripts/ci/fixtures/legacy-calls/` only where the current guard lacks coverage. VIVY SDK prerequisite — modify `sdk/internal/go_host.go` and test `sdk/internal/go_host_test.go` so host-local `./deps/laputa/*` replacements resolve to the exact Laputa snapshot in the sealed closure.

**Interfaces:** Existing CLI `python scripts/build-desktop.py --mode {test,build,repin} --host-dir PATH --vivy-dir PATH --laputa-dir PATH --output PATH`. Proposed `--platform {linux-amd64,windows-amd64}` must equal the native build target. Add `derive_target_lock(source_lock: dict, platform: str) -> dict` and `canonical_lock_bytes(lock: dict) -> bytes`; serialization is sorted-key compact JSON plus one newline. `tools.platforms` declares Linux `{goos:linux,goarch:amd64,cgo:true,buildTags:[gtk3]}` and Windows `{goos:windows,goarch:amd64,cgo:true,buildTags:[]}`; common Go/Wails/Node/pnpm pins and all source pins stay canonical. Derivation replaces target fields only, records `tools.sourceLockSHA256` of canonical source-lock bytes, and adds mandatory `vivy_headless` only to the actual Go build invocation. Proposed `--mode check-lock --platform PLATFORM [--derived-lock PATH]` checks actual target/tool/source inputs and byte-compares any retained generated lock to this derivation. Proposed `mode_test` flags `--test-run REGEX` and `--test-packages PACKAGE [PACKAGE ...]` execute selected tests with SDK-produced `consumer.mod`; defaults remain `./...` and all tests. Preserve `inofy_pin(vivy_root: Path) -> dict`, resolving pinned module full `Origin.Hash` via `go mod download -json github.com/ProjectViVy/inofy@VERSION` and checking pseudo-version suffix agreement. Proposed boundary `check_desktop_boundary(root: Path, artifact: Path | None = None) -> list[str]`; CLI `python scripts/ci/check_desktop_boundary.py [--artifact DIR] --selftest` exits `1` on violations, `0` after all negative fixtures reject and clean tree passes. Preserve existing legacy guard CLI.

- [ ] **Step 1: Write failing SDK staging and CI contract tests.** First add a focused `sdk/internal/go_host_test.go` regression that gives a host a `replace github.com/dashimaki/garden => ./deps/laputa/garden` and proves the generated consumer modfile targets `../deps/laputa/garden` in the staged, locked closure. Add a `test_desktop_ci_contract.py` regression proving test mode provides a minimal `index.html` before SDK pack. Then write `test_go_only_paths_trigger_push_and_pull_request` (checks `cmd/diva/**`, `internal/desktop/**`, `internal/speech/**`, `go.mod`, `go.sum`, `build/**`, `scripts/**`, `justfile`, `agent-diva-gui/**`, `.github/workflows/**`), `test_aggregate_includes_native_race_bindings_boundary_and_seal` (canonical host test, frontend tests/build, binding drift, native boundary, legacy guard, sealed pack/inspect), `test_linux_and_windows_use_locked_sources` (sibling checkouts at canonical source-lock SHAs and packageManager-pinned pnpm/Wails), and `test_target_locks_derive_from_single_source_owner` (deterministic target derivation, common source/recipe/dependency fields, correct Linux/Windows GOOS/GOARCH/buildTags, canonical source change updates both outputs, edited generated pin fails drift check, requested/native target mismatch rejected). Boundary tests `test_rejects_vivy_internal_import`, `test_rejects_direct_native_frontend_call`, `test_rejects_browser_provider_fetch`, and `test_rejects_packaged_sidecar_or_second_runtime` assert concrete violations; authorized SDK/desktop seams pass.

```python
self.assertNotEqual(check_desktop_boundary(forbidden_tree), [])
self.assertEqual(check_desktop_boundary(clean_tree), [])
self.assertIn("go-test", aggregate_dependencies)
self.assertEqual(set(native_platforms), {"linux-amd64", "windows-amd64"})
```

- [ ] **Step 2: Run tests to prove both omissions.** Run the focused SDK test and `python -m unittest discover -s scripts/ci -p 'test_desktop_*.py' -v`; expected failures include the unredirected host Laputa replacement, omitted active-host paths/aggregate dependencies, and missing active boundary checker.
- [ ] **Step 3: Implement source closure, test-mode asset, trigger, aggregate, and native gates.** Make the SDK rewrite DIVA's `./deps/laputa/*` module replacements to the staged `../deps/laputa/*` snapshot and reject any path escaping the host/VIVY/locked Laputa closures. Test mode writes a minimal empty `index.html` before invoking SDK pack. Add Linux/Windows `go-host-check` jobs using the canonical source lock and deterministic target derivation; install Linux GTK3/WebKit4.1 and Windows CGO/race toolchain. Pin Node `24`, Go from final source lock (floor `1.26.4`), and Wails CLI beta.27. Add `desktop-bindings-check` (`just desktop-bindings` then generated `git diff --exit-code`), `desktop-boundary-check`, and `desktop-seal-check` recipes. Aggregate `ci` consumes `gui-test gui-build go-test desktop-bindings-check desktop-boundary-check desktop-seal-check shell-bridge-test` plus legacy selftests. `mode_build` derives target lock into scratch, supplies it to SDK pack, retains canonical `source-inputs.lock.json` and generated `input-lock.json` with the final artifact, and checks their deterministic relationship. `check-lock` rejects actual target/tools/sources that differ from declarations; check-drift rejects edited generated input. Test mode retains SDK-produced replacements. Keep Rust/Tauri transition gates required by W6.
- [ ] **Step 4: Verify passing CI and meaningful negative fixtures.** Re-run the unittest command and both guard selftests. `just ci` on the focused committed clean tree must include the Go race run, binding diff, and sealed Inspect. Exercise the CI contract fixtures for a desktop-only, speech-only, lock-only, and script-only diff; verify actual Linux/Windows workflow runs on the future PR, rather than calling YAML inspection a native run.
- [ ] **Step 5: Commit this gate package.** Stage only the listed CI/guard/fixture/build-wrapper files, then `git commit -m "ci(desktop): gate the active Go host and sealed package"`.

### Task P1.3: Bind complete acceptance to immutable candidates before publication (H3)

**Files:** DIVA — modify `scripts/ci/check_wails_candidate.py`, `.github/workflows/desktop-release.yml`; create `scripts/ci/test_wails_candidate.py`, `scripts/ci/test_desktop_release_contract.py`, and `docs/plans/diva-next/fixtures/wails-candidate-acceptance-v2.example.json`. Preserve historical `docs/plans/diva-next/fixtures/wails-candidate-acceptance.json`. Use existing W5/archival contracts in `docs/plans/diva-next/wails/{W5,W6,archive}.md`.

**Interfaces:** Proposed `validate_candidate(doc: dict, artifacts: dict[str, Path], release_source_sha: str | None, require_all_passed: bool) -> list[str]`; `validate_platform(platform: str, candidate: dict, artifact_dir: Path) -> list[str]`; `validate_rows(rows: list[dict], require_all_passed: bool) -> list[str]`. Proposed strict CLI `python scripts/ci/check_wails_candidate.py REPORT --artifact-root DIR --release-source-sha FULL_SHA --require-all-passed`; platform subdirs are `linux-amd64` and `windows-amd64`. Preserve v1 optional-report CLI for historical validation, but reject v1 under strict promotion.

**Acceptance v2 contract:** Root fields `schema`, `captured_at`, `platforms`, `rows`. Each platform records `sources.{host,vivy,laputa}.{commit,treeSHA256}`, `sources.inofy.{module,version,commit}`, `recipe.{path,sha256}`, `locks` (canonical source lock, deterministic derived target lock, source Go/pnpm, canonical consumer hashes), `generation_id`, `frontend_sha256`, `tools.{go,wails,node,pnpm,goos,goarch,cgo,build_tags}`, and `artifacts` keyed by retained relative filename with SHA-256. Include retained build-report, generation manifest, checksums, native inspection log/result, binary, and installer if produced. Per row record exact canonical `id`, `requirement`, `scenario`, and per-platform `results[{platform,outcome,owner,evidence:[{path,sha256}],subcases:[{id,outcome,evidence}]}]`. Only pending rows require an owner; every pass needs hash-verified evidence. `validate_platform` compares actual files and `generation.json.hostBuild`, verifies derived lock bytes equal canonical-lock/target derivation, and matches requested GOOS/GOARCH/CGO/build tags to report/artifact. Fully qualify INOFY's Git commit before freeze; baseline 12-character text does not satisfy a full-commit claim.

**Canonical row IDs:** `W5-T1-CANDIDATE`, `W5-T2-FRESH-PROFILE`, `W5-T2-FIRST-TURN`, `W5-T2-GRANTS-CATALOG`, `W5-T3-CHAT-MATRIX`, `W5-T3-COGNITIVE`, `W5-T4-SINGLE-INSTANCE`, `W5-T4-CRASH-RESTART`, `W5-T4-CLEAN-QUIT`, `W5-T4-HIDE-REOPEN`, `W5-T4-WEBVIEW-RELOAD`, `W5-T4-EVENT-LOSS`, `W5-T5-LOGS`, `W5-T5-VOICE-SCRIPTED`, `W5-T5-VOICE-REAL`, `W5-T5-VOICE-LIFECYCLE`, `W5-WINDOWS-MATRIX`. Reject omissions, duplicates, extra unknown IDs, and inconsistent requirement/scenario mappings. Pin subcases from W5, including chat images/preset readback/approval accept/reject/cancel/edit/regenerate/rewind conflict/goal/session reopen; event overflow/active timeout/shutdown deadline; and real SiliconFlow STT/SiliconFlow TTS/MiniMax TTS with microphone/playback.

- [ ] **Step 1: Write failing checker and workflow tests.** Cases `test_missing_required_row_rejected`, `test_unknown_row_cannot_substitute`, `test_missing_required_subcase_rejected`, `test_pending_or_failed_platform_blocks_all_publication`, `test_strict_requires_reports_binaries_and_all_platforms`, `test_full_sha_not_prefix`, `test_dirty_vivy_or_laputa_rejected`, `test_recipe_generation_frontend_lock_tool_platform_mismatch`, `test_binary_or_installer_tamper_rejected`, `test_stale_evidence_hash_rejected`, `test_tag_peel_must_equal_accepted_host`, and `test_historical_v1_cannot_promote`. Workflow test `test_publication_depends_on_aggregate_acceptance_and_downloads_same_bytes` asserts publish `needs` the all-platform gate, candidate jobs have `contents: read`, publish contains no build/pack/sign step or `--clobber`, and one missing leg yields no release-upload invocation.

```python
self.assertNotEqual(validate_candidate(incomplete, artifact_dirs, host_sha, True), [])
self.assertEqual(validate_candidate(complete_exact_candidate, artifact_dirs, host_sha, True), [])
self.assertEqual(public_upload_calls, 0)  # pending Windows or one changed installer byte
```

- [ ] **Step 2: Run focused tests to prove failure.** `python -m unittest discover -s scripts/ci -p 'test_wails_candidate.py' -v` and `python -m unittest discover -s scripts/ci -p 'test_desktop_release_contract.py' -v`; expect current checker/workflow to admit omitted rows, prefix pins, or ungated publication.
- [ ] **Step 3: Implement strict checker and separate candidate/acceptance/publish jobs.** Candidate workflow builds each platform once, freezes final signing/packaging bytes and internal Actions artifacts, and emits no public release. Native runners inspect their own platform artifact with pinned VIVY `go run ./sdk inspect-artifact DIR` and capture hashed results. Aggregate job downloads both complete retained artifacts plus external acceptance, validates all rows/identities/hashes, verifies paired annotated archive tags and the selected approved W6 state, then emits a passing gate. Publish job only downloads that accepted artifact set, revalidates its hashes and tag peel, and uploads without overwrite. Historical fixtures never become default publication input.
- [ ] **Step 4: Verify passing and blocked publication paths.** Re-run both suites. Run strict checker on a complete synthetic fixture (PASS), then pending voice/Windows, missing row/platform/report, dirty VIVY, same-prefix-different-SHA, swapped generation/frontend/lock/tool, and tampered installer cases (each exit `1`). The current historical fixture remains reported `15 passed / 2 pending / 0 failed` and strict promotion must fail; no status is upgraded by this implementation.
- [ ] **Step 5: Commit checker/workflow/example schema.** Stage only the listed files and `git commit -m "fix(release): require exact all-platform candidate acceptance"`.

**P1 exit gate:** Frozen-input prerequisite and three focused repairs plus active-host CI and negative release simulations pass. Product/native acceptance is pending until P7; no public release is produced by P1.
