# N0 — acceptance

- Ordinary chat never loads or injects notebook content: `ServiceDeps` no
  longer accepts a `Notes` store, `Service` holds no digest helper, and
  `composeRunPreamble` composes only date + active-tool presence (+ face /
  collaboration). `TestServiceDoesNotInjectNotebook` proves ordinary,
  continued, and post-restart turns carry no note bytes.
- Explicit access is unchanged: `internal/tools/notes.go` / `writenote.go`,
  the three note tools, the legacy `notes` table and every stored note and
  saved message remain. No compat layer, flag, or substitute ContextSource
  was introduced.
- Gate evidence: plan-specified focused tests pass and `just ci` is green
  end-to-end (see verification.md).
