# Acceptance: what a person should see

## 1. The group is assembled, not hard-coded

```text
just dev                 # backend 127.0.0.1:8787 + Vite 127.0.0.1:3015
```

Open `http://127.0.0.1:3015`, click **VIVY** in the sidebar. The group lists, in
order:

```text
人格 | 面具 | 进化 | 记忆 | 记事本
```

Open each one. The page renders **inside** the app frame: the sidebar, session
list, composer, and the VIVY group stay visible, and the URL is
`/persona`, `/masks`, `/evolution`, `/memory`, `/notebook`. Copy is localized
(Chinese in the zh UI); no raw `plugin.vivy/...` key and no
`[missing translation: ...]` ever appears.

## 2. A Module can be unselected

Edit `recipes/default.vivy.yml` and remove one Module from all three places
(`modules`, the `order` list, `ui.extensions`), then run
`cd ui; pnpm run stage:ui` (or any of `dev`/`build`/`typecheck`/`test`, which
stage automatically).

- its sidebar entry disappears,
- its page is gone from the bundle (the staged tree under
  `ui/src/generated/ui/` no longer contains it),
- its path falls back to the app root instead of rendering a Module page,
- 面具 is still there, because it is core.

`minimal.vivy.yml` is the extreme case of the same rule: no UI Module at all,
no UI projection, and a packed Generation that boots.

## 3. It is the same Generation the SDK packs

```text
go run ./sdk pack --recipe recipes/default.vivy.yml --output .workspace/pack-default
go run ./sdk inspect-artifact .workspace/pack-default
```

The manifest lists the four UI Modules, four sealed catalogs
(`vivy/persona`, `vivy/evolution`, `vivy/memory`, `vivy/notebook`), four source
hashes, and no UI root. The same projection is what the embedded UI serves:
`pnpm build` bundles it (checked in a browser at the embedded build, where the
VIVY group lists all five entries and 进化 / 记忆 render inside the frame), and
`pack` builds that bundle into the artifact whose manifest `inspect-artifact`
prints.

## 4. Known non-goals a reviewer will notice

- The demo pages still say "演示 / 本地模拟": they remain local demo data
  (`vivy.demo.*`), only their code moved into Modules.
- Change the Recipe and forget to restage: the next `dev`/`build`/`typecheck`/
  `test` restages for you, so the checkout always builds the Recipe it ships.
- The embedded Playwright suite is red for a pre-existing reason unrelated to
  this change (its specs assume browser-locale detection that `d9ba447` removed,
  so their Chinese expectations fail against the `en` that a fresh e2e database
  hydrates). See `EMBEDDED-E2E-LOCALE` in `docs/TODO.md` and `verification.md`.