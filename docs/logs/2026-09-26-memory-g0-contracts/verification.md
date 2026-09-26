# Verification: memory G0 contract freeze + tracker foldback

Environment: Linux, repository root, branch `feat/memory`, 2026-09-26.
Docs-only iteration — the gate for G0 doc stories is review plus `just ci`
staying green; no Go/TS source, generated file, or CI recipe was touched, so
nothing in `just ci` is affected by these edits.

## 1 — Every cited path exists

```text
ls docs/research/2026-09-26-memory-upstream-pin.md \
   docs/architecture/VIVY-MEMORY-PROFILE.md \
   docs/architecture/VIVY-MEMORY-HOST-CONTRACTS.md \
   docs/research/2026-09-26-memory-g0-readiness.md \
   docs/logs/2026-09-26-memory-bml/notes.md \
   bml/provider.go bml/README.md
# all present
```

## 2 — No code changes

```text
git status --porcelain
# only *.md additions/modifications under docs/ (plus this log)
```

## 3 — No duplicate/stale memory entry

The former MEM-1 umbrella row was rewritten in place in `docs/DEFER.MD` and
expanded into stage rows in `docs/TODO.md` §0.1; `docs/research/OPEN-ITEMS.md`
mirrors the same state. Exactly one memory track entry per file; AutoDream /
Evolution / RAG carry their own deferred ID `MEM-CAP`.

## 4 — SCX cross-refs are one line each

`docs/architecture/SCX-PLUGIN-INTEGRATION.md`: one pointer sentence naming
`VIVY-MEMORY-PROFILE.md` + `VIVY-MEMORY-HOST-CONTRACTS.md` appended to the
external-memory note under the capability table.
`docs/architecture/SCX-ARCHITECTURE-DESIGN.md` §C: one pointer line to
`VIVY-MEMORY-PROFILE.md`. No restructuring in either file.
