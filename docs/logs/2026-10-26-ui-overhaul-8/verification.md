# Block 8 verification

Host: macOS arm64 (Apple Silicon Mac mini), worktree
`.worktrees/ui-overhaul/agent-vivy`, branch `feat/ui-overhaul`; submodule
`studio/` on branch `ui/skin-alignment`.

## Static gates

- `node studio/dsh-vivy-studio/check-tokens.mjs`
  - baseline at `ac55322`: **red**, 29 official tokens missing (upstream sheet
    grew after the skin was written).
  - after the fold: `ok: 117 official alias/specific tokens covered`.
  - The check resolves `../../../.workspace/deepseek-harness/upstream/...`;
    the reference file was mirrored from the root tree's gitignored
    `.workspace/` into `.worktrees/ui-overhaul/.workspace/` (scratch, not
    committed).
- Token equivalence proof (ad-hoc resolver, both themes): parsed old and new
  `theme.css`, resolved every `var()` chain, compared all 178 old `--dsw-*`
  declarations light+dark: **0 value diffs, 0 missing**. The fold is
  render-identical for every pre-existing token; the only intentional visual
  delta is the wordmark snapping 13.5px → 13px onto the type scale.
- Hub package (`studio/dsh-plugin-hub`, cloned `node_modules` from the root
  tree via APFS clonefile):
  - `npm run build` — server `tsc` + client `tsdown` OK
    (`client/client.js` 356 kB).
  - `npm run typecheck` — client/server/test configs clean.
  - `npm test` — 56/56 pass.
- Dead-class removal verified by re-running the unreferenced-class scan:
  remaining "hits" are only the dynamically composed LogsView badge/category
  classes and comment-text false positives.
- CSS brace balance checked per edited stylesheet (open == close).

## Runtime / visual gates

`just studio` on this host is a PowerShell-quoted recipe
(`& "{{go}}" build -o vivy-studio.exe ./cmd/vivy-studio`) and cannot run under
`sh`; this is pre-existing platform skew, not caused by this change. The
equivalent commands were run directly:

- `node scripts/ensure-laputa.mjs --quiet` then
  `GOTOOLCHAIN=go1.26.4 go build -o /tmp/vivy-studio.exe ./cmd/vivy-studio` —
  BUILD-OK. (`cmd/vivy-studio` is the lifecycle CLI, not the web server.)
- The Studio web surface is the `dsh` profile server. Evidence pair:
  - OLD = the already-running owner-profile server on `:3090`
    (`DSH_HOME=<root>/data/studio-home`, skin from the root tree at
    `ac55322`).
  - NEW = isolated instance on `:3091`: `DSH_HOME=/tmp/vivy-studio-overhaul`
    (clonefile copy of studio-home), profile `node_modules/dsh-vivy-studio`
    and `dsh-plugin` replaced with the worktree skin and the rebuilt hub,
    booted with `--no-open`. The owner's `:3090` instance was never touched or
    restarted.
- Playwright (system Chrome channel) captured old/new × light/dark ×
  {shell, plugin page, install modal} = 12 screenshots in `shots/`;
  **0 page errors** on both instances.
- Cascade regression found and fixed during capture: with the mapping block
  declared only on `html, body`, DSH's own
  `body[data-ds-dark-theme]` declarations out-ranked the Vivy sheet in dark
  (computed `--dsw-alias-button-primary-fill` = `#f9fafb`, install button
  rendered near-white/grey). After adding `body[data-ds-dark-theme]` to the
  mapping selector list, a computed-style probe on the dark modal reports
  `--hub-brand` / `--hub-btn-fill` / `--dsw-alias-button-primary-fill` =
  `#5fd4cf` (Vivy dark fluorite) on both button and body.
- Light old-vs-new screenshots are visually identical (sheet values
  unchanged); dark old-vs-new differ only where intended (hub dark now follows
  the Vivy dark palette instead of the fork's grey wash; danger stays red).

## Not run (recorded)

- Windows full-product `just ci` and the Go kernel gates
  (`just wiring-ci` / `just kernel-ci`): not runnable on this macOS host;
  unchanged scope (CSS-only in the submodule).
- `just studio` recipe verbatim (PowerShell syntax); equivalent commands run
  instead, see above.
- The isolated `:3091` instance and `/tmp/vivy-studio-overhaul` are scratch
  evidence; the instance is stopped and the clone removed after capture.
