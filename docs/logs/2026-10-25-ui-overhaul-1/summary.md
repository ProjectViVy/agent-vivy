# Summary — UI overhaul Block 1: theme foundation and families

Block 1 of the staged UI overhaul (`feat/ui-overhaul`). It replaces three
independent sources of theme truth with one authored CSS source, one pre-paint
path, and a family registry that holds no colour values — then rebuilds the
Vivy / Diva / Miku skins on that foundation and adds three families.

## What changed

### 1. `ui/src/styles.css` — the only authored token source

The four complete `[data-theme]` token blocks (two oklch, two hex) are gone.
The file is now:

1. **One neutral ramp per appearance.** `:root, [data-theme-preview]` declares
   the light ramp; `[data-appearance="dark"]` (later in the file, same
   specificity) replaces it with the dark ramp. Every surface, text and line
   value is expressed as `oklch(lightness chroma calc(var(--tint-c) × factor)
   var(--tint-h))`, so the ramp's temperature comes from two numbers.
2. **Family deltas.** Seven blocks — `default`, `dark`, `pink`, `miku`, `rice`,
   `fluor`, `indigo` — declare only `--tint-h`, `--tint-c` and the brand ramp
   (`--brand`, `--brand-hover`, `--brand-soft`, `--brand-fg`). Adding a family
   is now one short block, not a 60-line palette.
3. **Preview scope.** Each family block is also declared for
   `[data-theme-preview="…"]`, which the settings picker sets together with
   `data-appearance` on a tile. Preview tiles therefore render the real tokens
   of *every* family simultaneously, including the ones that are not active.
4. **Type scale and geometry.** `--type-2xs` 11px / `-meta` 12 / `-ui` 13 /
   `-read` 14 / `-title` 16 / `-page` 20, with `1.25 / 1.5 / 1.7` line heights;
   `--control-sm/md/lg` 28/32/40px; `--hit` 44px; `--row` 32px;
   `--content-max` 52rem; `--sidebar-w` 18rem; `--radius` unchanged.
   Exposed as Tailwind utilities (`text-2xs` … `text-page`, `max-w-content`,
   `bg-canvas`, `bg-raised`, `border-line`, `bg-ok-soft`, …) through
   `@theme inline`, so a call site names a role, not a colour.
5. **Semantic token set.** `bg-app / bg-surface / bg-raised / bg-sunken /
   bg-hover / bg-selected / scrim`; `fg / fg-muted / fg-subtle / fg-disabled`;
   `line / line-strong / ring`; `brand / brand-hover / brand-soft / brand-fg`;
   `ok | warn | danger | info`, each with `-soft` and `-fg`. Status tokens are
   deliberately identical across families: a warning must not change meaning
   when the user switches skin.
6. **Legacy aliases are derived, not duplicated.** The shadcn names
   (`--color-background`, `--color-card`, `--color-primary`, `--color-muted-
   foreground`, …) now point at the semantic tokens. ~100 components keep
   rendering through them while Blocks 2–6 migrate call sites and delete the
   aliases. There is no second palette to keep in sync.

### 2. `ui/index.html` — one pre-paint path

