# Block 2 verification

All commands run from the worktree root unless a `cd` is shown, on macOS arm64,
Node v24.18.0, pnpm 12.10.1, `GOTOOLCHAIN=go1.26.4` for every UI script that
runs the `stage-ui-assembly` pre-hook.

## Typecheck

```
cd ui && GOTOOLCHAIN=go1.26.4 pnpm typecheck
$ node scripts/stage-ui-assembly.mjs
{"recipe":".../recipes/default.vivy.yml", ... "extensions":[7 modules]}
$ tsc --noEmit
```

PASS (no output after the script banner). The first run failed on
`ui-build-provenance.test.ts` and `ui-sdk-face-compat.test.ts` because
`sessionDrawerOpen` / `setSessionDrawerOpen` still existed in
`sdk/ui/src/module.ts`; after removing them from `FaceStoreState` the same
command is clean. That failure is the contract check doing its job, not a test
adjustment.

## Unit tests

Known environment red first, then the workaround used to get a real signal:

```
cd ui && GOTOOLCHAIN=go1.26.4 pnpm test        # without a localStorage binding
→ 45 failed / 9 failed files, all "localStorage.clear is not a function"
```

That is `UI-VITEST-NODE25`, reproduced identically by
`src/components/chat/session-list-view.test.ts` on files this block never
touched. With Node's per-process storage enabled (a local workaround only; no
repository change):

```
cd ui && rm -f /tmp/vitest-ls.db \
  && NODE_OPTIONS="--localstorage-file=/tmp/vitest-ls.db" GOTOOLCHAIN=go1.26.4 pnpm test
→ Test Files 1 failed | 87 passed (88)
   Tests      1 failed | 708 passed (709)
```

The single failure is pre-existing and unrelated:

```
FAIL src/lib/store.test.ts > Vivy store integrity > keeps the run.failed payload as the conversation error
Expected: "Unable to connect! Check your provider configuration!"
Received: "provider deepseek: API key missing"
```

Proved pre-existing by restoring `HEAD`'s `store.ts` only
(`git checkout HEAD -- ui/src/lib/store.ts`, run, restore), which fails the
same assertion at `49544156`. Recorded as `STORE-RUN-FAILED-COPY` in
`docs/TODO.md` §0.1.

Focused re-runs:

```
cd ui && NODE_OPTIONS="--localstorage-file=/tmp/vitest-ls.db" npx vitest run \
  src/components/chat/ConversationSidebar.test.tsx
→ 6 passed
```

`ConversationSidebar.test.tsx` needed one mechanical change: Radix opens the
menu on `pointerdown`, which happy-dom's `.click()` does not emit, so the test
dispatches `new PointerEvent('pointerdown', { bubbles: true, button: 0 })`.
All assertions (role `menu` / `menuitemradio`, "Flat list", persistence of
`SESSION_LIST_VIEW_KEY`, folder dialog behaviour) are unchanged.

`HistoryPicker.test.tsx`, `i18n/surfaces.test.tsx` and `theme/tokens.test.ts`
pass unchanged against the shared row and the trimmed primitive set.

## Token ratchet

```
[token-ratchet] src/palette: 158 (baseline 158); 30 files (baseline 30)
[token-ratchet] src/textSize: 81 (baseline 81); 24 files (baseline 24)
[token-ratchet] src/hex: 0 (baseline 0); 0 files (baseline 0)
[token-ratchet] plugins/palette: 29 (baseline 29); 7 files (baseline 7)
[token-ratchet] plugins/textSize: 45 (baseline 45); 8 files (baseline 8)
[token-ratchet] plugins/hex: 0 (baseline 0); 0 files (baseline 0)
```

## Build

```
cd ui && GOTOOLCHAIN=go1.26.4 pnpm build
dist/assets/index-*.css   125.84 kB │ gzip: 20.82 kB
dist/assets/index-*.js  1,844.66 kB │ gzip: 551.59 kB
✓ built in 1.64s
```

## Screenshot audit

Split pair running in this worktree at `http://127.0.0.1:3016` (Vite) against
the isolated backend on `127.0.0.1:8796`; the owner's `:3015` / `:8787` pair
was not touched.

```
cd ui && node scripts/audit/screenshots.mjs \
  --base-url http://127.0.0.1:3016 --out ../docs/logs/2026-10-25-ui-overhaul-2/audit
→ audit: 231 screenshots, 0 failures
```

7 families × 11 surfaces × 3 widths. The full set was inspected for failures
via the manifest and two frames were kept as evidence
(`chat-empty-default-390.png`, `chat-empty-default-1280.png`); the remaining
231 files were removed to keep the log small.

## Browser interaction pass

Playwright against the same pair (`docs/logs/2026-10-25-ui-overhaul-2/interaction/`):

| Shot | What it proves |
| --- | --- |
| `sidebar-session-row-1280.png` | one session row renders the created session |
| `sidebar-session-row-hover-1280.png` | hover reveals edit/delete on the shared row |
| `sidebar-view-menu-1280.png` | Radix radio menu, portal-positioned, dot on the active mode |
| `sidebar-flat-list-1280.png` | choosing "Flat list" drops the workspace group row |
| `sidebar-rename-result-1280.png` | in-place rename saves through `onRenameSession` |
| `mobile-nav-sheet-390.png` | mobile nav sheet opens at 390px |
| `workspace-dialog-390.png` | workspace dialog fits 390px with a scrolling body |

## Dead-symbol grep

```
rg '\b(SessionDrawer|ConversationModePlaceholder|sessionDrawerOpen|setSessionDrawerOpen)\b' \
  ui/src plugins sdk/ui/src --glob '!ui/src/generated/**'
→ 0 matches
```

Residual matches for other removed names are unrelated: `Calendar` is a
`lucide-react` icon in `plugins/vivy-notebook`, `RadioGroup` is the
`DropdownMenu` primitive, and `Sidebar` only appears in i18n copy and type
names.

```
rg 'role="(menu|listbox|menuitemradio|menuitem|option)"' ui/src plugins \
  --glob '!generated/**' --glob '!*.test.tsx'
→ ComposerCommands.tsx:185 role="listbox", :186 role="option"   (combobox popup)
```

## i18n gates (pre-existing red, unchanged)

```
node scripts/check-i18n-completeness.js
→ Uncatalogued Han: ui/src/lib/failure.ts:56,57,58
node scripts/check-i18n-cross-face.js
→ Unclassified Web key: chatInput.sendNow
```

Identical to the baseline recorded in
`docs/logs/2026-10-25-ui-overhaul-1/verification.md` and on the board as
`I18N-CHECK-RED`; this block adds no new key.

## Not run

- `sdk/ui` own vitest lane (`sdk/ui/src/module.test.ts`): that package has no
  installed `node_modules` in this checkout and no justfile/CI recipe invokes
  it. The SDK surface this block edited is covered by
  `ui/src/lib/ui-sdk-face-compat.test.ts` and
  `ui/src/lib/ui-build-provenance.test.ts`, both green in the full run.
- `just ci` from the repo root: the Windows/embedded gate is not this host's
  lane; the local substitute above is typecheck + full vitest + build.
- `pnpm e2e`: no Playwright e2e suite targets the deleted primitives; the
  browser pass above covers the affected surfaces in the real app.
