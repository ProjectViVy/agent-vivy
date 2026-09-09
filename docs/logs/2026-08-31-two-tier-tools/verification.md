# Verification record — 2026-08-31 two-tier tool system

Environment: worktree `../agent-vivy-two-tier-tools` (branch `feat/two-tier-tools`,
baseline e983b49). Every commit required `just ci` to pass; the first two commits were each
verified before the next was created.

## Commands and results

| Step | Command | Result |
|---|---|---|
| Commit 1 | `just ci` (worktree; run `pnpm build` first for `go:embed`) | all green (fmt-check / vet / go test ./... / headless-compile / UI typecheck+test+build) |
| Commit 2 | `just ci` | all green (CI_EXIT=0) |
| Commit 3 targeted | `go test ./internal/runtime/ -run 'TestEngineHiddenTools\|TestSelectToolsBinds\|TestToolSurface\|TestToolAdapterAllowsMounted\|TestEinoSkillBackendDeclaredTools\|TestMountedTools\|TestSkillView'` | ok |
| Commit 3 full | `just ci` | The first run caught unformatted `skills_backend.go` in fmt-check (fixed with gofmt -w); the rerun passed (JUST_CI_EXIT=0, see below) |

> Note: the first ci run failed in vet/`go:embed` because `ui/dist` was missing—a known cold-start
> issue in a worktree; running `pnpm build` first made it all green (consistent with TODO §0.1 UI-CI-BOOTSTRAP).

## Representative assertions added/rewritten

- `TestServiceRunBindsToolsOnKeywordlessRequest`: the `model.request.selected_tools` value for a keywordless
  message ("What tools do you have now?") is non-empty—the assertion necessarily failed under the old selector and is the regression gate for this fix.
- `TestSelectToolsBindsFullActiveSurface` / `TestEngineHiddenToolsStayOutOfActiveSurface`：
  full binding in registration order; hidden tools enter the executable universe but not the active surface.
- `TestToolSurfaceMiddlewareFiltersViewModel` / `...ToleratesMissingMountsAndNilInfos`：
  view filtering (active ∪ mounted ∪ foreign) and the empty state.
- `TestToolAdapterAllowsMountedTool`: the execution gate allows a mounted tool.
- `TestEinoSkillBackendDeclaredTools`: frontmatter `tools:` is parsed into
  `SkillSummary.Tools`, and `SetSkillEnabled` preserves the declaration when re-rendering.
- `TestSaveAndLoadToolsEnabledOverlay` / `TestValidateToolsEnabledOverlay`:
  the overlay's three states (default/list/empty table) and structural validation.
- `TestToolsCatalogListAndSetActive`: the complete tools/list + set-active path
  (overlay write, `OnSettingsChanged` trigger, rejection of unknown/duplicate names, empty table = pure chat).

## Real-path smoke test (completed, worktree 8790 embedded new UI)

Environment constraint and workaround: port 3015 was occupied by Vite (`--strictPort`) from another lane in the root tree,
and 8787 was occupied by its backend with a single active organism lease. The smoke backend therefore used `127.0.0.1:8790`
(the worktree's own scratch data/, without touching the production journal), using the worktree's **embedded new UI** from `ui/dist`
(built by just ci and containing all changes from this iteration).

| # | Step | Result |
|---|---|---|
| 1 | Start the smoke backend (new code) | `healthz ok`, `vivy starting addr=127.0.0.1:8790` |
| 2 | Open Settings → Tools in the browser (IAB) | Real catalog cards rendered: all built-in tools, read-only/approval-required badges, and toggle states matching the config defaults (2 tools); the notice said "No override written"—`tools/list` completed the real UI + RPC path |
| 3 | Turn off the `write_note` toggle → save tool configuration | `tools/set-active` wrote to worktree `data/settings.yaml`: `tools_enabled: [echo_info]` |
| 4 | Refresh the page and reopen the Tools page | `echo_info=checked / write_note=unchecked`; the notice said "Currently using a custom override"—persistence and display were correct |
| 5 | Send "What tools do you have now? Can you see what is in the workspace?" in a new session | As expected, the run failed when it reached the model request (this environment has no provider key; "Unable to connect! Check the provider configuration!" is the established error after mock removal); **journal `model.request` recorded `"selected_tools":["echo_info"]`**—the keywordless message bound a non-empty active surface and obeyed the overlay; under the old keyword selector this field would have been `[]` |

Not covered in the browser: mounting through `skill_view` in the same run requires a real model-driven ReAct loop
(no key is available in this environment), so unit tests cover it (middleware view filtering, adapter admission,
mount records, frontmatter parsing, and preservation during re-rendering). Real model replies and the user-visible SKILL-mount acceptance
must be performed on a normal instance with a key, following acceptance.md.

Cleanup: the smoke backend was stopped, the temporary config port was restored to 8787, and the browser tab was closed;
the worktree `data/` is per-checkout scratch and was left unchanged.