From 39 changed lines of per-theme FOUC CSS plus a duplicated id/appearance map
to: root sizing, **two** neutral colour pairs (`#f0f8fd` / `#253035` light,
`#0c1418` / `#ecf3f6` dark — the Vivy ramp's endpoints), and a script that reads
`vivy.appearance`, sets `data-appearance`, and toggles `.dark`. It contains no
theme id, no `oklch()`, no per-family hex.

### 3. `ui/src/hooks/use-theme.ts` — registry, no colours

Seven entries of `{ id, appearance }`; storage ids `default|dark|pink|miku` are
unchanged so existing installs migrate for free, and `rice|fluor|indigo` are
new. `applyThemeToDOM` writes `data-theme`, `data-appearance` and `.dark` — and
nothing else, so module themes registered through
`plugins/presentation-host.tsx` (which write inline variables) still win. It
persists both `vivy.theme` and `vivy.appearance`; on module load it also writes
the appearance, so an install that predates this block gets migrated on its
first load and is flash-free from the second reload onward.

### 4. `ui/src/components/settings/ThemePicker.tsx` — real previews

The hand-maintained gradient + accent literals are gone. Each tile carries
`data-theme-preview={id}` and `data-appearance`, shows a brand chip (`Aa`), a
raised sample card with three text bars, and the four status dots — all in that
family's tokens. Selection is `border-brand` + `ring-brand/20`; the check icon
and the card header use `brand` / `brand-soft`.

### 5. i18n

`themes.{default,dark,pink,miku,rice,fluor,indigo}.{label,description}` in both
`en.ts` and `zh.ts`. Display names: Vivy Light / Vivy Dark / Diva / Miku /
Rice & Vermilion / Graphite & Fluor / Night Indigo (zh: Vivy 浅色 / Vivy 深色 /
Diva / 初音未来 / 宣纸朱砂 / 石墨荧光 / 夜蓝靛). The picker's sample glyph is
`settings.themeSwatchSample` (`Aa` / `字`) rather than a bare JSX string, and the
seven new theme keys plus that one are classified in
`scripts/i18n-cross-face-contract.json`. Two i18n gates were already failing at
the parent commit for unrelated reasons; that is recorded as `I18N-CHECK-RED`
in `docs/TODO.md` instead of being silently absorbed.

### 6. Verification tooling

- `ui/src/theme/sources.test.ts` (new, 7 tests) enforces the ownership rules:
  `index.html` has exactly two neutral pairs and knows appearances but no theme
  ids; it reads `vivy.appearance`, sets `data-appearance`, toggles `.dark`, and
  carries no `oklch()` or custom property; `styles.css` has no hex; every family
  is declared for both the applied and preview scope; the registry has no
  colour literal.
- `ui/scripts/audit/contrast.mjs` (new) reads the real CSS and registry,
  resolves `var()` chains and nested `calc()`, composites alpha, and checks 21
  foreground/background pairs per family. It fails on a WCAG AA miss or an
  out-of-sRGB token (a value the browser would silently clamp).
- `ui/scripts/audit/screenshots.mjs` now derives its family list from the
  registry instead of a hardcoded array, seeds both storage keys so a shot
  cannot hide a flash, and has one scrolled surface (`theme-picker`, via
  `data-testid`) so the picker is photographed at every width.
- `ui/src/plugins/presentation-host.test.tsx` gained a test that a module theme
  variable (`--brand`) keeps winning while the kernel switches families.
- `ui/src/hooks/use-theme.test.ts` covers `data-appearance`, legacy id
  migration, persistence of both keys, and that the root element gets no inline
  style.

## Ratchet movement

| Group | Before | After |
| --- | --- | --- |
| `ui/src` hex | 21 in 2 files | **5 in 1 file** |
| `ui/src` palette | 159 in 31 files | 159 in 31 files (unchanged) |
| `ui/src` `text-[…]` | 85 in 26 files | 85 in 26 files (unchanged) |
| `plugins/*/ui` | 29 / 45 / 0 | unchanged |

The 16 removed hex literals are the preview gradients and accent colours in
`use-theme.ts`. The five that remain are all in `components/ui/chart.tsx`, a
zero-import primitive deleted in Block 2.

## Deviations from the approved plan

- **Utility names.** The plan wrote `bg-bg-raised`, `text-fg-muted`. Shipping
  `bg-canvas`, `bg-surface`, `bg-raised`, `bg-sunken`, `bg-hover`,
  `bg-selected`, `text-fg`, `text-fg-muted`, `text-fg-subtle`, `border-line`,
  `bg-brand`, `text-brand-fg`, `bg-ok-soft`, `max-w-content` — the token names
  keep their `bg-` prefix, the utility does not repeat it. Same contract, six
  fewer characters per call site.
- **`--fg-on-brand` dropped.** The plan listed both `--fg-on-brand` and
  `--brand-fg`; only `--brand-fg` exists (one name per role).
- **Dark is an appearance, not a family.** The plan's phrasing allowed
  `[data-appearance="dark"]` as the neutral ramp; the `dark` storage id remains
  a family entry whose delta only restates the brand ramp. A user picking
  "Vivy Dark" gets the dark ramp plus the dark-brand delta, which is exactly the
  old `dark` skin, re-tuned.
- **Init-time appearance persistence** is new relative to the plan text: it is
  what makes the legacy-install migration invisible from the second load
  onward.
- **`theme-picker` audit surface.** Block 0's audit had 10 surfaces; this block
  adds an 11th (settings scrolled to the picker) because the picker is the
  block's product surface. 231 shots instead of 210.

## Scope and what was explicitly not done

- No chat, composer, navigation, page or plugin component was migrated. Stock
  palette classes and arbitrary text sizes are still where Block 0 measured
  them; Blocks 2–6 remove them and the ratchet follows.
- The legacy shadcn token aliases remain on purpose until their last call site
  is migrated; the same applies to `dark:` colour pairs in components.
- No Studio shell change (Block 8) and no plugin UI module change (Block 7).
- `ui/src/generated/**` was regenerated only by the existing `stage-ui-assembly`
  pre-hooks and is unchanged in this commit.
