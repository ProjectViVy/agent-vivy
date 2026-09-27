# Verification: MEM-1C memory UI management surface

Environment: branch `feat/memory`, 2026-09-27, executed in the lead session.

## Gates

```text
cd ui && pnpm run stage:ui          # green (module source-hash verified)
cd ui && pnpm typecheck             # tsc --noEmit clean
cd ui && pnpm test                  # 50 files / 409 tests passed
```

i18n parity: 33 units in `plugins/vivy-memory/i18n/catalog.json`, every unit
has non-empty `en` and `zh` messages (scripted check, 0 missing).

## Module source digest

Both pin sites set to the same fixed-point digest:

```text
plugins/vivy-memory/vivy-module.yaml  source.sha256
plugins/vivy-memory/module.go         module.Source.SHA256
= 651cad18f9facf19a8aacb9d9c1e93131bb51fe476c3d236e5a1abceb230420b
```

`go run ./sdk/internal/cmd/source-hash plugins/vivy-memory <digest>` returns
the digest unchanged; `stage:ui` accepts the module.

## Behavioral coverage (view.test.tsx, 9 cases)

- status strip issues `vivy.memory.status` once and shows the startup
  revision / no-database line.
- add sends `{kind: "long_term", content}` and selects the applied entry.
- update sends `{id, content, base_revision: 3}`; a
  `memory_revision_conflict` keeps the dialog open, shows the conflict copy,
  and the refresh affordance re-fetches via `vivy.memory.get`.
- delete confirm stays disabled until a non-empty reason and sends
  `{id, reason, base_revision}`.
- rules panel renders `rules.read` content and writes
  `{content, base_revision: "<digest>"}`.

## Not verified here

Visual/browser pass of the live app (needs a running backend with a real BML
home); deferred to a UI-test handoff if requested.
