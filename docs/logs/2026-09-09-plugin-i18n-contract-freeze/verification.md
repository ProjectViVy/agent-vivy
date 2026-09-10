# Plugin I18N contract freeze verification

Date: 2026-09-09

## Passed checks

- Public Port table count: 14.
- Proposal-marker audit over the four normative contracts: zero matches.
- Normative vocabulary audit found `vivy.i18n/v1`,
  `INCOMPLETE_LOCALE`, `canonical_catalog_json`, and literal
  `plugin.<module-id>` ownership in the canonical corpus.
- Program status audit found PLG-P1 `SCHEDULED · IN PROGRESS` and PLG-P2
  `SCHEDULED · QUEUED`.
- `node scripts/check-plugin-v1-fixtures.mjs`: 9 cases, 2 accepted and 7
  rejected.
- `node --test scripts/check-plugin-v1-fixtures.test.mjs`: 5 passed, 0
  failed.
- `go test ./sdk/module ./sdk/port -count=1` with Go 1.26.4: passed.
- `git diff --check`: passed.

## Gate note

The local environment does not have `just` or PowerShell, while the repository
`justfile` selects PowerShell as its shell. The equivalent focused Node and Go
checks above ran locally. Full repository Go verification is tracked with the
P1/P2 implementation and remote CI; the unrelated cron baseline failure found
during the first full run was reproduced, root-caused, and fixed in the
separate commit `1334e4d`.
