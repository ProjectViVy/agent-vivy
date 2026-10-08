# Bash redirect classification — quoted targets, heredocs, fd duplication

## Problem

`ClassifyShellScript` rejected normal file-writing scripts as "redirection
outside the run workspace" (fail-closed deny). Reproduced forms:

```bash
cat > probe.txt <<'EOF'
benchmark payload
EOF

printf 'benchmark payload\n' > 'probe.txt'
```

Two root causes in `internal/tools/bashclass.go`:

1. `redirectTarget` required the redirect word to be a single `*syntax.Lit`.
   A statically-quoted target (`'probe.txt'`, `"probe.txt"`) parses as
   `SglQuoted`/`DblQuoted`, so the helper returned `""`, which the walk
   treats as an unverifiable target and denies.
2. Heredoc redirects (`<<`, `<<-`, `<<<`) were sent through path validation
   even though their word is the delimiter or inline content, not a path.
   A quoted delimiter (`<<'EOF'`) failed the same literal check and denied
   the whole script; the heredoc body was likewise at risk of being
   scanned as a path.

Each false deny made the model rewrite the command and burn API turns.

## Fix

`redirectTarget` now classifies by redirect operator:

- `<<`, `<<-`, `<<<` (Hdoc/DashHdoc/WordHdoc): no filesystem target, skipped.
- `>&word` / `<&word` (DplOut/DplIn): a decimal fd number or `-` duplicates
  or closes an open descriptor — skipped. A non-numeric word is bash's
  legacy `&>` spelling and names a file, so it is still path-checked.
- Path operators (`>`, `>>`, `>|`, `<>`, `<`, `&>`, `&>>`): the target word
  is resolved with the existing `staticShellWord`, which already expands
  literal `SglQuoted`/`DblQuoted` parts, so quoted paths validate correctly.
- Words containing expansions (`$f`, `$(...)`, `${x}`) still return the
  unverifiable marker and fail closed, unchanged.

Skipped (non-path) redirects still get their word scanned for dynamic
expansion (`shellWordHasExpansion` on `redir.Word`/`redir.Hdoc`), so an
unquoted heredoc body like `cat <<EOF\n$HOME\nEOF` keeps the same
"mutating: dynamic shell expansion" tier as `echo $HOME`, while
`<<'EOF'` bodies stay literal and safe.

Resulting tier changes (all intentional):

- `ls >'out.txt'`, `cat >f <<'EOF'`, `ls 3>fd.txt`, `echo x >& both.log`,
  `echo x &> all.log`: now `mutating` (write detected) instead of `denied`.
- `cat <<'EOF'`, `cat <<< 'x'`, `ls 2>&1`, `ls >&2`, `ls >&-`: now `safe`
  instead of `denied`/`mutating`.
- `echo x > '/tmp/x'`, `echo x > '../x'`, `echo x > $HOME/x`, heredoc into
  `../escape`: still `denied`.

The workspace boundary is unchanged: the same `isForbiddenHostPath` +
`containsParentTraversal` predicates gate every real path target.

## Files

- `internal/tools/bashclass.go` — operator-aware `redirectTarget`, expansion
  scan for non-path redirects, `isFileDescriptorWord` helper.
- `internal/tools/bashclass_test.go` — 21 classifier cases covering quoted
  paths, heredoc/herestring forms, fd dup/close, and the preserved denies.
- `internal/runtime/command_backend_bash_test.go` — end-to-end regression:
  native bash writes `probe.txt` (heredoc) and `quoted.txt` (quoted target)
  inside the run workspace with exact content; quoted absolute/traversal
  targets still never execute.
- `sdk/internal/assembly/conformance_results.json` — internal source digest
  re-pinned to `41bade3a…8b37c3`.

See [verification](verification.md) and [acceptance](acceptance.md).
