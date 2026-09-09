# 2026-08-26 — Tool polish: read_file line numbers, echo_info off by default, network_search config surface

Branch: `feat/tool-polish` (worktree `../agent-vivy-tool-polish`, per
`parallel-worktree-isolation` — the root tree hosted an active `list_dir`
lane while this work ran). One focused commit on this branch; landing back
to `main` is via merge/PR, not by piling files into the root tree.

## What changed

1. **`read_file` output carries line numbers.** Text `content` is now
   prefixed per line with its 1-based file line number (`"N\tline"`,
   cat -n style), starting at the requested `start_line` (1 when unset).
   Binary payloads and empty ranges are untouched; `bytes`/`total_lines`
   still describe the raw file. Spec description documents the format so
   the model can anchor `patch` and range reads on exact lines.
   Implemented in the tools layer (`internal/tools/filesystem.go`), so the
   runtime backend contract is unchanged.

2. **`echo_info` off the default-enabled list.** Removed from
   `config.Default().Tools.Enabled` and from `config.example.yaml`. The
   tool stays registered (`BuiltinWithCommands`) for verification/tests;
   `config.example.yaml` documents how to re-enable it. Explicit
   `tools.enabled` lists that include `echo_info` keep working (e2e
   bootstrap unchanged).

3. **`network_search` degraded-experience documentation.**
   - Tool spec description now names each provider's requirement
     (`BING_SEARCH_API_KEY`, `GOOGLE_SEARCH_API_KEY`+`GOOGLE_SEARCH_CX`,
     `SEARXNG_SEARCH_URL`, keyless duckduckgo/wikipedia) and states the
     no-key fallback behavior explicitly.
   - `config.example.yaml` gained a comment block under `tools:` covering
     the three env vars, the automatic DuckDuckGo/Wikipedia degradation,
     and the new `tools.network_search.provider` key.

4. **`network_search` preferred provider, end to end.**
   - Config: new optional `tools.network_search.provider` ("" = auto;
     allowlist-validated in `Validate()`), carried through the custom
     `Tools.UnmarshalYAML` mirror.
   - Runtime: `NetworkSearchService.SetPreferredProvider` — an
     unqualified request uses the preferred provider when it is usable,
     and silently degrades to the existing keyless walk when it is not
     (e.g. bing without a key). Explicit `provider` arguments still win.
   - Settings: `data/<data-root>/settings.yaml` gained
     `network_search.provider`, validated with the same allowlist, and the
     app settings overlay applies it over the config value at startup.
   - RPC: `settings/get` / `settings/update` now carry a
     `network_search` section — saved provider, config default, and a
     per-provider availability roster (keyless / configured / env var
     name; presence only, never values, D-010). `settings/update` keeps
     whole-document replace semantics.
   - UI: the Settings → Tools tab gained a real (non-demo) Network Search card —
     provider select with automatic fallback text, availability badges naming
     the env var to set, and saving into the real settings document. All
     `saveSettings` call sites (SettingsView model form, WelcomeWizard,
     MaskAndModelSwitcher) now send the full document including the
     network_search preference so no save path wipes it.

## Scope boundaries / explicitly not done

- No searxng URL editing from the UI (env-only, `SEARXNG_SEARCH_URL`);
  the card documents the env var instead.
- No changes to tool registry membership beyond the default list;
  `echo_info` stays registered.
- No live search executed against real providers in tests (deterministic
  httptest servers only).
- Lane isolation: root-tree `list_dir` work untouched; merge order and
  conflict resolution on `filesystem.go`/`config.go`/
  `config.example.yaml` is left to the landing merge.

## Verification

See `verification.md`. Gate: `just ci` green (after one D-010 audit fix,
see notes). Browser smoke on the real split pair documented in
`verification.md` and `acceptance.md`.
