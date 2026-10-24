# Verification

Host: macOS 27.0.1 arm64. Repository: `stable` at `83029dc0` plus this change.

## Checks run

```text
git diff --check                                  # clean
git diff --cached --check                         # clean
gofmt -l <changed .go files>                      # empty
rg 'superpowers/(plans|specs)'                    # no matches
rg 'docs/(AGENT-VIVY-ARCHITECTURE-V0|IMPLEMENTATION-PLAN|...)\.md'
                                                  # no matches
node /tmp/check-doc-links.mjs                     # 712 relative links checked
```

## Link results

- Relative Markdown links: all resolve. The checker reports two targets that
  are inline code rather than links, because the code contains the closing
  bracket, parenthesis, and a word (`ctx` in one, the numeral `8` in the
  other); both predate this change and are not link syntax.
- Absolute `docs/...` references: 476 unique paths checked. 83 unresolved,
  all pre-existing: external-repository paths (INOFY, DSH, diva-next),
  truncated example tokens, or shorthand such as
  `docs/archive/plans/nudge/ND-0..4.md`. No moved path is left unresolved.
- `docs/superpowers/` is gone and no document references it as a path.
  Remaining `superpowers` matches are skill names (`superpowers:executing-plans`)
  and marketplace entries, not documentation paths.

## Structure

```text
docs/
  README.md  TODO.md  COMPLETE.MD  DEFER.MD
  architecture/  specs/  plans/  research/  dev/  logs/  archive/
```

`docs/logs/` contains 461 delivery directories (one renamed to a full date);
nothing else was added or removed there.

## Not run

- `just ci` (full product gate; docs-only scope).
- Windows `dev.ps1` / native acceptance.
